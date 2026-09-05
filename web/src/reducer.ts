import type { Card, Event, TurnResult, ViewContext } from "./types";
export type Activity =
  { kind: "tool"; id: string } | { kind: "message"; id: string; text: string };
export interface Live {
  turnId: string;
  seq: number;
  text: string;
  cards: Card[];
  tools: {
    id: string;
    name: string;
    role?: string;
    parent_id?: string;
    ok?: boolean;
    error?: string;
    duration_ms?: number;
    started_at?: string;
  }[];
  activity: Activity[];
  progress: { id: string; message: string; at: string }[];
  status: string;
  elapsed?: number;
  error_code?: string;
  context?: ViewContext;
}
export const emptyLive: Live = {
  turnId: "",
  seq: 0,
  text: "",
  cards: [],
  tools: [],
  activity: [],
  progress: [],
  status: "idle",
};
// A streamed placeholder owns its position. Completing or updating it must not
// reorder neighboring evidence while the operator is reading.
function upsertCard(cards: Card[], card: Card): Card[] {
  return cards.some((item) => item.id === card.id)
    ? cards.map((item) => (item.id === card.id ? card : item))
    : [...cards, card];
}
export function reduceEvent(state: Live, event: Event): Live {
  if (event.type === "client.reset") return emptyLive;
  if (event.v !== "1") return state;
  if (event.turnId === state.turnId && event.seq <= state.seq) return state;
  if (
    state.turnId &&
    event.turnId !== state.turnId &&
    event.type !== "turn.started"
  )
    return state;
  let next: Live = {
    ...(event.turnId !== state.turnId ? emptyLive : state),
    turnId: event.turnId,
    seq: event.seq,
  };
  switch (event.type) {
    case "turn.started":
      return { ...next, status: "running" };
    case "context.bound":
      return { ...next, context: event.data as ViewContext };
    case "text.delta": {
      const { text, id } = event.data as { text: string; id?: string };
      const last = next.activity.at(-1);
      return {
        ...next,
        text: next.text + text,
        activity:
          last?.kind === "message" && (!id || last.id === id)
            ? [
                ...next.activity.slice(0, -1),
                { ...last, text: last.text + text },
              ]
            : [
                ...next.activity,
                {
                  kind: "message",
                  id: id || `${event.turnId}-${event.seq}`,
                  text,
                },
              ],
      };
    }
    case "ui.upsert": {
      const c = event.data as Card;
      return {
        ...next,
        cards: upsertCard(next.cards, c),
      };
    }
    case "ui.partial": {
      const c = event.data as Card;
      if (!c.type) return next;
      return {
        ...next,
        cards: upsertCard(next.cards, c),
      };
    }
    case "tool.started": {
      const tool = event.data as Live["tools"][number];
      return {
        ...next,
        activity: next.activity.some(
          (item) => item.kind === "tool" && item.id === tool.id,
        )
          ? next.activity
          : [...next.activity, { kind: "tool", id: tool.id }],
        tools: [
          ...next.tools.filter(
            (item) => item.id !== (event.data as Live["tools"][number]).id,
          ),
          { ...tool, started_at: event.at },
        ],
      };
    }
    case "tool.finished": {
      const tool = event.data as Live["tools"][number];
      return {
        ...next,
        activity: next.activity.some(
          (item) => item.kind === "tool" && item.id === tool.id,
        )
          ? next.activity
          : [...next.activity, { kind: "tool", id: tool.id }],
        tools: next.tools.some((item) => item.id === tool.id)
          ? next.tools.map((item) =>
              item.id === tool.id ? { ...item, ...tool } : item,
            )
          : [...next.tools, tool],
      };
    }
    case "progress.updated": {
      const update = event.data as { message?: string };
      if (!update.message) return next;
      return {
        ...next,
        progress: [
          ...next.progress,
          {
            id: `${event.turnId}-${event.seq}`,
            message: update.message,
            at: event.at,
          },
        ],
      };
    }
    case "turn.completed": {
      const result = event.data as TurnResult;
      return {
        ...next,
        status: result.status,
        text: result.text,
        cards: result.cards?.length
          ? result.cards.reduce(
              upsertCard,
              next.cards.filter((card) =>
                result.cards.some((saved) => saved.id === card.id),
              ),
            )
          : next.cards.filter((card) => !card.pending),
        elapsed: result.elapsed_ms,
        error_code: result.error_code,
      };
    }
    default:
      return next;
  }
}
