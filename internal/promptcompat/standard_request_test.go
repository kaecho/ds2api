package promptcompat

import (
	"testing"

	"ds2api/internal/config"
)

func TestCompletionPayloadAppliesRequestReplacementsToPromptOnly(t *testing.T) {
	req := StandardRequest{
		RequestedModel: "deepseek-chat",
		FinalPrompt:    `<|DSML|tool_calls><|DSML|invoke name="search"></|DSML|invoke></|DSML|tool_calls>`,
		PassThrough: map[string]any{
			"metadata": `<|DSML|should_not_change>`,
		},
	}

	payload := req.CompletionPayloadWithRequestReplacements("session-1", []config.ResponseReplacementRule{
		{From: "<|DSML", To: "<|DEML"},
		{From: "</|DSML", To: "</|DEML"},
	})

	gotPrompt, _ := payload["prompt"].(string)
	wantPrompt := `<|DEML|tool_calls><|DEML|invoke name="search"></|DEML|invoke></|DEML|tool_calls>`
	if gotPrompt != wantPrompt {
		t.Fatalf("prompt=%q want %q", gotPrompt, wantPrompt)
	}

	if req.FinalPrompt != `<|DSML|tool_calls><|DSML|invoke name="search"></|DSML|invoke></|DSML|tool_calls>` {
		t.Fatalf("FinalPrompt was mutated: %q", req.FinalPrompt)
	}

	if payload["metadata"] != `<|DSML|should_not_change>` {
		t.Fatalf("metadata was unexpectedly replaced: %#v", payload["metadata"])
	}
}

func TestCompletionPayloadKeepsPromptUnchangedByDefault(t *testing.T) {
	req := StandardRequest{
		RequestedModel: "deepseek-chat",
		FinalPrompt:    `<|DSML|tool_calls>`,
	}

	payload := req.CompletionPayload("session-1")

	if got := payload["prompt"]; got != `<|DSML|tool_calls>` {
		t.Fatalf("prompt=%q want unchanged DSML prompt", got)
	}
}

func TestStandardRequestCompletionPayloadSetsModelTypeFromResolvedModel(t *testing.T) {
	tests := []struct {
		name      string
		model     string
		thinking  bool
		search    bool
		modelType string
	}{
		{name: "default", model: "deepseek-v4-flash", thinking: false, search: false, modelType: "default"},
		{name: "default_nothinking", model: "deepseek-v4-flash-nothinking", thinking: false, search: false, modelType: "default"},
		{name: "expert", model: "deepseek-v4-pro", thinking: true, search: false, modelType: "expert"},
		{name: "vision", model: "deepseek-v4-vision", thinking: true, search: false, modelType: "vision"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := StandardRequest{
				ResolvedModel: tc.model,
				FinalPrompt:   "hello",
				Thinking:      tc.thinking,
				Search:        tc.search,
				RefFileIDs:    []string{"file-a", "file-b"},
				PassThrough: map[string]any{
					"temperature": 0.3,
				},
			}

			payload := req.CompletionPayload("session-123")

			if got := payload["model_type"]; got != tc.modelType {
				t.Fatalf("expected model_type %s, got %#v", tc.modelType, got)
			}
			if got := payload["chat_session_id"]; got != "session-123" {
				t.Fatalf("unexpected chat_session_id: %#v", got)
			}
			if got := payload["thinking_enabled"]; got != tc.thinking {
				t.Fatalf("unexpected thinking_enabled: %#v", got)
			}
			if got := payload["search_enabled"]; got != tc.search {
				t.Fatalf("unexpected search_enabled: %#v", got)
			}
			if got := payload["temperature"]; got != 0.3 {
				t.Fatalf("expected passthrough temperature, got %#v", got)
			}
			refFileIDs, ok := payload["ref_file_ids"].([]any)
			if !ok {
				t.Fatalf("expected ref_file_ids slice, got %#v", payload["ref_file_ids"])
			}
			if len(refFileIDs) != 2 || refFileIDs[0] != "file-a" || refFileIDs[1] != "file-b" {
				t.Fatalf("unexpected ref_file_ids: %#v", refFileIDs)
			}
		})
	}
}
