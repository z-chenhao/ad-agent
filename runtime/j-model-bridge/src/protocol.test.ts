import test from "node:test";
import assert from "node:assert/strict";
import type { AssistantMessage } from "@earendil-works/pi-ai";
import {
  buildContext,
  parseInput,
  parseState,
  toJMessage,
} from "./protocol.js";

const selection = {
  provider: "openai-codex" as const,
  model: "gpt-5.6-luna",
  reasoning: "medium" as const,
  auth_mode: "chatgpt_oauth" as const,
};

function assistant(): AssistantMessage {
  return {
    role: "assistant",
    api: "openai-codex-responses",
    provider: "openai-codex",
    model: "gpt-5.6-luna",
    responseId: "private",
    content: [
      { type: "thinking", thinking: "private", thinkingSignature: "opaque" },
      {
        type: "toolCall",
        id: "call-1",
        name: "read_report",
        arguments: { b: 2, a: 1 },
        thoughtSignature: "opaque",
      },
    ],
    usage: {
      input: 1,
      output: 2,
      cacheRead: 0,
      cacheWrite: 0,
      totalTokens: 3,
      cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 },
    },
    stopReason: "toolUse",
    timestamp: 1,
  };
}

test("replays only matching native assistant state", () => {
  const native = assistant();
  const j = toJMessage(native, selection);
  const context = buildContext(
    {
      messages: [
        { role: "system", content: [{ type: "text", text: "system" }] },
        { role: "user", content: [{ type: "text", text: "read" }] },
        j,
        {
          role: "tool",
          content: [{ type: "text", text: "result" }],
          toolCallId: "call-1",
          toolName: "read_report",
        },
      ],
    },
    { version: 1, assistants: [native] },
    selection,
  );
  assert.equal(context.systemPrompt, "system");
  assert.equal(context.messages[1], native);
  assert.equal(context.messages[2]?.role, "toolResult");
});

test("rejects mismatched and non-Codex state", () => {
  const native = assistant();
  assert.throws(() =>
    buildContext(
      {
        messages: [
          { role: "user", content: [{ type: "text", text: "read" }] },
          { role: "assistant", content: [{ type: "text", text: "changed" }] },
        ],
      },
      { version: 1, assistants: [native] },
      selection,
    ),
  );
  assert.throws(() =>
    parseState(
      { version: 1, assistants: [{ ...native, provider: "other" }] },
      selection,
    ),
  );
});

test("bounds and validates frames", () => {
  assert.equal(
    parseInput(JSON.stringify({ type: "start", model: selection })).type,
    "start",
  );
  assert.throws(() => parseInput('{"type":"start"}'));
  assert.throws(() =>
    parseInput(
      JSON.stringify({
        type: "start",
        model: { ...selection, model: "unknown" },
      }),
    ),
  );
  assert.throws(() => parseInput('{"type":"complete","id":"","request":{}}'));
  assert.throws(() => parseInput("x".repeat(8 * 1024 * 1024 + 1)));
});

test("normalizes omitted empty reasoning text from Go", () => {
  const native = assistant();
  native.content = [
    { type: "thinking", thinking: "", thinkingSignature: "opaque" },
  ];
  assert.doesNotThrow(() =>
    buildContext(
      {
        messages: [
          { role: "user", content: [{ type: "text", text: "read" }] },
          { role: "assistant", content: [{ type: "reasoning" }] },
        ],
      },
      { version: 1, assistants: [native] },
      selection,
    ),
  );
});
