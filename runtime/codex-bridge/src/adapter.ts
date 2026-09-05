import { createRequire } from "node:module";
import { dirname, join, isAbsolute, relative, sep } from "node:path";
import {
  chmod,
  lstat,
  mkdir,
  mkdtemp,
  readFile,
  realpath,
  rename,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { AppServer } from "./rpc.js";
import {
  digest,
  fence,
  isolatedConfig,
  type Start,
  type Reply,
} from "./protocol.js";

interface Tokens {
  accessToken: string;
  chatgptAccountId: string;
  chatgptPlanType?: string;
}
interface NativeThread {
  thread: { id: string };
  model: string;
  instructionSources?: string[];
}
interface Checkpoint {
  runtime: "codex";
  thread: string;
  binding: string;
}
interface NativeCall {
  threadId: string;
  turnId: string;
  callId: string;
  namespace?: string;
  tool: string;
  arguments: unknown;
}
interface Item {
  id: string;
  type: string;
  text?: string;
  phase?: string;
}
interface Notification {
  threadId?: string;
  turnId?: string;
  itemId?: string;
  delta?: string;
  item?: Item;
  turn?: { id: string; status: string; error?: unknown };
  tokenUsage?: {
    total: {
      inputTokens: number;
      outputTokens: number;
      cachedInputTokens: number;
    };
  };
}
export interface BridgeHooks {
  emit: (frame: unknown) => void;
  execute: (
    id: string,
    name: string,
    args: unknown,
    round: number,
  ) => Promise<Reply["result"]>;
}

export function nativeBinary(): string {
  const require = createRequire(import.meta.url);
  const targets: Record<string, string> = {
    "darwin-arm64": "aarch64-apple-darwin",
    "darwin-x64": "x86_64-apple-darwin",
    "linux-arm64": "aarch64-unknown-linux-musl",
    "linux-x64": "x86_64-unknown-linux-musl",
    "win32-x64": "x86_64-pc-windows-msvc",
    "win32-arm64": "aarch64-pc-windows-msvc",
  };
  const target = targets[`${process.platform}-${process.arch}`];
  if (!target) throw new Error("unsupported_codex_platform");
  const pkg = require.resolve(
    `@openai/codex-${process.platform}-${process.arch}/package.json`,
  );
  return join(
    dirname(pkg),
    "vendor",
    target,
    "bin",
    process.platform === "win32" ? "codex.exe" : "codex",
  );
}

export async function oauth(
  create = ModelRuntime.create,
): Promise<() => Promise<Tokens>> {
  // Pi provides only credential resolution/refresh, never the Codex model loop.
  const auth = await create({
    modelsPath: null,
    allowModelNetwork: false,
    // isUsingOAuth reads the initialized credential snapshot. Skipping initial
    // refresh reports false even when getAuth can resolve a valid saved login.
    signal: AbortSignal.timeout(10_000),
  });
  let boundAccount: string | undefined;
  return async () => {
    if (!auth.isUsingOAuth("openai-codex"))
      throw new Error("chatgpt_oauth_required");
    const result = await auth.getAuth("openai-codex", {
      signal: AbortSignal.timeout(8000),
      minOAuthValidityMs: 300_000,
    });
    const token = result?.auth.apiKey;
    if (!token) throw new Error("chatgpt_oauth_required");
    // The account claim accompanies an existing credential, not advertising authority.
    const claims = JSON.parse(
      Buffer.from(token.split(".")[1] ?? "", "base64url").toString(),
    ) as Record<
      string,
      { chatgpt_account_id?: string; chatgpt_plan_type?: string }
    >;
    const account = claims["https://api.openai.com/auth"];
    if (
      !account?.chatgpt_account_id ||
      (boundAccount && boundAccount !== account.chatgpt_account_id)
    )
      throw new Error("oauth_account_changed");
    boundAccount = account.chatgpt_account_id;
    return {
      accessToken: token,
      chatgptAccountId: boundAccount,
      chatgptPlanType: account.chatgpt_plan_type,
    };
  };
}

export async function run(
  request: Start,
  hooks: BridgeHooks,
  signal: AbortSignal,
): Promise<void> {
  const ephemeral = !request.session_dir;
  const directory =
    request.session_dir ?? (await mkdtemp(join(tmpdir(), "ad-agent-codex-")));
  if (!isAbsolute(directory)) throw new Error("invalid_session_directory");
  await mkdir(directory, { recursive: true, mode: 0o700 });
  await chmod(directory, 0o700);
  const root = ephemeral ? directory : dirname(directory);
  const home = join(root, "codex-private");
  const cwd = join(home, "workspace");
  await mkdir(cwd, { recursive: true, mode: 0o700 });
  await chmod(home, 0o700);
  const binding = digest({
    system: request.system,
    tools: request.tools,
    model: request.model,
  });
  let checkpoint: Checkpoint | undefined;
  if (request.checkpoint) {
    const path = await realpath(request.checkpoint);
    const rel = relative(await realpath(root), path);
    const info = await lstat(request.checkpoint);
    if (
      !rel ||
      rel.startsWith(".." + sep) ||
      isAbsolute(rel) ||
      !info.isFile() ||
      info.isSymbolicLink() ||
      info.mode & 0o077 ||
      info.size > 4096
    )
      throw new Error("invalid_checkpoint_location");
    checkpoint = JSON.parse(await readFile(path, "utf8")) as Checkpoint;
    if (
      checkpoint.runtime !== "codex" ||
      checkpoint.binding !== binding ||
      !/^[a-zA-Z0-9-]+$/.test(checkpoint.thread)
    )
      throw new Error("checkpoint_binding_mismatch");
  }
  const config = isolatedConfig(request);
  // This is a private runtime home, not the operator's global Codex config.
  const env: NodeJS.ProcessEnv = {};
  for (const name of [
    "PATH",
    "HOME",
    "TMPDIR",
    "SYSTEMROOT",
    "SSL_CERT_FILE",
    "SSL_CERT_DIR",
    "HTTP_PROXY",
    "HTTPS_PROXY",
    "ALL_PROXY",
    "NO_PROXY",
    "http_proxy",
    "https_proxy",
    "all_proxy",
    "no_proxy",
  ])
    if (process.env[name]) env[name] = process.env[name];
  env.CODEX_HOME = home;
  if (request.model.auth_mode === "api_key") {
    const key = process.env[request.model.api_key_env!];
    if (!key) throw new Error("api_key_missing");
    env[request.model.api_key_env!] = key;
  }
  const server = new AppServer(
    nativeBinary(),
    [
      "app-server",
      "--listen",
      "stdio://",
      "-c",
      'cli_auth_credentials_store="ephemeral"',
      "-c",
      "analytics.enabled=false",
    ],
    cwd,
    env,
  );
  let thread = "",
    turn = "",
    settled = false,
    denied = false,
    budget = false;
  let calls = 0,
    textBytes = 0,
    finalText = "",
    lastText = "";
  const seen = new Set<string>();
  const texts = new Map<string, string>();
  const names = new Set(request.tools.map((tool) => tool.name));
  let dispatch: Promise<unknown> = Promise.resolve();
  const usage = { input: 0, output: 0, cache_read: 0, cache_write: 0 };
  let baseline: typeof usage | undefined;
  let resolveTurn!: () => void, rejectTurn!: (error: Error) => void;
  const completion = new Promise<void>((resolve, reject) => {
    resolveTurn = resolve;
    rejectTurn = reject;
  });
  // Setup can fail before the event loop awaits completion.
  void completion.catch(() => {});
  const fail = (code: string) => rejectTurn(new Error(code));
  const interrupt = () => {
    if (thread && turn)
      void server
        .request("turn/interrupt", { threadId: thread, turnId: turn }, 1000)
        .catch(() => {});
    fail("cancelled");
    void server.close();
  };
  signal.addEventListener("abort", interrupt, { once: true });
  try {
    server.onFailure = (err) => rejectTurn(err);
    const tokens =
      request.model.auth_mode === "chatgpt_oauth" ? await oauth() : undefined;
    server.onRequest = async (method, raw) => {
      if (method === "account/chatgptAuthTokens/refresh" && tokens)
        return tokens();
      // Native approval is never interpreted as business approval.
      if (method !== "item/tool/call") throw new Error("native_request_denied");
      const call = raw as NativeCall;
      if (
        call.threadId !== thread ||
        call.turnId !== turn ||
        !call.callId ||
        seen.has(call.callId) ||
        call.namespace ||
        !names.has(call.tool)
      ) {
        fail("native_tool_boundary_violation");
        throw new Error("native_tool_boundary_violation");
      }
      seen.add(call.callId);
      // Native Code Mode can issue parallel requests. Serialize dispatch so a
      // successful terminal presentation closes even already-queued callbacks.
      const result = dispatch.then(async () => {
        if (signal.aborted) throw new Error("cancelled");
        calls++;
        if (calls > 64) {
          fail("tool_call_limit_exceeded");
          throw new Error("tool_call_limit_exceeded");
        }
        // Native model rounds are not exposed. Bound internal tool dispatches;
        // the main request (zero) has no fixed model-round ceiling.
        if (request.max_rounds > 0 && calls > request.max_rounds) {
          denied = true;
          budget = true;
        }
        const reply = denied
          ? { ok: false, error: "tools_closed_finish_response" }
          : await hooks.execute(
              call.callId,
              call.tool,
              call.arguments,
              request.max_rounds > 0 ? calls : 1,
            );
        if (reply.close) denied = true;
        return {
          success: reply.ok,
          contentItems: [{ type: "inputText", text: fence(reply) }],
        };
      });
      dispatch = result;
      return result;
    };
    server.onNotification = (method, raw) => {
      const event = raw as Notification;
      if (event.threadId && thread && event.threadId !== thread) return;
      if (method === "turn/started" && event.turn) {
        turn = event.turn.id;
        return;
      }
      if (method === "model/rerouted") {
        fail("unexpected_model_reroute");
        return;
      }
      if (method === "thread/tokenUsage/updated" && event.tokenUsage) {
        const total = event.tokenUsage.total;
        const value = {
          input: total.inputTokens - total.cachedInputTokens,
          output: total.outputTokens,
          cache_read: total.cachedInputTokens,
          cache_write: 0,
        };
        if (!turn) baseline = value;
        else
          for (const key of ["input", "output", "cache_read"] as const)
            usage[key] = Math.max(0, value[key] - (baseline?.[key] ?? 0));
      }
      if (
        method === "item/started" &&
        event.item &&
        [
          "commandExecution",
          "fileChange",
          "mcpToolCall",
          "collabToolCall",
          "webSearch",
          "imageView",
        ].includes(event.item.type)
      ) {
        fail("unexpected_native_capability");
        return;
      }
      if (
        method === "item/agentMessage/delta" &&
        event.itemId &&
        typeof event.delta === "string"
      ) {
        textBytes += Buffer.byteLength(event.delta);
        if (textBytes > 65536) {
          fail("model_text_limit_exceeded");
          return;
        }
        texts.set(event.itemId, (texts.get(event.itemId) ?? "") + event.delta);
        hooks.emit({ type: "text_delta", id: event.itemId, text: event.delta });
      }
      if (method === "item/completed" && event.item?.type === "agentMessage") {
        const item = event.item;
        const text = item.text ?? "";
        const streamed = texts.get(item.id) ?? "";
        if (!text.startsWith(streamed)) {
          fail("native_text_mismatch");
          return;
        }
        const suffix = text.slice(streamed.length);
        if (suffix) {
          textBytes += Buffer.byteLength(suffix);
          if (textBytes > 65536) {
            fail("model_text_limit_exceeded");
            return;
          }
          hooks.emit({ type: "text_delta", id: item.id, text: suffix });
        }
        lastText = text;
        if (item.phase === "final_answer") finalText = text;
      }
      // No raw reasoning, native auth, raw error, transcript, or unrelated events.
      if (method === "turn/completed" && event.turn?.id === turn) {
        if (event.turn.status !== "completed")
          rejectTurn(
            new Error(
              event.turn.status === "interrupted"
                ? "cancelled"
                : "native_turn_failed",
              { cause: event.turn.error },
            ),
          );
        else {
          settled = true;
          resolveTurn();
        }
      }
    };
    await server.request("initialize", {
      clientInfo: {
        name: "ad_agent",
        title: "Ad Agent",
        version: "0.1.0-alpha.1",
      },
      capabilities: { experimentalApi: true },
    });
    server.notify("initialized");
    if (tokens)
      await server.request("account/login/start", {
        type: "chatgptAuthTokens",
        ...(await tokens()),
      });
    // Discover names only and disable automatic native skills, including global
    // roots. Advertising skills continue through application-owned load_skill.
    const skills = await server.request<{
      data: { skills: { path: string }[] }[];
    }>("skills/list", { cwds: [cwd] });
    config["skills.config"] = skills.data.flatMap((group) =>
      group.skills.map((skill) => ({ path: skill.path, enabled: false })),
    );
    const options = {
      model: request.model.model,
      modelProvider:
        request.model.auth_mode === "api_key" ? "ad_direct" : "openai",
      cwd,
      approvalPolicy: "never",
      sandbox: "read-only",
      baseInstructions: request.system,
      developerInstructions: "",
      config,
      allowProviderModelFallback: false,
    };
    let native: NativeThread;
    if (checkpoint)
      native = await server.request<NativeThread>("thread/fork", {
        ...options,
        threadId: checkpoint.thread,
      });
    else
      native = await server.request<NativeThread>("thread/start", {
        ...options,
        environments: [],
        ephemeral,
        dynamicTools: request.tools.map((tool) => ({
          name: tool.name,
          description: tool.description,
          inputSchema: tool.parameters,
          deferLoading: false,
        })),
      });
    if (
      native.model !== request.model.model ||
      native.instructionSources?.length
    )
      throw new Error("native_context_isolation_failed");
    thread = native.thread.id;
    if (signal.aborted) throw new Error("cancelled");
    const started = await server.request<{ turn: { id: string } }>(
      "turn/start",
      {
        threadId: thread,
        input: [{ type: "text", text: request.prompt, text_elements: [] }],
        environments: [],
        effort: request.model.reasoning,
        summary: "none",
      },
    );
    if (turn && turn !== started.turn.id)
      throw new Error("native_turn_correlation_failed");
    turn = started.turn.id;
    await completion;
    let saved = "";
    if (!ephemeral) {
      // Write a small opaque native-session reference, never the provider transcript.
      saved = join(directory, "codex-checkpoint.json");
      const temp = saved + ".tmp";
      await writeFile(
        temp,
        JSON.stringify({
          runtime: "codex",
          thread,
          binding,
        } satisfies Checkpoint),
        { mode: 0o600, flag: "wx" },
      );
      await rename(temp, saved);
    }
    hooks.emit({
      type: "done",
      text: finalText || lastText,
      stop: budget ? "budget" : "stop",
      checkpoint: saved,
      usage,
    });
  } finally {
    signal.removeEventListener("abort", interrupt);
    if (!settled && thread && turn)
      await server
        .request("turn/interrupt", { threadId: thread, turnId: turn }, 1000)
        .catch(() => {});
    await server.close();
    if (ephemeral) await rm(directory, { recursive: true, force: true });
  }
}
