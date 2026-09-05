import { spawn } from "node:child_process";
const child = spawn(process.execPath, ["-e", "setInterval(() => {}, 1000)"], {
  stdio: "ignore",
});
process.stdout.write(
  JSON.stringify({
    type: "tool_call",
    id: "child",
    name: "child_pid",
    round: 1,
    arguments: { pid: child.pid },
  }) + "\n",
);
setInterval(() => {}, 1000);
