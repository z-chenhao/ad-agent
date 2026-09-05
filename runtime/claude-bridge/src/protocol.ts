export interface ToolSpec {
  name: string;
  description: string;
  parameters: Record<string, unknown>;
}

export function publicTextSuffix(previous: string, completed: string): string {
  if (!completed.startsWith(previous)) throw new Error("model_text_mismatch");
  return completed.slice(previous.length);
}

export interface Start {
  type: "start";
  system: string;
  prompt: string;
  model: {
    provider: string;
    model: string;
    reasoning: "medium";
    auth_mode: "api_key";
    api: "anthropic-messages";
    base_url: string;
    api_key_env: string;
    context_window: number;
    max_output_tokens: number;
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
  const value: unknown = JSON.parse(line);
  if (!value || typeof value !== "object") throw new Error("invalid_frame");
  const frame = value as Record<string, unknown>;
  if (
    frame.type === "tool_result" &&
    typeof frame.id === "string" &&
    frame.result &&
    typeof frame.result === "object" &&
    typeof (frame.result as Record<string, unknown>).ok === "boolean"
  )
    return frame as unknown as Reply;
  const model = frame.model as Record<string, unknown> | undefined;
  if (
    frame.type !== "start" ||
    typeof frame.system !== "string" ||
    typeof frame.prompt !== "string" ||
    !model ||
    model.provider !== "anthropic" ||
    model.auth_mode !== "api_key" ||
    model.api !== "anthropic-messages" ||
    typeof model.model !== "string" ||
    !/^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$/.test(model.model) ||
    model.reasoning !== "medium" ||
    typeof model.base_url !== "string" ||
    typeof model.api_key_env !== "string" ||
    !/^[A-Z][A-Z0-9_]{0,127}$/.test(model.api_key_env) ||
    !Array.isArray(frame.tools) ||
    !Number.isInteger(frame.max_rounds) ||
    Number(frame.max_rounds) < 0 ||
    Number(frame.max_rounds) > 16
  )
    throw new Error("invalid_start");
  const names = new Set<string>();
  for (const tool of frame.tools) {
    if (
      !tool ||
      typeof tool.name !== "string" ||
      !/^[a-z][a-z0-9_]{0,63}$/.test(tool.name) ||
      typeof tool.description !== "string" ||
      !tool.parameters ||
      typeof tool.parameters !== "object" ||
      names.has(tool.name)
    )
      throw new Error("invalid_tool");
    names.add(tool.name);
  }
  return frame as unknown as Start;
}

export function fence(value: unknown): string {
  return (
    "<untrusted_tool_data>\n" +
    JSON.stringify(value)
      .replaceAll("<", "\\u003c")
      .replaceAll(">", "\\u003e") +
    "\n</untrusted_tool_data>"
  );
}
