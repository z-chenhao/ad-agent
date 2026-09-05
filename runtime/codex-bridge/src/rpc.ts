import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";

type ID = number | string;
interface Message {
  id?: ID;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: unknown;
}

// Bidirectional JSON-RPC is internal to this adapter. It is never an HTTP route
// and never forwards raw native events/errors to the product.
export class AppServer {
  private process: ChildProcessWithoutNullStreams;
  private next = 0;
  private pending = new Map<
    ID,
    {
      method: string;
      resolve: (value: unknown) => void;
      reject: (error: Error) => void;
    }
  >();
  private failure?: Error;
  onNotification: (method: string, params: unknown) => void = () => {};
  onRequest: (method: string, params: unknown) => Promise<unknown> =
    async () => {
      throw new Error("unsupported_native_request");
    };
  onFailure: (error: Error) => void = () => {};

  constructor(
    binary: string,
    args: string[],
    cwd: string,
    env: NodeJS.ProcessEnv,
  ) {
    this.process = spawn(binary, args, { cwd, env, stdio: "pipe" });
    // Native diagnostics can contain prompts or auth details. Do not mirror them.
    this.process.stderr.resume();
    this.process.stdin.on("error", () =>
      this.fail(new Error("native_input_closed")),
    );
    const input = createInterface({
      input: this.process.stdout,
      crlfDelay: Infinity,
    });
    input.on("line", (line) => {
      try {
        if (Buffer.byteLength(line) > 8 * 1024 * 1024)
          throw new Error("native_frame_limit");
        const message = JSON.parse(line) as Message;
        if (message.method && message.id !== undefined) {
          void this.onRequest(message.method, message.params)
            .then(
              (result) => this.send({ id: message.id, result }),
              () =>
                this.send({
                  id: message.id,
                  error: {
                    code: -32601,
                    message: "Request unavailable in Ad Agent",
                  },
                }),
            )
            .catch(() => this.fail(new Error("native_reply_failed")));
        } else if (message.method)
          this.onNotification(message.method, message.params);
        else if (message.id !== undefined) {
          const handler = this.pending.get(message.id);
          if (!handler) throw new Error("native_response_correlation_failed");
          this.pending.delete(message.id);
          if (message.error)
            handler.reject(
              new Error("native_request_failed:" + handler.method, {
                cause: message.error,
              }),
            );
          else handler.resolve(message.result);
        } else throw new Error("invalid_native_frame");
      } catch {
        this.fail(new Error("native_protocol_failed"));
      }
    });
    this.process.on("error", () => this.fail(new Error("native_start_failed")));
    this.process.on("exit", () => this.fail(new Error("native_exited")));
  }
  private fail(error: Error) {
    if (this.failure) return;
    this.failure = error;
    for (const handler of this.pending.values()) handler.reject(error);
    this.pending.clear();
    this.onFailure(error);
  }
  private send(message: Message) {
    if (this.failure) throw this.failure;
    this.process.stdin.write(JSON.stringify(message) + "\n");
  }
  notify(method: string, params: unknown = {}) {
    this.send({ method, params });
  }
  request<T>(method: string, params: unknown, timeout = 15_000): Promise<T> {
    const id = ++this.next;
    return new Promise<unknown>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error("native_request_timeout"));
      }, timeout);
      this.pending.set(id, {
        method,
        resolve: (result) => {
          clearTimeout(timer);
          resolve(result);
        },
        reject: (err) => {
          clearTimeout(timer);
          reject(err);
        },
      });
      try {
        this.send({ id, method, params });
      } catch {
        this.pending.get(id)?.reject(new Error("native_send_failed"));
        this.pending.delete(id);
      }
    }) as Promise<T>;
  }
  async close() {
    this.onFailure = () => {};
    this.fail(new Error("native_closed"));
    if (this.process.exitCode !== null || this.process.signalCode !== null)
      return;
    await new Promise<void>((resolve) => {
      const timer = setTimeout(() => this.process.kill("SIGKILL"), 1500);
      this.process.once("exit", () => {
        clearTimeout(timer);
        resolve();
      });
      this.process.stdin.end();
      this.process.kill("SIGTERM");
    });
  }
}
