export interface ToolSpec {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
}
export interface Start {
  type: "start";
  system: string;
  prompt: string;
  model: {
    provider: "openai-codex";
    model: string;
    reasoning: "medium";
  };
  tools: ToolSpec[];
  max_rounds: number;
  checkpoint?: string;
  session_dir?: string;
}
export interface Reply {
  type: "tool_result";
  id: string;
  result: { ok: boolean; data?: unknown; error?: string; close?: boolean };
}
export function parseInput(line: string): Start | Reply {
  if (Buffer.byteLength(line) > 1_048_576) throw new Error("frame_limit");
  const v: unknown = JSON.parse(line);
  if (!v || typeof v !== "object") throw new Error("invalid_frame");
  const o = v as Record<string, unknown>;
  if (
    o.type === "tool_result" &&
    typeof o.id === "string" &&
    o.result &&
    typeof o.result === "object" &&
    typeof (o.result as Record<string, unknown>).ok === "boolean"
  )
    return o as unknown as Reply;
  if (
    o.type !== "start" ||
    typeof o.system !== "string" ||
    typeof o.prompt !== "string" ||
    !o.model ||
    typeof o.model !== "object" ||
    (o.model as Record<string, unknown>).provider !== "openai-codex" ||
    !supportedModels.has(String((o.model as Record<string, unknown>).model)) ||
    (o.model as Record<string, unknown>).reasoning !== "medium" ||
    !Array.isArray(o.tools) ||
    !Number.isInteger(o.max_rounds) ||
    Number(o.max_rounds) < 1 ||
    Number(o.max_rounds) > 16
  )
    throw new Error("invalid_start");
  const names = new Set<string>();
  for (const t of o.tools) {
    if (
      !t ||
      typeof t.name !== "string" ||
      !/^[a-z][a-z0-9_]{0,63}$/.test(t.name) ||
      typeof t.description !== "string" ||
      t.parameters?.type !== "object" ||
      names.has(t.name)
    )
      throw new Error("invalid_tool");
    names.add(t.name);
  }
  for (const k of ["checkpoint", "session_dir"])
    if (o[k] !== undefined && typeof o[k] !== "string")
      throw new Error("invalid_path");
  return o as unknown as Start;
}

const supportedModels = new Set([
  "gpt-5.3-codex-spark",
  "gpt-5.4",
  "gpt-5.4-mini",
  "gpt-5.5",
  "gpt-5.6-luna",
  "gpt-5.6-sol",
  "gpt-5.6-terra",
]);
// JSON escaping prevents data from closing the boundary. This is defense in depth, not authorization.
export function fence(value: unknown): string {
  return (
    "<untrusted_tool_data>\n" +
    JSON.stringify(value)
      .replaceAll("<", "\\u003c")
      .replaceAll(">", "\\u003e") +
    "\n</untrusted_tool_data>"
  );
}
