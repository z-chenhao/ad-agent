import { test, expect } from "@playwright/test";
import { reduceEvent, emptyLive } from "../src/reducer";
import type { Event } from "../src/types";
const event = (
  type: string,
  seq: number,
  data: unknown,
  turnId = "one",
): Event => ({ v: "1", type, seq, data, turnId, at: "2026-09-04" });
test("separate assistant messages do not merge when no tool separates them", () => {
  const state = [
    event("turn.started", 1, {}),
    event("text.delta", 2, { id: "commentary", text: "Checking." }),
    event("text.delta", 3, { id: "final", text: "All " }),
    event("text.delta", 4, { id: "final", text: "done." }),
    event("turn.completed", 5, {
      status: "completed",
      text: "All done.",
      elapsed_ms: 1,
    }),
  ].reduce(reduceEvent, emptyLive);
  expect(state.activity).toEqual([
    { kind: "message", id: "commentary", text: "Checking." },
    { kind: "message", id: "final", text: "All done." },
  ]);
});
test("replayed events are idempotent and new sessions clear all cards", () => {
  let s = reduceEvent(emptyLive, event("turn.started", 1, {}));
  s = reduceEvent(s, event("text.delta", 2, { text: "hello" }));
  s = reduceEvent(s, event("text.delta", 2, { text: "hello" }));
  expect(s.text).toBe("hello");
  s = reduceEvent(
    s,
    event("ui.upsert", 3, { id: "card", type: "unrecognized" }),
  );
  expect(s.cards).toHaveLength(1);
  s = reduceEvent(s, event("text.delta", 4, { text: "stale" }, "other"));
  expect(s.text).toBe("hello");
  s = reduceEvent(s, event("client.reset", 0, {}));
  expect(s).toEqual(emptyLive);
});

test("partial presentation is replaced by the trusted final card", () => {
  let state = reduceEvent(emptyLive, event("turn.started", 1, {}));
  state = reduceEvent(
    state,
    event("ui.partial", 2, {
      id: "card-one",
      type: "digest",
      pending: true,
    }),
  );
  expect(state.cards).toEqual([
    { id: "card-one", type: "digest", pending: true },
  ]);
  state = reduceEvent(
    state,
    event("ui.upsert", 3, {
      id: "card-one",
      type: "digest",
      digest: { title: "Today", items: [] },
    }),
  );
  expect(state.cards).toHaveLength(1);
  expect(state.cards[0]).toMatchObject({ id: "card-one" });
  expect(state.cards[0]?.pending).toBeUndefined();
});

test("out-of-order card completion never moves existing evidence", () => {
  const cards = [
    { id: "first", type: "digest" },
    { id: "second", type: "metrics" },
  ];
  const state = [
    event("turn.started", 1, {}),
    event("ui.partial", 2, { ...cards[0], pending: true }),
    event("ui.upsert", 3, cards[1]),
    event("ui.upsert", 4, cards[0]),
    event("turn.completed", 5, {
      status: "completed",
      text: "Decision",
      cards: [...cards].reverse(),
    }),
  ].reduce(reduceEvent, emptyLive);
  expect(state.cards).toEqual(cards);
});

test("tool and progress replay stays idempotent", () => {
  let state = reduceEvent(emptyLive, event("turn.started", 1, {}));
  state = reduceEvent(
    state,
    event("progress.updated", 2, { message: "Reading account context" }),
  );
  state = reduceEvent(
    state,
    event("tool.started", 3, { id: "read-1", name: "list_campaigns" }),
  );
  state = reduceEvent(
    state,
    event("tool.started", 3, { id: "read-1", name: "list_campaigns" }),
  );
  state = reduceEvent(
    state,
    event("tool.finished", 4, {
      id: "read-1",
      name: "list_campaigns",
      ok: true,
    }),
  );
  expect(state.progress.map((item) => item.message)).toEqual([
    "Reading account context",
  ]);
  expect(state.tools).toEqual([
    {
      id: "read-1",
      name: "list_campaigns",
      ok: true,
      started_at: "2026-09-04",
    },
  ]);
  expect(state.activity).toEqual([{ kind: "tool", id: "read-1" }]);
});

test("public commentary stays interleaved with tools after final answer and replay", () => {
  const events = [
    event("turn.started", 1, {}),
    event("text.delta", 2, { text: "Checking " }),
    event("text.delta", 3, { text: "delivery." }),
    event("tool.started", 4, { id: "read", name: "get_performance_report" }),
    event("tool.finished", 5, {
      id: "read",
      name: "get_performance_report",
      ok: true,
      duration_ms: 3,
    }),
    event("text.delta", 6, { text: "Here is the result." }),
    event("turn.completed", 7, {
      text: "Here is the result.",
      status: "completed",
      elapsed_ms: 2500,
    }),
  ];
  const result = events.reduce(reduceEvent, emptyLive);
  expect(result.text).toBe("Here is the result.");
  expect(result.activity).toEqual([
    { kind: "message", id: "one-2", text: "Checking delivery." },
    { kind: "tool", id: "read" },
    { kind: "message", id: "one-6", text: "Here is the result." },
  ]);
  expect(events.reduce(reduceEvent, result)).toEqual(result);
  expect(result.tools[0]?.duration_ms).toBe(3);
});

test("a completed event without cards does not erase streamed cards", () => {
  let state = reduceEvent(emptyLive, event("turn.started", 1, {}));
  state = reduceEvent(
    state,
    event("ui.upsert", 2, { id: "card-one", type: "digest" }),
  );
  state = reduceEvent(
    state,
    event("turn.completed", 3, {
      turn_id: "one",
      session_id: "web",
      status: "completed",
      text: "Done",
      cards: [],
      elapsed_ms: 12,
      usage: { input: 0, output: 0, cache_read: 0, cache_write: 0 },
    }),
  );
  expect(state.cards).toEqual([{ id: "card-one", type: "digest" }]);
});
