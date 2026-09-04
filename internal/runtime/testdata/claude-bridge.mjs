import { createInterface } from "node:readline";
import { mkdir, writeFile } from "node:fs/promises";
import { join } from "node:path";

const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
const send = (value) => process.stdout.write(JSON.stringify(value) + "\n");
let started = false;
input.on("line", async (line) => {
  const frame = JSON.parse(line);
  if (frame.type === "start" && !started) {
    started = true;
    send({ type: "text_delta", text: "working" });
    send({ type: "tool_call", id: "claude-call", name: "read_data", arguments: {}, round: 1 });
    return;
  }
  if (frame.type === "tool_result" && frame.id === "claude-call") {
    const dir = process.env.CLAUDE_FAKE_SESSION_DIR;
    await mkdir(dir, { recursive: true });
    const checkpoint = join(dir, "claude-checkpoint.json");
    await writeFile(checkpoint, JSON.stringify({ version: 1 }));
    send({ type: "done", text: "done", stop: "stop", checkpoint, usage: { input: 1, output: 2, cache_read: 3, cache_write: 4 } });
    input.close();
  }
});
