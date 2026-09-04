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
    provider: string;
    model: string;
    reasoning: "medium";
    auth_mode: "chatgpt_oauth" | "api_key";
    api?: "anthropic-messages" | "openai-responses" | "openai-completions";
    base_url?: string;
    api_key_env?: string;
    context_window?: number;
    max_output_tokens?: number;
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
    !validModel(o.model as Record<string, unknown>) ||
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

function validModel(model: Record<string, unknown>): boolean {
  if (model.reasoning !== "medium") return false;
  if (model.auth_mode === "chatgpt_oauth")
    return model.provider === "openai-codex" && supportedModels.has(String(model.model));
  return (
    model.auth_mode === "api_key" &&
    typeof model.provider === "string" && /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$/.test(model.provider) &&
    typeof model.model === "string" && /^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$/.test(model.model) &&
    ["anthropic-messages", "openai-responses", "openai-completions"].includes(String(model.api)) &&
    typeof model.base_url === "string" &&
    typeof model.api_key_env === "string" && /^[A-Z][A-Z0-9_]{0,127}$/.test(model.api_key_env) &&
    Number.isInteger(model.context_window) && Number(model.context_window) >= 4096 &&
    Number.isInteger(model.max_output_tokens) && Number(model.max_output_tokens) >= 256
  );
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
