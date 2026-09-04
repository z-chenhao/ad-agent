import { createInterface } from "node:readline";
const rl = createInterface({ input: process.stdin });
const send = (v) => process.stdout.write(JSON.stringify(v) + "\n");
let req;
rl.on("line", (line) => {
  const v = JSON.parse(line);
  if (v.type === "start") {
    req = v;
    if (v.prompt === "crash") process.exit(2);
    if (v.prompt === "malformed") {
      process.stdout.write("not-json\n");
      return;
    }
    if (v.prompt === "wait") return;
    if (v.prompt === "partial") {
      send({
        type: "tool_delta",
        id: "one",
        name: "present_metrics",
        arguments: { record_id: "report" },
      });
    }
    send({
      type: "tool_call",
      id: "one",
      name: "read",
      arguments: {},
      round: v.prompt === "budget" ? 2 : 1,
    });
  } else {
    if (req.prompt === "duplicate") {
      send({
        type: "tool_call",
        id: "one",
        name: "read",
        arguments: {},
        round: 1,
      });
      return;
    }
    send({
      type: "done",
      text: JSON.stringify(v.result),
      stop: "stop",
      usage: { input: 1, output: 1, cache_read: 0, cache_write: 0 },
    });
  }
});
