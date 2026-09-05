package runtime

import (
	"context"
	"errors"
	"strings"
)

// These codes, not native/provider error strings, are the diagnostic boundary.
func SafeFailureCode(code string) string {
	switch code {
	case "provider_duplicate_item", "provider_encrypted_state_rejected", "provider_history_rejected",
		"provider_rate_limited", "provider_auth_failed", "provider_context_limit", "provider_timeout",
		"provider_transport_failed", "provider_request_rejected", "provider_failed", "provider_incomplete",
		"assistant_state_mismatch", "assistant_state_count_mismatch", "model_text_limit_exceeded",
		"chatgpt_oauth_required", "oauth_account_changed", "oauth_or_model_missing", "api_key_missing",
		"native_start_failed", "native_request_timeout", "native_turn_failed", "native_context_isolation_failed",
		"native_tool_boundary_violation", "native_protocol_failed", "unexpected_model_reroute",
		"runtime_timeout", "runtime_cancelled", "runtime_checkpoint_invalid", "event_persistence_failed":
		return code
	default:
		return "runtime_failed"
	}
}

func FailureCode(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "runtime_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "runtime_cancelled"
	}
	// Adapters wrap fixed codes with a local label. No arbitrary substring is copied.
	for _, word := range strings.FieldsFunc(err.Error(), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r == '_')
	}) {
		if code := SafeFailureCode(word); code != "runtime_failed" {
			return code
		}
	}
	return "runtime_failed"
}
