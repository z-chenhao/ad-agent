// Only fixed categories may cross the bridge. Never forward provider messages,
// which can contain request fragments, URLs, credentials or private reasoning.
export function providerFailure(message = ""): string {
  const text = message.toLowerCase();
  if (/duplicate.*item|item.*duplicate/.test(text))
    return "provider_duplicate_item";
  if (/encrypted|decrypt/.test(text))
    return "provider_encrypted_state_rejected";
  if (
    /reasoning.*(following|required|missing)|tool.*(output|result).*(missing|found)|no tool output|without.*required/.test(
      text,
    )
  )
    return "provider_history_rejected";
  if (/usage limit|quota|rate.limit|too many requests|429/.test(text))
    return "provider_rate_limited";
  if (/unauthorized|authentication|invalid.*(token|key)|401|403/.test(text))
    return "provider_auth_failed";
  if (/context.*(length|limit)|too many tokens/.test(text))
    return "provider_context_limit";
  if (/timeout|timed out/.test(text)) return "provider_timeout";
  if (/fetch failed|connection|socket|network/.test(text))
    return "provider_transport_failed";
  if (/invalid|bad request|400/.test(text)) return "provider_request_rejected";
  return "provider_failed";
}

export const providerFailureCodes = new Set([
  "provider_duplicate_item",
  "provider_encrypted_state_rejected",
  "provider_history_rejected",
  "provider_rate_limited",
  "provider_auth_failed",
  "provider_context_limit",
  "provider_timeout",
  "provider_transport_failed",
  "provider_request_rejected",
  "provider_failed",
]);
