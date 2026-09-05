import { test, expect } from "@playwright/test";
import { api, onSessionExpired, setCSRF, streamTurn } from "../src/api";

test("an expired session blocks subsequent requests without replaying a write", async () => {
  const original = globalThis.fetch;
  let requests = 0;
  let expired = 0;
  const unsubscribe = onSessionExpired(() => expired++);
  setCSRF("test-session");
  globalThis.fetch = async () => {
    requests++;
    return new Response("{}", { status: 401 });
  };
  try {
    await expect(
      api("/changes/draft/apply", { session_id: "web" }),
    ).rejects.toThrow("Session expired");
    await expect(api("/changes")).rejects.toThrow("Sign in");
    await expect(
      streamTurn(
        "web",
        "request",
        {
          provider: "openai-codex",
          model: "gpt-5.6-luna",
          reasoning: "medium",
          auth_mode: "chatgpt_oauth",
        },
        { page: "today" },
        new AbortController().signal,
        () => {},
      ),
    ).rejects.toThrow("Sign in");
    expect(requests).toBe(1);
    expect(expired).toBe(1);
  } finally {
    globalThis.fetch = original;
    unsubscribe();
    setCSRF("");
  }
});

test("a late unauthorized response cannot invalidate a new login", async () => {
  const original = globalThis.fetch;
  let respond!: (response: Response) => void;
  let expired = 0;
  const unsubscribe = onSessionExpired(() => expired++);
  setCSRF("old-session");
  globalThis.fetch = () =>
    new Promise((resolve) => {
      respond = resolve;
    });
  try {
    const pending = api("/changes");
    setCSRF("new-session");
    respond(new Response("{}", { status: 401 }));
    await expect(pending).rejects.toThrow("Session expired");
    globalThis.fetch = async () => Response.json({ ok: true });
    await expect(api("/changes")).resolves.toEqual({ ok: true });
    expect(expired).toBe(0);
  } finally {
    globalThis.fetch = original;
    unsubscribe();
    setCSRF("");
  }
});
