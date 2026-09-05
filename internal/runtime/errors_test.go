package runtime

import (
	"context"
	"errors"
	"testing"
)

func TestFailureCodesExcludeProviderDetails(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nil, ""}, {context.Canceled, "runtime_cancelled"}, {context.DeadlineExceeded, "runtime_timeout"},
		{errors.New("Builtin model bridge failed: provider_history_rejected"), "provider_history_rejected"},
		{errors.New("runtime_checkpoint_invalid"), "runtime_checkpoint_invalid"},
		{errors.New("secret_token and private provider response"), "runtime_failed"},
	} {
		if got := FailureCode(tc.err); got != tc.want {
			t.Fatalf("code=%q want=%q", got, tc.want)
		}
	}
}
