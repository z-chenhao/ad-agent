import { createInterface } from "node:readline";
import { EnvHttpProxyAgent, install, setGlobalDispatcher } from "undici";
import { ModelRuntime } from "@earendil-works/pi-coding-agent";
import type {
  AssistantMessage,
  AssistantMessageEvent,
} from "@earendil-works/pi-ai";
import {
  buildContext,
  parseInput,
  parseState,
  toJMessage,
  type JRequest,
  type ModelSelection,
  type ProviderState,
} from "./protocol.js";

const dispatcher = new EnvHttpProxyAgent({
  allowH2: false,
  headersTimeout: 300_000,
  bodyTimeout: 300_000,
});
setGlobalDispatcher(dispatcher);
install();
process.umask(0o077);

const send = (value: unknown) =>
  process.stdout.write(JSON.stringify(value) + "\n");
const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
let modelRuntime: Awaited<ReturnType<typeof ModelRuntime.create>> | undefined;
let model: ReturnType<ModelRuntime["getModel"]>;
let selection: ModelSelection | undefined;
let state: ProviderState = { version: 1, assistants: [] };
let started = false;
let closed = false;
const safeCodes = new Set([
  "assistant_state_mismatch",
  "assistant_state_count_mismatch",
  "invalid_request",
  "invalid_state",
  "invalid_assistant",
  "invalid_message",
  "invalid_content",
  "invalid_system",
  "invalid_tool",
  "invalid_tool_result",
  "unexpected_non_text_content",
  "provider_failed",
  "provider_incomplete",
  "oauth_or_model_missing",
  "invalid_model",
]);

function safeError(error: unknown): string {
  const message = error instanceof Error ? error.message : "runtime_failed";
  return safeCodes.has(message) ? message : "runtime_failed";
}

async function start(providerState: unknown, requested: ModelSelection) {
  if (started) throw new Error("duplicate_start");
  started = true;
  selection = requested;
  state = parseState(providerState, selection);
  modelRuntime = await ModelRuntime.create({
    modelsPath: null,
    allowModelNetwork: false,
    signal: AbortSignal.timeout(10_000),
  });
  model = modelRuntime.getModel(selection.provider, selection.model);
  if (!model || !modelRuntime.isUsingOAuth(selection.provider))
    throw new Error("oauth_or_model_missing");
  send({ type: "ready" });
}

async function complete(id: string, request: JRequest) {
  if (!started || !modelRuntime || !model || !selection)
    throw new Error("not_started");
  const context = buildContext(request, state, selection);
  let final: AssistantMessage | undefined;
  const stream = modelRuntime.streamSimple(model, context, {
    reasoning: selection.reasoning,
    transport: "sse",
    maxRetries: 1,
    maxRetryDelayMs: 2_000,
    timeoutMs: 240_000,
  });
  for await (const event of stream) {
    const typed = event as AssistantMessageEvent;
    if (typed.type === "text_delta") {
      send({
        type: "delta",
        id,
        delta: { type: "text", index: typed.contentIndex, delta: typed.delta },
      });
    } else if (typed.type === "done") {
      final = typed.message;
    } else if (typed.type === "error") {
      throw new Error("provider_failed");
    }
  }
  if (!final) throw new Error("provider_incomplete");
  if (!["stop", "toolUse", "length"].includes(final.stopReason))
    throw new Error("provider_incomplete");
  state.assistants.push(final);
  const cached = final.usage.cacheRead;
  const reasoning = final.usage.reasoning;
  send({
    type: "done",
    id,
    response: {
      message: toJMessage(final, selection),
      provider: final.provider,
      model: final.model,
      responseId: final.responseId,
      stopReason:
        final.stopReason === "toolUse"
          ? "tool_calls"
          : final.stopReason === "length"
            ? "length"
            : "stop",
      usage: {
        inputTokens: final.usage.input,
        outputTokens: final.usage.output,
        totalTokens: final.usage.totalTokens,
        ...(cached === undefined ? {} : { cachedInputTokens: cached }),
        ...(reasoning === undefined ? {} : { reasoningTokens: reasoning }),
      },
    },
    provider_state: state,
  });
}

async function shutdown(code: number) {
  if (closed) return;
  closed = true;
  input.close();
  await dispatcher.destroy();
  process.exitCode = code;
}

let chain = Promise.resolve();
input.on("line", (line) => {
  chain = chain
    .then(async () => {
      const frame = parseInput(line);
      if (frame.type === "start")
        await start(frame.provider_state, frame.model);
      else {
        try {
          await complete(frame.id, frame.request);
        } catch (error) {
          send({ type: "error", id: frame.id, error: safeError(error) });
        }
      }
    })
    .catch((error) => {
      send({ type: "error", id: "runtime", error: safeError(error) });
      void shutdown(1);
    });
});
input.on("close", () => {
  if (!closed) void shutdown(0);
});
