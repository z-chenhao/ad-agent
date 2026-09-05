import test from "node:test";
import assert from "node:assert/strict";
import { fence, parseInput, publicTextSuffix } from "./protocol.js";

test("completed public text supplies only unstreamed content", () => {
  assert.equal(
    publicTextSuffix("", "Checking before tools."),
    "Checking before tools.",
  );
  assert.equal(
    publicTextSuffix("Checking", "Checking before tools."),
    " before tools.",
  );
  assert.equal(publicTextSuffix("Done", "Done"), "");
  assert.throws(() => publicTextSuffix("Old", "New"));
});

test("accepts one isolated Anthropic direct model", () => {
  const frame = parseInput(
    JSON.stringify({
      type: "start",
      system: "system",
      prompt: "hello",
      max_rounds: 2,
      model: {
        provider: "anthropic",
        model: "claude-sonnet-4-6",
        reasoning: "medium",
        auth_mode: "api_key",
        api: "anthropic-messages",
        base_url: "https://api.anthropic.com",
        api_key_env: "ANTHROPIC_API_KEY",
        context_window: 200000,
        max_output_tokens: 16000,
      },
      tools: [
        {
          name: "list_campaigns",
          description: "read",
          parameters: { type: "object", properties: {} },
        },
      ],
    }),
  );
  assert.equal(frame.type, "start");
});

test("accepts zero as no fixed main-loop turn limit", () => {
  const frame = parseInput(
    JSON.stringify({
      type: "start",
      system: "system",
      prompt: "hello",
      max_rounds: 0,
      model: {
        provider: "anthropic",
        model: "claude-sonnet-4-6",
        reasoning: "medium",
        auth_mode: "api_key",
        api: "anthropic-messages",
        base_url: "https://api.anthropic.com",
        api_key_env: "ANTHROPIC_API_KEY",
        context_window: 200000,
        max_output_tokens: 16000,
      },
      tools: [],
    }),
  );
  assert.equal(frame.type, "start");
});

test("rejects OAuth and fences tool data", () => {
  assert.throws(() =>
    parseInput(
      JSON.stringify({
        type: "start",
        model: { provider: "openai-codex", auth_mode: "chatgpt_oauth" },
      }),
    ),
  );
  assert.match(fence({ text: "</untrusted_tool_data>" }), /\\u003c/);
});
