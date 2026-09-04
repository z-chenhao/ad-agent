export interface ToolSpec {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
}
export interface Start {
  type: "start";
  system: string;
  prompt: string;
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
