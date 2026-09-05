package runtime

import (
	"strings"
	"testing"
)

func TestModelProcessEnvDropsTikTokCredentials(t *testing.T) {
	t.Setenv("AD_AGENT_TIKTOK_APP_SECRET", "not-for-model")
	t.Setenv("TIKTOK_ACCESS_TOKEN", "not-for-model")
	t.Setenv("AD_AGENT_ENV_SENTINEL", "kept")
	joined := strings.Join(modelProcessEnv(), "\n")
	if strings.Contains(joined, "not-for-model") {
		t.Fatal("TikTok credential inherited by model process")
	}
	if !strings.Contains(joined, "AD_AGENT_ENV_SENTINEL=kept") {
		t.Fatal("unrelated environment removed")
	}
}
