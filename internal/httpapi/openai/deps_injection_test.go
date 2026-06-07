package openai

import (
	"ds2api/internal/config"
	"strings"
	"testing"

	"ds2api/internal/promptcompat"
)

type mockOpenAIConfig struct {
	aliases                     map[string]string
	autoDeleteMode              string
	toolMode                    string
	earlyEmit                   string
	responsesTTL                int
	embedProv                   string
	currentInputEnabled         bool
	currentInputMin             int
	thinkingInjection           *bool
	thinkingPrompt              string
	responseReplacementsEnabled bool
	responseReplacementRules    []config.ResponseReplacementRule
}

func (m mockOpenAIConfig) ModelAliases() map[string]string     { return m.aliases }
func (m mockOpenAIConfig) ToolcallMode() string                { return m.toolMode }
func (m mockOpenAIConfig) ToolcallEarlyEmitConfidence() string { return m.earlyEmit }
func (m mockOpenAIConfig) ResponsesStoreTTLSeconds() int       { return m.responsesTTL }
func (m mockOpenAIConfig) EmbeddingsProvider() string          { return m.embedProv }
func (m mockOpenAIConfig) AutoDeleteMode() string {
	if m.autoDeleteMode == "" {
		return "none"
	}
	return m.autoDeleteMode
}
func (m mockOpenAIConfig) AutoDeleteSessions() bool      { return false }
func (m mockOpenAIConfig) CurrentInputFileEnabled() bool { return m.currentInputEnabled }
func (m mockOpenAIConfig) CurrentInputFileMinChars() int {
	return m.currentInputMin
}
func (m mockOpenAIConfig) ThinkingInjectionEnabled() bool {
	if m.thinkingInjection == nil {
		return false
	}
	return *m.thinkingInjection
}
func (m mockOpenAIConfig) ThinkingInjectionPrompt() string          { return m.thinkingPrompt }
func (m mockOpenAIConfig) OutputIntegrityGuardEnabled() bool        { return true }
func (m mockOpenAIConfig) OutputIntegrityGuardText() string         { return "" }
func (m mockOpenAIConfig) SentinelsEnabled() bool                   { return true }
func (m mockOpenAIConfig) SentinelOverrides() config.SentinelConfig { return config.SentinelConfig{} }
func (m mockOpenAIConfig) ToolCallInstructionsEnabled() bool        { return true }
func (m mockOpenAIConfig) ToolCallInstructionsText() string         { return "" }
func (m mockOpenAIConfig) ReadToolCacheGuardEnabled() bool          { return true }
func (m mockOpenAIConfig) ReadToolCacheGuardText() string           { return "" }
func (m mockOpenAIConfig) EmptyOutputRetrySuffixEnabled() bool      { return true }
func (m mockOpenAIConfig) EmptyOutputRetrySuffixText() string       { return "" }

func (m mockOpenAIConfig) ResponseReplacementsEnabled() bool { return m.responseReplacementsEnabled }
func (m mockOpenAIConfig) ResponseReplacementRules() []config.ResponseReplacementRule {
	return m.responseReplacementRules
}

func TestNormalizeOpenAIChatRequestWithConfigInterface(t *testing.T) {
	cfg := mockOpenAIConfig{
		aliases: map[string]string{
			"my-model": "deepseek-v4-flash-search",
		},
	}
	req := map[string]any{
		"model":    "my-model",
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
	}
	out, err := promptcompat.NormalizeOpenAIChatRequest(cfg, req, "")
	if err != nil {
		t.Fatalf("promptcompat.NormalizeOpenAIChatRequest error: %v", err)
	}
	if out.ResolvedModel != "deepseek-v4-flash-search" {
		t.Fatalf("resolved model mismatch: got=%q", out.ResolvedModel)
	}
	if !out.Search || !out.Thinking {
		t.Fatalf("unexpected model flags: thinking=%v search=%v", out.Thinking, out.Search)
	}
}

func TestNormalizeOpenAIChatRequestDisablesThinkingForNoThinkingModel(t *testing.T) {
	cfg := mockOpenAIConfig{}
	req := map[string]any{
		"model":            "deepseek-v4-pro-nothinking",
		"messages":         []any{map[string]any{"role": "user", "content": "hello"}},
		"reasoning_effort": "high",
	}
	out, err := promptcompat.NormalizeOpenAIChatRequest(cfg, req, "")
	if err != nil {
		t.Fatalf("promptcompat.NormalizeOpenAIChatRequest error: %v", err)
	}
	if out.ResolvedModel != "deepseek-v4-pro-nothinking" {
		t.Fatalf("resolved model mismatch: got=%q", out.ResolvedModel)
	}
	if out.Thinking {
		t.Fatalf("expected nothinking model to force thinking off")
	}
	if out.Search {
		t.Fatalf("expected search=false for deepseek-v4-pro-nothinking, got=%v", out.Search)
	}
}

func TestNormalizeOpenAIResponsesRequestAlwaysAcceptsWideInput(t *testing.T) {
	req := map[string]any{
		"model": "deepseek-v4-flash",
		"input": "hi",
	}

	out, err := promptcompat.NormalizeOpenAIResponsesRequest(mockOpenAIConfig{
		aliases: map[string]string{},
	}, req, "")
	if err != nil {
		t.Fatalf("unexpected error for wide input request: %v", err)
	}
	if out.Surface != "openai_responses" {
		t.Fatalf("unexpected surface: %q", out.Surface)
	}
	if !strings.Contains(out.FinalPrompt, "<|User|>hi") {
		t.Fatalf("unexpected final prompt: %q", out.FinalPrompt)
	}
}
