import type {
  AssistantMessage,
  Context,
  Message,
  Tool,
} from "@earendil-works/pi-ai";

export type JContent =
  | { type: "text" | "reasoning"; text?: string }
  | {
      type: "tool_call";
      toolCall: { id: string; name: string; arguments: unknown };
    };
export interface JMessage {
  role: "system" | "user" | "assistant" | "tool";
  content: JContent[];
  toolCallId?: string;
  toolName?: string;
  isError?: boolean;
}
export interface JTool {
  name: string;
  description: string;
  inputSchema: Record<string, unknown>;
}
export interface JRequest {
  messages: JMessage[];
  tools?: JTool[];
}
export interface ProviderState {
  version: 1;
  assistants: AssistantMessage[];
}
export type InputFrame =
  | { type: "start"; provider_state?: unknown }
  | { type: "complete"; id: string; request: JRequest };

export function parseInput(line: string): InputFrame {
  if (Buffer.byteLength(line) > 8 * 1024 * 1024) throw new Error("frame_limit");
  const value: unknown = JSON.parse(line);
  if (!value || typeof value !== "object") throw new Error("invalid_frame");
  const frame = value as Record<string, unknown>;
  if (frame.type === "start") return frame as InputFrame;
  if (
    frame.type !== "complete" ||
    typeof frame.id !== "string" ||
    !frame.id ||
    !frame.request ||
    typeof frame.request !== "object" ||
    !Array.isArray((frame.request as Record<string, unknown>).messages)
  )
    throw new Error("invalid_complete");
  return frame as unknown as InputFrame;
}

export function parseState(value: unknown): ProviderState {
  if (value === undefined || value === null)
    return { version: 1, assistants: [] };
  if (!value || typeof value !== "object") throw new Error("invalid_state");
  const state = value as Record<string, unknown>;
  if (state.version !== 1 || !Array.isArray(state.assistants))
    throw new Error("invalid_state");
  const encoded = JSON.stringify(state);
  if (Buffer.byteLength(encoded) > 8 * 1024 * 1024)
    throw new Error("state_limit");
  for (const message of state.assistants) validateNativeAssistant(message);
  return state as unknown as ProviderState;
}

export function buildContext(request: JRequest, state: ProviderState): Context {
  if (!Array.isArray(request.messages) || !Array.isArray(request.tools ?? []))
    throw new Error("invalid_request");
  let systemPrompt: string | undefined;
  let assistantIndex = 0;
  const messages: Message[] = [];
  for (let index = 0; index < request.messages.length; index++) {
    const message = request.messages[index]!;
    validateJMessage(message);
    if (message.role === "system") {
      if (index !== 0 || systemPrompt !== undefined)
        throw new Error("invalid_system");
      systemPrompt = joinText(message);
      continue;
    }
    if (message.role === "user") {
      messages.push({ role: "user", content: joinText(message), timestamp: 0 });
      continue;
    }
    if (message.role === "tool") {
      if (!message.toolCallId || !message.toolName)
        throw new Error("invalid_tool_result");
      messages.push({
        role: "toolResult",
        toolCallId: message.toolCallId,
        toolName: message.toolName,
        content: [{ type: "text", text: joinText(message) }],
        isError: Boolean(message.isError),
        timestamp: 0,
      });
      continue;
    }
    const native = state.assistants[assistantIndex++];
    if (
      !native ||
      canonical(normalizeNative(native)) !==
        canonical(normalizeJ(message.content))
    )
      throw new Error("assistant_state_mismatch");
    messages.push(native);
  }
  if (assistantIndex !== state.assistants.length)
    throw new Error("assistant_state_count_mismatch");
  const tools: Tool[] = (request.tools ?? []).map((tool) => {
    if (
      !tool ||
      typeof tool.name !== "string" ||
      typeof tool.description !== "string" ||
      !tool.inputSchema ||
      typeof tool.inputSchema !== "object" ||
      Array.isArray(tool.inputSchema)
    )
      throw new Error("invalid_tool");
    return {
      name: tool.name,
      description: tool.description,
      parameters: tool.inputSchema as Tool["parameters"],
    };
  });
  return { systemPrompt, messages, tools };
}

export function toJMessage(message: AssistantMessage): JMessage {
  validateNativeAssistant(message);
  return {
    role: "assistant",
    content: normalizeNative(message),
  };
}

function validateNativeAssistant(
  value: unknown,
): asserts value is AssistantMessage {
  if (!value || typeof value !== "object") throw new Error("invalid_assistant");
  const message = value as Record<string, unknown>;
  if (
    message.role !== "assistant" ||
    message.provider !== "openai-codex" ||
    message.model !== "gpt-5.6-luna" ||
    !Array.isArray(message.content) ||
    !message.usage ||
    typeof message.stopReason !== "string"
  )
    throw new Error("invalid_assistant");
  for (const content of message.content) {
    if (!content || typeof content !== "object")
      throw new Error("invalid_assistant");
    const block = content as Record<string, unknown>;
    if (block.type === "text" && typeof block.text === "string") continue;
    if (block.type === "thinking" && typeof block.thinking === "string")
      continue;
    if (
      block.type === "toolCall" &&
      typeof block.id === "string" &&
      typeof block.name === "string" &&
      block.arguments !== null &&
      typeof block.arguments === "object" &&
      !Array.isArray(block.arguments)
    )
      continue;
    throw new Error("invalid_assistant");
  }
}

function validateJMessage(message: JMessage): void {
  if (
    !message ||
    !["system", "user", "assistant", "tool"].includes(message.role) ||
    !Array.isArray(message.content)
  )
    throw new Error("invalid_message");
  for (const content of message.content) {
    if (!content || typeof content !== "object")
      throw new Error("invalid_content");
    if (
      (content.type === "text" || content.type === "reasoning") &&
      (content.text === undefined || typeof content.text === "string")
    )
      continue;
    if (
      content.type === "tool_call" &&
      content.toolCall &&
      typeof content.toolCall.id === "string" &&
      typeof content.toolCall.name === "string"
    )
      continue;
    throw new Error("invalid_content");
  }
}

function joinText(message: JMessage): string {
  if (message.content.some((part) => part.type !== "text"))
    throw new Error("unexpected_non_text_content");
  return message.content
    .map((part) => ("text" in part ? (part.text ?? "") : ""))
    .join("");
}

function normalizeJ(content: JContent[]): JContent[] {
  return content.map((part) =>
    part.type === "text" || part.type === "reasoning"
      ? { type: part.type, text: part.text ?? "" }
      : part,
  );
}

function normalizeNative(message: AssistantMessage): JContent[] {
  return message.content.map((content) => {
    if (content.type === "text") return { type: "text", text: content.text };
    if (content.type === "thinking")
      return { type: "reasoning", text: content.thinking };
    return {
      type: "tool_call",
      toolCall: {
        id: content.id,
        name: content.name,
        arguments: content.arguments,
      },
    };
  });
}

function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value && typeof value === "object") {
    const object = value as Record<string, unknown>;
    return `{${Object.keys(object)
      .sort()
      .map((key) => `${JSON.stringify(key)}:${canonical(object[key])}`)
      .join(",")}}`;
  }
  return JSON.stringify(value);
}
