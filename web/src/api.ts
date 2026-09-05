import type { Event, ModelSelection, ViewContext } from "./types";
let csrf = "";
let authGeneration = 0;
const expiredListeners = new Set<() => void>();
export function setCSRF(value: string) {
  csrf = value;
  authGeneration++;
}
export function onSessionExpired(listener: () => void) {
  expiredListeners.add(listener);
  return () => {
    expiredListeners.delete(listener);
  };
}
function checkSession(response: Response, generation: number) {
  if (response.status !== 401) return;
  // Ignore a late response from a previous login. Never replay a failed write.
  if (csrf && generation === authGeneration) {
    setCSRF("");
    for (const listener of expiredListeners) listener();
  }
  throw new Error("Session expired. Sign in again; no request was retried.");
}
export async function api<T>(
  path: string,
  body?: unknown,
  signal?: AbortSignal,
): Promise<T> {
  if (!csrf && path !== "/auth" && path !== "/login")
    throw new Error("Sign in to the local workspace.");
  const generation = authGeneration;
  const response = await fetch("/api/v1" + path, {
    method: body === undefined ? "GET" : "POST",
    credentials: "same-origin",
    headers:
      body === undefined
        ? {}
        : { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    signal,
  });
  checkSession(response, generation);
  const result: unknown = await response.json();
  if (!response.ok)
    throw new Error((result as { error?: string }).error ?? "request_failed");
  return result as T;
}
export async function streamTurn(
  sessionId: string,
  message: string,
  model: ModelSelection,
  viewContext: ViewContext,
  signal: AbortSignal,
  emit: (event: Event) => void,
  runtime?: string,
) {
  if (!csrf) throw new Error("Sign in to the local workspace.");
  const generation = authGeneration;
  const response = await fetch("/api/v1/agent/turn", {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/json", "X-CSRF-Token": csrf },
    body: JSON.stringify({
      session_id: sessionId,
      message,
      model,
      view_context: viewContext,
      runtime,
    }),
    signal,
  });
  checkSession(response, generation);
  if (!response.ok || !response.body) {
    const problem = (await response.json().catch(() => ({}))) as {
      error?: string;
    };
    throw new Error(
      problem.error?.replaceAll("_", " ") ??
        "Unable to start the session. Check Settings and local login.",
    );
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let terminal = false;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value, { stream: !done });
      let at: number;
      while ((at = buffer.indexOf("\n\n")) >= 0) {
        const block = buffer.slice(0, at);
        buffer = buffer.slice(at + 2);
        const raw = block
          .split("\n")
          .filter((s) => s.startsWith("data: "))
          .map((s) => s.slice(6))
          .join("\n");
        if (!raw) continue;
        const event = JSON.parse(raw) as Event;
        if (event.type) {
          emit(event);
          if (event.type === "turn.completed") terminal = true;
        } else if (!terminal)
          throw new Error(
            "The turn failed. Refresh the session to inspect saved state.",
          );
      }
      if (buffer.length > 1_048_576)
        throw new Error("Event stream exceeded the size limit.");
      if (done) break;
    }
  } finally {
    reader.releaseLock();
  }
  if (!terminal)
    throw new Error(
      "Connection interrupted; success is unconfirmed. Refresh to recover saved records.",
    );
}
