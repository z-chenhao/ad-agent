import { createInterface } from "node:readline";
import { copyFile, mkdir, chmod } from "node:fs/promises";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import { EnvHttpProxyAgent, setGlobalDispatcher, install } from "undici";
import {
  createAgentSession,
  createExtensionRuntime,
  ModelRuntime,
  SessionManager,
  SettingsManager,
  type ResourceLoader,
  type ToolDefinition,
  type AgentSession,
} from "@earendil-works/pi-coding-agent";
import { fence, parseInput, type Start, type Reply } from "./protocol.js";

// Importing the SDK does not initialize the CLI's network layer. Own this bootstrap explicitly.
const dispatcher = new EnvHttpProxyAgent({
  allowH2: false,
  headersTimeout: 300_000,
  bodyTimeout: 300_000,
});
setGlobalDispatcher(dispatcher);
install();
process.umask(0o077);
const send = (v: unknown) => process.stdout.write(JSON.stringify(v) + "\n");
const pending = new Map<string, (r: Reply["result"]) => void>();
let session: AgentSession | undefined;
let started = false;
let finished = false;
const input = createInterface({ input: process.stdin, crlfDelay: Infinity });

async function run(req: Start) {
  const cwd = process.cwd();
  const runtime = await ModelRuntime.create({
    modelsPath: null,
    allowModelNetwork: false,
    signal: AbortSignal.timeout(10_000),
  });
  const model = runtime.getModel(req.model.provider, req.model.model);
  if (!model || !runtime.isUsingOAuth(req.model.provider))
    throw new Error("oauth_or_model_missing");
  const resources: ResourceLoader = {
    getExtensions: () => ({
      extensions: [],
      errors: [],
      runtime: createExtensionRuntime(),
    }),
    getSkills: () => ({ skills: [], diagnostics: [] }),
    getPrompts: () => ({ prompts: [], diagnostics: [] }),
    getThemes: () => ({ themes: [], diagnostics: [] }),
    getAgentsFiles: () => ({ agentsFiles: [] }),
    getSystemPrompt: () => req.system,
    getSystemPromptSource: () => undefined,
    getAppendSystemPrompt: () => [],
    getAppendSystemPromptSources: () => [],
    extendResources: () => {},
    reload: async () => {},
  };
  let manager: SessionManager;
  if (req.session_dir) {
    await mkdir(req.session_dir, { recursive: true, mode: 0o700 });
    if (req.checkpoint) {
      const fork = join(req.session_dir, randomUUID() + ".jsonl");
      await copyFile(req.checkpoint, fork);
      await chmod(fork, 0o600);
      manager = SessionManager.open(fork, req.session_dir, cwd);
    } else manager = SessionManager.create(cwd, req.session_dir);
  } else manager = SessionManager.inMemory(cwd);
  let round = 0;
  let budget = false;
  const partialTools = new Map<number, { id: string; name: string }>();
  const tools: ToolDefinition[] = req.tools.map((tool) => ({
    name: tool.name,
    label: tool.name,
    description: tool.description,
    parameters: tool.parameters as ToolDefinition["parameters"],
    execute: async (id, args, signal) => {
      if (pending.has(id)) throw new Error("duplicate_call");
      if (signal?.aborted) throw new Error("cancelled");
      const result = await new Promise<Reply["result"]>((resolve, reject) => {
        const abort = () => {
          pending.delete(id);
          reject(new Error("cancelled"));
        };
        signal?.addEventListener("abort", abort, { once: true });
        pending.set(id, (r) => {
          signal?.removeEventListener("abort", abort);
          resolve(r);
        });
        send({
          type: "tool_call",
          id,
          name: tool.name,
          arguments: args,
          round,
        });
      });
      if (result.close) session?.setActiveToolsByName([]);
      return {
        content: [{ type: "text" as const, text: fence(result) }],
        details: { ok: result.ok },
      };
    },
  }));
  ({ session } = await createAgentSession({
    cwd,
    model,
    modelRuntime: runtime,
    thinkingLevel: req.model.reasoning,
    tools: req.tools.map((t) => t.name),
    customTools: tools,
    resourceLoader: resources,
    sessionManager: manager,
    settingsManager: SettingsManager.inMemory({
      transport: "sse",
      compaction: { enabled: true },
      retry: {
        enabled: true,
        maxRetries: 1,
        baseDelayMs: 500,
        provider: { maxRetries: 1, maxRetryDelayMs: 2_000 },
      },
    }),
  }));
  if (
    session.getActiveToolNames().sort().join() !==
    req.tools
      .map((t) => t.name)
      .sort()
      .join()
  )
    throw new Error("tool_isolation_failed");
  const startIndex = session.messages.length;
  session.subscribe((event) => {
    if (event.type === "turn_start") {
      round++;
      // At the next model step remove tools; Go independently rejects over-budget business calls.
      if (round > req.max_rounds) {
        budget = true;
        session!.setActiveToolsByName([]);
      }
      if (round > req.max_rounds + 1) void session!.abort();
    }
    if (
      event.type === "message_update" &&
      event.assistantMessageEvent.type === "text_delta"
    )
      send({ type: "text_delta", text: event.assistantMessageEvent.delta });
    if (event.type === "message_update") {
      const update = event.assistantMessageEvent;
      if (
        update.type === "toolcall_start" ||
        update.type === "toolcall_delta" ||
        update.type === "toolcall_end"
      ) {
        const block = update.partial.content[update.contentIndex];
        if (block?.type === "toolCall")
          partialTools.set(update.contentIndex, {
            id: block.id,
            name: block.name,
          });
        const tool = partialTools.get(update.contentIndex);
        if (tool)
          send({
            type: "tool_delta",
            id: tool.id,
            name: tool.name,
            arguments: update.type === "toolcall_delta" ? update.delta : "",
          });
        if (update.type === "toolcall_end")
          partialTools.delete(update.contentIndex);
      }
    }
  });
  await session.prompt(req.prompt);
  const messages = session.messages.slice(startIndex);
  const assistants = messages.filter((m) => m.role === "assistant");
  const last = assistants.at(-1);
  const calls = assistants.flatMap((m) =>
    m.content.filter((b) => b.type === "toolCall").map((b) => b.id),
  );
  const results = messages
    .filter((m) => m.role === "toolResult")
    .map((m) => m.toolCallId);
  if (
    calls.length !== results.length ||
    new Set(calls).size !== calls.length ||
    calls.some((id) => results.filter((r) => r === id).length !== 1)
  )
    throw new Error("unpaired_tools");
  if (!last || last.stopReason !== "stop") throw new Error("model_incomplete");
  const text = last.content
    .filter((b) => b.type === "text")
    .map((b) => b.text)
    .join("");
  const usage = assistants.reduce(
    (a, m) => ({
      input: a.input + m.usage.input,
      output: a.output + m.usage.output,
      cache_read: a.cache_read + m.usage.cacheRead,
      cache_write: a.cache_write + m.usage.cacheWrite,
    }),
    { input: 0, output: 0, cache_read: 0, cache_write: 0 },
  );
  send({
    type: "done",
    text,
    stop: budget ? "budget" : "stop",
    usage,
    checkpoint: manager.getSessionFile(),
  });
}
async function shutdown(code: number) {
  if (finished) return;
  finished = true;
  session?.dispose();
  input.close();
  await dispatcher.destroy();
  process.exitCode = code;
  process.stdin.destroy();
}
input.on("line", (line) => {
  try {
    const frame = parseInput(line);
    if (frame.type === "start") {
      if (started) throw new Error("duplicate_start");
      started = true;
      void run(frame)
        .then(() => shutdown(0))
        .catch(() => {
          send({ type: "error", error: "runtime_failed" });
          void shutdown(1);
        });
    } else {
      const resolve = pending.get(frame.id);
      if (!resolve) throw new Error("unknown_correlation");
      pending.delete(frame.id);
      resolve(frame.result);
    }
  } catch {
    send({ type: "error", error: "invalid_protocol" });
    void session?.abort();
    void shutdown(1);
  }
});
input.on("close", () => {
  if (!finished) {
    void session?.abort();
    void shutdown(1);
  }
});
