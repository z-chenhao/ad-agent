import { test, expect } from "@playwright/test";
import { reduceEvent, emptyLive } from "../src/reducer";
import type { Event } from "../src/types";
const event = (
  type: string,
  seq: number,
  data: unknown,
  turnId = "one",
): Event => ({ v: "0", type, seq, data, turnId, at: "2026-09-04" });
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
