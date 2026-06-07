package server

import (
	"testing"

	"ds2api/internal/config"
	"ds2api/internal/prompt"
)

func TestApplyPromptConfigFromStoreResetsSentinelDefaults(t *testing.T) {
	t.Cleanup(prompt.ResetSentinelDefaults)

	prompt.SentinelSystem = "custom-system-marker"
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["k1"],"prompt":{"sentinels":{"enabled":true}}}`)
	t.Setenv("DS2API_ENV_WRITEBACK", "0")

	store := config.LoadStore()
	applyPromptConfigFromStore(store)

	if got := prompt.SentinelSystem; got != prompt.DefaultSentinelSystem {
		t.Fatalf("expected sentinel system marker to reset to default, got %q", got)
	}
}
