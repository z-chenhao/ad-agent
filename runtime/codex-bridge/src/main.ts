import { createInterface } from "node:readline";
import { EnvHttpProxyAgent, install, setGlobalDispatcher } from "undici";
import { run } from "./adapter.js";
import { parseInput, type Reply } from "./protocol.js";

process.umask(0o077);
setGlobalDispatcher(
  new EnvHttpProxyAgent({
    allowH2: false,
    headersTimeout: 300_000,
    bodyTimeout: 300_000,
  }),
);
install();
const controller = new AbortController();
const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
const pending = new Map<string, (result: Reply["result"]) => void>();
const send = (frame: unknown) =>
  process.stdout.write(JSON.stringify(frame) + "\n");
let started = false;
const abort = () => {
  controller.abort();
  for (const resolve of pending.values())
    resolve({ ok: false, error: "cancelled" });
  pending.clear();
};
process.on("SIGTERM", abort);
process.on("SIGINT", abort);
input.on("close", abort);
input.on("line", (line) => {
  try {
    const message = parseInput(line);
    if (message.type === "tool_result") {
      const resolve = pending.get(message.id);
      if (!resolve) throw new Error("unknown_tool_reply");
      pending.delete(message.id);
      resolve(message.result);
    } else {
      if (started) throw new Error("duplicate_start");
      started = true;
      void run(
        message,
        {
          emit: send,
          execute: (id, name, args, round) =>
            new Promise((resolve) => {
              if (pending.has(id)) throw new Error("duplicate_tool_call");
              pending.set(id, resolve);
              send({ type: "tool_call", id, name, arguments: args, round });
            }),
        },
        controller.signal,
      )
        .catch((error: unknown) => {
          const code = error instanceof Error ? error.message : "";
          const allowed = new Set([
            "chatgpt_oauth_required",
            "oauth_account_changed",
            "api_key_missing",
            "native_start_failed",
            "native_request_timeout",
            "native_turn_failed",
            "native_context_isolation_failed",
            "native_tool_boundary_violation",
            "native_protocol_failed",
            "unexpected_model_reroute",
            "model_text_limit_exceeded",
          ]);
          send({
            type: "error",
            error: allowed.has(code) ? code : "runtime_failed",
          });
        })
        .finally(() => {
          input.close();
          process.exitCode = 0;
        });
    }
  } catch {
    send({ type: "error", error: "invalid_bridge_input" });
    abort();
    input.close();
  }
});
