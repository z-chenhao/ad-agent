import type { Card, Event, TurnResult } from "./types";
export interface Live {
  turnId: string;
  seq: number;
  text: string;
  cards: Card[];
  tools: { id: string; name: string; role?: string; ok?: boolean }[];
  status: string;
  elapsed?: number;
}
export const emptyLive: Live = {
  turnId: "",
  seq: 0,
  text: "",
  cards: [],
  tools: [],
  status: "idle",
};
export function reduceEvent(state: Live, event: Event): Live {
  if (event.type === "client.reset") return emptyLive;
  if (event.v !== "0") return state;
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
    case "text.delta":
      return {
        ...next,
        text: next.text + (event.data as { text: string }).text,
      };
    case "ui.upsert": {
      const c = event.data as Card;
      return {
        ...next,
        cards: [
          ...next.cards.filter(
            (x) => x.id !== c.id && !(x.pending && x.type === c.type),
          ),
          c,
        ],
      };
    }
    case "ui.partial": {
      const c = event.data as Card;
      if (!c.type || next.cards.some((x) => x.type === c.type && !x.pending))
        return next;
      return {
        ...next,
        cards: [...next.cards.filter((x) => x.id !== c.id), c],
      };
    }
    case "tool.started":
      return {
        ...next,
        tools: [...next.tools, event.data as Live["tools"][number]],
      };
    case "tool.finished": {
      const tool = event.data as Live["tools"][number];
      return {
        ...next,
        tools: next.tools.map((x) =>
          x.id === tool.id ? { ...x, ...tool } : x,
        ),
      };
    }
    case "turn.completed": {
      const result = event.data as TurnResult;
      return {
        ...next,
        status: result.status,
        text: result.text,
        cards: result.cards,
        elapsed: result.elapsed_ms,
      };
    }
    default:
      return next;
  }
}
