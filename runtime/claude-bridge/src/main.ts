import { createInterface } from "node:readline";
import { chmod, mkdir, readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { randomUUID } from "node:crypto";
import {
  createSdkMcpServer,
  query,
  tool,
  type SDKResultMessage,
} from "@anthropic-ai/claude-agent-sdk";
import { z, type ZodType } from "zod";
import {
  fence,
  parseInput,
  publicTextSuffix,
  type Reply,
  type Start,
} from "./protocol.js";

process.umask(0o077);
process.env.CLAUDE_AGENT_SDK_MCP_NO_PREFIX = "1";
const send = (value: unknown) =>
  process.stdout.write(JSON.stringify(value) + "\n");
const pending = new Map<string, (result: Reply["result"]) => void>();
const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
let started = false;
let finished = false;

function toolShape(schema: Record<string, unknown>): Record<string, ZodType> {
  if (
    schema.type !== "object" ||
    !schema.properties ||
    typeof schema.properties !== "object"
  )
    throw new Error("invalid_tool_schema");
  const required = new Set(
    Array.isArray(schema.required) ? schema.required.map(String) : [],
  );
  const shape: Record<string, ZodType> = {};
  for (const [name, property] of Object.entries(
    schema.properties as Record<string, unknown>,
  )) {
    let value = z.fromJSONSchema(
      property as Parameters<typeof z.fromJSONSchema>[0],
    );
    if (!required.has(name)) value = value.optional();
    shape[name] = value;
  }
  return shape;
}

async function run(req: Start) {
  const apiKey = process.env[req.model.api_key_env];
  if (!apiKey) throw new Error("api_key_missing");
  if (!req.session_dir) throw new Error("session_dir_required");
  await mkdir(req.session_dir, { recursive: true, mode: 0o700 });
  let resume: string | undefined;
  if (req.checkpoint) {
    const parsed = JSON.parse(await readFile(req.checkpoint, "utf8")) as {
      version?: unknown;
      session_id?: unknown;
      model?: unknown;
    };
    if (
      parsed.version !== 1 ||
      typeof parsed.session_id !== "string" ||
      parsed.model !== req.model.model
    )
      throw new Error("invalid_checkpoint");
    resume = parsed.session_id;
  }
  let round = 0;
  let calls = 0;
  let closed = false;
  const sdkTools = req.tools.map((spec) =>
    tool(
      spec.name,
      spec.description,
      toolShape(spec.parameters),
      async (args) => {
        const id = randomUUID();
        calls++;
        if (
          closed ||
          calls > 64 ||
          round < 1 ||
          (req.max_rounds > 0 && round > req.max_rounds)
        )
          return {
            content: [
              {
                type: "text" as const,
                text: fence({ ok: false, error: "tool_budget_exhausted" }),
              },
            ],
            isError: true,
          };
        const result = await new Promise<Reply["result"]>((resolve) => {
          pending.set(id, resolve);
          send({
            type: "tool_call",
            id,
            name: spec.name,
            arguments: args,
            round,
          });
        });
        if (result.close) closed = true;
        return {
          content: [{ type: "text" as const, text: fence(result) }],
          ...(result.ok ? {} : { isError: true }),
        };
      },
    ),
  );
  const server = createSdkMcpServer({
    name: "ad_backend",
    version: "0.1.0",
    tools: sdkTools,
    alwaysLoad: true,
    timeout: 300_000,
  });
  const childEnv = { ...process.env };
  for (const name of [
    "ANTHROPIC_AUTH_TOKEN",
    "CLAUDE_CODE_OAUTH_TOKEN",
    "CLAUDE_CODE_USE_BEDROCK",
    "CLAUDE_CODE_USE_VERTEX",
    "CLAUDE_CODE_USE_FOUNDRY",
  ])
    delete childEnv[name];
  childEnv.ANTHROPIC_API_KEY = apiKey;
  childEnv.ANTHROPIC_BASE_URL = req.model.base_url;
  childEnv.CLAUDE_AGENT_SDK_MCP_NO_PREFIX = "1";
  childEnv.CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC = "1";
  let result: SDKResultMessage | undefined;
  let messageID = "";
  const publicBlocks = new Map<string, Map<number, string>>();
  const emitText = (id: string, index: number, text: string) => {
    const blocks = publicBlocks.get(id) ?? new Map<number, string>();
    blocks.set(index, (blocks.get(index) ?? "") + text);
    publicBlocks.set(id, blocks);
    if (text) send({ type: "text_delta", id, text });
  };
  const turnIDs = new Set<string>();
  const stream = query({
    prompt: req.prompt,
    options: {
      cwd: req.session_dir,
      model: req.model.model,
      systemPrompt: { type: "custom", prompt: req.system, snapshot: true },
      tools: [],
      allowedTools: req.tools.map((spec) => spec.name),
      mcpServers: { ad_backend: server },
      strictMcpConfig: true,
      settingSources: [],
      plugins: [],
      skills: [],
      permissionMode: "dontAsk",
      permissionPrompts: "none",
      includePartialMessages: true,
      ...(req.max_rounds > 0 ? { maxTurns: req.max_rounds + 1 } : {}),
      persistSession: true,
      ...(resume ? { resume, forkSession: true } : {}),
      env: childEnv,
    },
  });
  try {
    for await (const message of stream) {
      if (message.type === "assistant" && message.parent_tool_use_id === null) {
        message.message.content.forEach((block, index) => {
          if (block.type === "text")
            emitText(
              message.message.id,
              index,
              publicTextSuffix(
                publicBlocks.get(message.message.id)?.get(index) ?? "",
                block.text,
              ),
            );
        });
        const uses = message.message.content.filter(
          (block) => block.type === "tool_use",
        );
        if (uses.length && !turnIDs.has(message.message.id)) {
          turnIDs.add(message.message.id);
          round++;
        }
      } else if (
        message.type === "stream_event" &&
        message.parent_tool_use_id === null
      ) {
        const event = message.event;
        if (event.type === "message_start") messageID = event.message.id;
        if (
          event.type === "content_block_delta" &&
          event.delta.type === "text_delta"
        )
          emitText(messageID, event.index, event.delta.text);
      } else if (message.type === "result") {
        result = message;
      }
    }
  } finally {
    stream.close();
  }
  if (!result || result.subtype !== "success" || result.is_error)
    throw new Error("provider_failed");
  const checkpoint = join(req.session_dir, "claude-checkpoint.json");
  await writeFile(
    checkpoint,
    JSON.stringify({
      version: 1,
      session_id: result.session_id,
      model: req.model.model,
    }),
    { mode: 0o600 },
  );
  await chmod(checkpoint, 0o600);
  const usage = result.usage;
  send({
    type: "done",
    text: result.result,
    stop:
      req.max_rounds > 0 && result.num_turns > req.max_rounds
        ? "budget"
        : "stop",
    checkpoint,
    usage: {
      input: usage.input_tokens,
      output: usage.output_tokens,
      cache_read: usage.cache_read_input_tokens,
      cache_write: usage.cache_creation_input_tokens,
    },
  });
}

async function shutdown(code: number) {
  if (finished) return;
  finished = true;
  input.close();
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
    void shutdown(1);
  }
});
