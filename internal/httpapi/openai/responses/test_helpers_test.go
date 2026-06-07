package responses

import (
	"encoding/json"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/config"
	"ds2api/internal/httpapi/openai/shared"
)

func asString(v any) string {
	return shared.AsString(v)
}

func decodeJSONBody(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("decode json failed: %v, body=%s", err, body)
	}
	return out
}

type mockResponsesConfig struct {
	responseReplacementsEnabled bool
	responseReplacementRules    []config.ResponseReplacementRule
}

func (m mockResponsesConfig) ModelAliases() map[string]string     { return nil }
func (m mockResponsesConfig) ToolcallMode() string                { return "" }
func (m mockResponsesConfig) ToolcallEarlyEmitConfidence() string { return "" }
func (m mockResponsesConfig) ResponsesStoreTTLSeconds() int       { return 0 }
func (m mockResponsesConfig) EmbeddingsProvider() string          { return "" }
func (m mockResponsesConfig) AutoDeleteMode() string              { return "none" }
func (m mockResponsesConfig) AutoDeleteSessions() bool            { return false }
func (m mockResponsesConfig) CurrentInputFileEnabled() bool       { return false }
func (m mockResponsesConfig) CurrentInputFileMinChars() int       { return 0 }
func (m mockResponsesConfig) ThinkingInjectionEnabled() bool      { return false }
func (m mockResponsesConfig) ThinkingInjectionPrompt() string     { return "" }
func (m mockResponsesConfig) OutputIntegrityGuardEnabled() bool   { return true }
func (m mockResponsesConfig) SentinelsEnabled() bool              { return true }
func (m mockResponsesConfig) SentinelOverrides() config.SentinelConfig {
	return config.SentinelConfig{}
}
func (m mockResponsesConfig) ToolCallInstructionsEnabled() bool { return true }
func (m mockResponsesConfig) ToolCallInstructionsText() string  { return "" }
func (m mockResponsesConfig) ReadToolCacheGuardEnabled() bool   { return true }
func (m mockResponsesConfig) ReadToolCacheGuardText() string    { return "" }
func (m mockResponsesConfig) EmptyOutputRetrySuffixEnabled() bool {
	return true
}
func (m mockResponsesConfig) EmptyOutputRetrySuffixText() string { return "" }
func (m mockResponsesConfig) ResponseReplacementsEnabled() bool {
	return m.responseReplacementsEnabled
}
func (m mockResponsesConfig) ResponseReplacementRules() []config.ResponseReplacementRule {
	return m.responseReplacementRules
}

func RegisterRoutes(r chi.Router, h *Handler) {
	r.Post("/v1/responses", h.Responses)
	r.Get("/v1/responses/{response_id}", h.GetResponseByID)
}
