import { createInterface } from "node:readline";

const input = createInterface({ input: process.stdin, crlfDelay: Infinity });
const send = (value) => process.stdout.write(JSON.stringify(value) + "\n");
let state = { version: 1, assistants: [] };
let model = { provider: "openai-codex", model: "gpt-5.6-luna" };

input.on("line", (line) => {
  const frame = JSON.parse(line);
  if (frame.type === "start") {
    state = frame.provider_state ?? state;
    model = frame.model ?? model;
    send({ type: "ready" });
    return;
  }
  if (frame.type !== "complete") process.exit(2);
  const messages = frame.request.messages;
  const last = messages.at(-1);
  if (last?.role === "user" && last.content?.[0]?.text === "hang") return;
  const hasAssistant = messages.some((message) => message.role === "assistant");
  if (!hasAssistant && (frame.request.tools?.length ?? 0) > 0) {
    const userText = messages.findLast((message) => message.role === "user")
      ?.content?.[0]?.text;
    const calls =
      userText === "parallel"
        ? [
            { id: "call-1", name: "read_data", arguments: { key: "one" } },
            { id: "call-2", name: "read_data", arguments: { key: "two" } },
          ]
        : [{ id: "call-1", name: "read_data", arguments: { key: "value" } }];
    const message = {
      role: "assistant",
      content: calls.map((toolCall) => ({ type: "tool_call", toolCall })),
    };
    if (userText === "content_with_tools")
      message.content.unshift(
        { type: "reasoning", text: "private reasoning must stay private" },
        { type: "text", text: "I will inspect the account." },
      );
    state.assistants.push({ fake: "tool" });
    send({
      type: "done",
      id: frame.id,
      response: {
        message,
        provider: model.provider,
        model: model.model,
        stopReason: "tool_calls",
        usage: {
          inputTokens: 2,
          outputTokens: 3,
          totalTokens: 5,
          cachedInputTokens: 1,
        },
      },
      provider_state: state,
    });
    return;
  }
  send({
    type: "delta",
    id: frame.id,
    delta: { type: "text", index: 0, delta: "done" },
  });
  const message = {
    role: "assistant",
    content: [{ type: "text", text: "done" }],
  };
  state.assistants.push({ fake: "text" });
  send({
    type: "done",
    id: frame.id,
    response: {
      message,
      provider: model.provider,
      model: model.model,
      stopReason: "stop",
      usage: {
        inputTokens: 4,
        outputTokens: 1,
        totalTokens: 5,
        cachedInputTokens: 2,
      },
    },
    provider_state: state,
  });
});
