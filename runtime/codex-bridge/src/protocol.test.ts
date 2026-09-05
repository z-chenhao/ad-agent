import assert from "node:assert/strict";
import { test } from "node:test";
import { createServer } from "node:http";
import { mkdtemp, mkdir, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { run, oauth } from "./adapter.js";
import type { ModelRuntime } from "@earendil-works/pi-coding-agent";
import { isolatedConfig, parseInput, type Start } from "./protocol.js";

const template: Start = {
  type: "start",
  system:
    "You are an advertising assistant. Only use supplied advertising tools.",
  prompt: "Read the account.",
  max_rounds: 0,
  tools: [
    {
      name: "read_account",
      description: "Read advertising account",
      parameters: {
        type: "object",
        properties: {},
        additionalProperties: false,
      },
    },
  ],
  model: {
    provider: "test-provider",
    model: "gpt-5.6-luna",
    reasoning: "medium",
    auth_mode: "api_key",
    api: "openai-responses",
    base_url: "http://127.0.0.1:1/v1",
    api_key_env: "AD_AGENT_CODEX_TEST_KEY",
    context_window: 128000,
    max_output_tokens: 4096,
  },
};
test("OAuth hydrates the saved credential snapshot before testing its type", async () => {
  let initialized = false;
  const token = `header.${Buffer.from(JSON.stringify({ "https://api.openai.com/auth": { chatgpt_account_id: "test-account" } })).toString("base64url")}.signature`;
  const resolve = await oauth(async (options) => {
    initialized = options?.refreshOnCreate !== false;
    return {
      isUsingOAuth: () => initialized,
      getAuth: async () => ({ auth: { apiKey: token } }),
    } as unknown as ModelRuntime;
  });
  assert.equal((await resolve()).chatgptAccountId, "test-account");
});
test("Codex requires explicit Responses transport and disabled environment capabilities", () => {
  assert.deepEqual(parseInput(JSON.stringify(template)), template);
  assert.throws(() =>
    parseInput(
      JSON.stringify({
        ...template,
        model: { ...template.model, api: "openai-completions" },
      }),
    ),
  );
  const config = isolatedConfig(template);
  for (const feature of [
    "shell_tool",
    "multi_agent",
    "multi_agent_v2",
    "apps",
    "plugins",
    "hooks",
    "code_mode",
    "image_generation",
  ])
    assert.equal(config[`features.${feature}`], false);
  assert.equal(config.web_search, "disabled");
});

// Pinned native App Server + HTTP fake, not a replacement runtime or provider probe.
test(
  "native Codex sends only allowed tools, pairs results and forks private checkpoints",
  { timeout: 40_000 },
  async () => {
    const root = await mkdtemp(join(tmpdir(), "ad-agent-codex-test-"));
    const bodies: Record<string, unknown>[] = [];
    let requests = 0;
    const server = createServer(async (request, response) => {
      if (!request.url?.endsWith("/responses")) {
        response.writeHead(404).end();
        return;
      }
      const chunks: Buffer[] = [];
      for await (const chunk of request) chunks.push(Buffer.from(chunk));
      bodies.push(
        JSON.parse(Buffer.concat(chunks).toString()) as Record<string, unknown>,
      );
      requests++;
      response.writeHead(200, { "Content-Type": "text/event-stream" });
      const emit = (type: string, data: unknown) =>
        response.write(
          `event: ${type}\ndata: ${JSON.stringify({ type, ...(data as object) })}\n\n`,
        );
      emit("response.created", { response: { id: `response-${requests}` } });
      const item =
        requests === 1
          ? {
              type: "custom_tool_call",
              id: "function-1",
              call_id: "call-1",
              name: "exec",
              input: "text(await tools.read_account({}));",
            }
          : {
              type: "message",
              id: `message-${requests}`,
              role: "assistant",
              phase: "final_answer",
              content: [
                { type: "output_text", text: "Account read successfully." },
              ],
            };
      if (requests === 1) {
        const commentary = {
          type: "message",
          id: "commentary-1",
          role: "assistant",
          phase: "commentary",
          content: [
            { type: "output_text", text: "I will inspect the account." },
          ],
        };
        emit("response.output_item.added", {
          output_index: 1,
          item: commentary,
        });
        emit("response.output_item.done", {
          output_index: 1,
          item: commentary,
        });
      }
      emit("response.output_item.added", { output_index: 0, item });
      emit("response.output_item.done", { output_index: 0, item });
      emit("response.completed", {
        response: {
          id: `response-${requests}`,
          status: "completed",
          output: [item],
          usage: {
            input_tokens: 100,
            output_tokens: 10,
            total_tokens: 110,
            input_tokens_details: { cached_tokens: 5 },
          },
        },
      });
      response.end();
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    const address = server.address() as { port: number };
    const originalKey = process.env.AD_AGENT_CODEX_TEST_KEY;
    process.env.AD_AGENT_CODEX_TEST_KEY = "test-only-not-a-real-key";
    try {
      const frames: {
        type?: string;
        checkpoint?: string;
        text?: string;
        usage?: Record<string, number>;
      }[] = [];
      const req = {
        ...template,
        model: {
          ...template.model,
          base_url: `http://127.0.0.1:${address.port}/v1`,
        },
        session_dir: join(root, "first"),
      };
      let calls = 0;
      await run(
        req,
        {
          emit: (frame) => frames.push(frame as (typeof frames)[number]),
          execute: async (id, name) => {
            calls++;
            assert.ok(id);
            assert.equal(name, "read_account");
            return { ok: true, data: { account: "Aster & Pine Home" } };
          },
        },
        AbortSignal.timeout(25_000),
      );
      assert.equal(calls, 1);
      assert.equal(requests, 2);
      assert.equal(
        frames
          .filter((frame) => frame.type === "text_delta")
          .map((frame) => frame.text)
          .join(""),
        "I will inspect the account.Account read successfully.",
      );
      const done = frames.find((frame) => frame.type === "done");
      assert.equal(done?.text, "Account read successfully.");
      assert.deepEqual(done?.usage, {
        input: 190,
        output: 20,
        cache_read: 10,
        cache_write: 0,
      });
      assert.ok(done?.checkpoint);
      const saved = JSON.parse(await readFile(done.checkpoint, "utf8"));
      assert.equal(saved.runtime, "codex");
      const body = bodies[0]!;
      const tools = (
        body.input as {
          type: string;
          tools?: { name: string; description: string }[];
        }[]
      )
        .filter((item) => item.type === "additional_tools")
        .flatMap((item) => item.tools ?? []);
      // Luna's native catalog selects Code Mode even with the feature default off.
      // Its V8 wrapper cannot access filesystem/network, and nested capabilities
      // remain the advertising tool plus inert plan/empty-skill-catalog utilities.
      assert.deepEqual(tools.map((tool) => tool.name).sort(), ["exec", "wait"]);
      const declarations = tools[0]!.description
        .match(/declare const tools: \{ ([a-zA-Z0-9_]+)/g)
        ?.map((value) => value.split(" ").at(-1));
      assert.deepEqual(
        declarations?.sort(),
        ["read_account", "update_plan", "skills__list", "skills__read"].sort(),
      );
      const serialized = JSON.stringify(body);
      assert.ok(serialized.includes(template.system));
      assert.ok(!serialized.includes("Personal Engineering Constitution"));
      assert.ok(!serialized.includes("Available skills"));
      assert.ok(!serialized.includes(process.env.AD_AGENT_CODEX_TEST_KEY!));
      assert.ok(
        JSON.stringify(bodies[1]).includes("Aster &amp; Pine Home") ||
          JSON.stringify(bodies[1]).includes("Aster & Pine Home"),
      );
      const second = {
        ...req,
        session_dir: join(root, "second"),
        checkpoint: done.checkpoint,
        prompt: "What account did you read?",
      };
      const secondFrames: typeof frames = [];
      await run(
        second,
        {
          emit: (frame) => secondFrames.push(frame as (typeof frames)[number]),
          execute: async () => {
            throw new Error("unexpected tool");
          },
        },
        AbortSignal.timeout(15_000),
      );
      const secondDone = secondFrames.find((frame) => frame.type === "done");
      assert.equal(secondDone?.text, "Account read successfully.");
      assert.deepEqual(secondDone?.usage, {
        input: 95,
        output: 10,
        cache_read: 5,
        cache_write: 0,
      });
      const fork = JSON.parse(await readFile(secondDone!.checkpoint!, "utf8"));
      assert.notEqual(fork.thread, saved.thread);
      assert.deepEqual(
        JSON.parse(await readFile(done.checkpoint, "utf8")),
        saved,
      );
      await assert.rejects(
        run(
          { ...second, system: "Changed system" },
          { emit: () => {}, execute: async () => ({ ok: true }) },
          AbortSignal.timeout(1000),
        ),
        /checkpoint_binding_mismatch/,
      );
    } finally {
      if (originalKey === undefined) delete process.env.AD_AGENT_CODEX_TEST_KEY;
      else process.env.AD_AGENT_CODEX_TEST_KEY = originalKey;
      server.closeAllConnections();
      await new Promise<void>((resolve) => server.close(() => resolve()));
      await rm(root, { recursive: true, force: true });
    }
  },
);
