import { createHash } from "node:crypto";

export interface Start {
  type: "start";
  system: string;
  prompt: string;
  tools: {
    name: string;
    description: string;
    parameters: Record<string, unknown>;
  }[];
  model: {
    provider: string;
    model: string;
    reasoning: "medium";
    auth_mode: "chatgpt_oauth" | "api_key";
    api?: string;
    base_url?: string;
    api_key_env?: string;
    context_window?: number;
    max_output_tokens?: number;
  };
  max_rounds: number;
  session_dir?: string;
  checkpoint?: string;
}
export interface Reply {
  type: "tool_result";
  id: string;
  result: { ok: boolean; data?: unknown; error?: string; close?: boolean };
}

export function parseInput(line: string): Start | Reply {
  if (Buffer.byteLength(line) > 1_048_576) throw new Error("frame_limit");
  const value = JSON.parse(line) as Start | Reply;
  if (
    value.type === "tool_result" &&
    typeof value.id === "string" &&
    typeof value.result?.ok === "boolean"
  )
    return value;
  if (
    value.type !== "start" ||
    typeof value.system !== "string" ||
    typeof value.prompt !== "string" ||
    !Array.isArray(value.tools) ||
    !Number.isInteger(value.max_rounds) ||
    value.max_rounds < 0 ||
    value.max_rounds > 16 ||
    value.model?.reasoning !== "medium"
  )
    throw new Error("invalid_start");
  if (value.model.auth_mode === "chatgpt_oauth") {
    if (value.model.provider !== "openai-codex")
      throw new Error("invalid_oauth_provider");
  } else if (
    value.model.auth_mode !== "api_key" ||
    value.model.api !== "openai-responses" ||
    !value.model.base_url ||
    !/^[A-Z][A-Z0-9_]*$/.test(value.model.api_key_env ?? "")
  )
    throw new Error("codex_requires_responses_protocol");
  const names = new Set<string>();
  for (const tool of value.tools) {
    if (
      !/^[a-z][a-z0-9_]{0,63}$/.test(tool.name) ||
      names.has(tool.name) ||
      tool.parameters?.type !== "object" ||
      typeof tool.description !== "string"
    )
      throw new Error("invalid_tool");
    names.add(tool.name);
  }
  return value;
}

export const digest = (value: unknown) =>
  createHash("sha256").update(JSON.stringify(value)).digest("hex");
export const fence = (value: unknown) =>
  `<untrusted_tool_data>\n${JSON.stringify(value).replaceAll("<", "\\u003c").replaceAll(">", "\\u003e")}\n</untrusted_tool_data>`;

// Private, version-pinned policy. No workspace environment means no shell,
// patch or image/file access; features additionally remove external capabilities.
export function isolatedConfig(request: Start): Record<string, unknown> {
  const config: Record<string, unknown> = {
    "features.shell_tool": false,
    "features.unified_exec": false,
    "features.multi_agent": false,
    "features.multi_agent_v2": false,
    "features.apps": false,
    "features.plugins": false,
    "features.hooks": false,
    "features.memories": false,
    "features.image_generation": false,
    "features.code_mode": false,
    "features.js_repl": false,
    "features.request_permissions_tool": false,
    "features.goals": false,
    "features.browser_use": false,
    "features.computer_use": false,
    "features.in_app_browser": false,
    "features.workspace_dependencies": false,
    "features.tool_suggest": false,
    web_search: "disabled",
    "tools.experimental_request_user_input": { enabled: false },
    project_doc_max_bytes: 0,
    "skills.include_instructions": false,
    "skills.bundled.enabled": false,
    mcp_servers: {},
    model_reasoning_effort: request.model.reasoning,
    model_reasoning_summary: "none",
    model_provider:
      request.model.auth_mode === "api_key" ? "ad_direct" : "openai",
  };
  if (request.model.auth_mode === "api_key") {
    config["model_providers.ad_direct"] = {
      name: request.model.provider,
      base_url: request.model.base_url,
      env_key: request.model.api_key_env,
      wire_api: "responses",
      supports_websockets: false,
      requires_openai_auth: false,
      request_max_retries: 0,
      stream_max_retries: 0,
    };
    config["model_context_window"] = request.model.context_window;
  }
  return config;
}
