# Tool Interception Policy Unification Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Gemini native streaming obey the same tool-interception policy chain as OpenAI and Claude, and centralize the policy decision so future adapters do not reimplement it incorrectly.

**Architecture:** Add a small shared `internal/toolpolicy` helper that answers whether a given `promptcompat.ToolChoicePolicy` should buffer tool markup or allow tool parsing. Keep each adapter's stream protocol emitter separate, but route OpenAI, Claude, Gemini native, and `assistantturn` through the same helper and the same `ToolChoice` plumbing.

**Tech Stack:** Go backend (`internal/toolpolicy`, `internal/promptcompat`, `internal/assistantturn`, `internal/httpapi/openai`, `internal/httpapi/claude`, `internal/httpapi/gemini`), Go tests.

---

## File Structure

- Create `internal/toolpolicy/policy.go` — tiny shared helper for stream-time buffering and finalize-time tool parsing decisions.
- Create `internal/toolpolicy/policy_test.go` — mode matrix tests for `auto`, `none`, `required`, and `forced`.
- Modify `internal/assistantturn/turn.go` — replace direct `ToolChoice.IsNone()` branches with the shared helper.
- Modify `internal/httpapi/openai/chat/handler_chat.go` — derive `bufferToolContent` from the shared helper.
- Modify `internal/httpapi/openai/chat/empty_retry_runtime.go` — derive retry runtime buffering from the shared helper.
- Modify `internal/httpapi/openai/chat/chat_stream_runtime.go` — derive `PreserveToolMarkup` from the shared helper.
- Modify `internal/httpapi/openai/responses/responses_handler.go` — derive `bufferToolContent` from the shared helper.
- Modify `internal/httpapi/openai/responses/empty_retry_runtime.go` — derive retry runtime buffering from the shared helper.
- Modify `internal/httpapi/openai/responses/responses_stream_runtime_core.go` — derive `PreserveToolMarkup` from the shared helper.
- Modify `internal/httpapi/claude/stream_runtime_core.go` — derive Claude buffering from the shared helper.
- Modify `internal/httpapi/gemini/convert_request.go` — set explicit default `ToolChoice` in Gemini native normalization.
- Modify `internal/httpapi/gemini/handler_generate.go` — pass `ToolChoice` into Gemini native stream setup.
- Modify `internal/httpapi/gemini/handler_stream_runtime.go` — store `ToolChoice`, compute `bufferContent` from the shared helper, and pass `ToolChoice` into `assistantturn.BuildTurnFromStreamSnapshot(...)`.
- Modify `internal/httpapi/gemini/convert_request_test.go` — assert Gemini native normalization sets explicit default tool policy.
- Modify `internal/httpapi/gemini/handler_test.go` — cover native Gemini stream behavior for default tool policy and explicit `tool_choice:none`.

---

### Task 1: Add the shared tool policy helper

**Files:**
- Create: `internal/toolpolicy/policy.go`
- Create: `internal/toolpolicy/policy_test.go`

- [ ] **Step 1: Write the failing helper tests**

Create `internal/toolpolicy/policy_test.go` with:

```go
package toolpolicy

import (
	"testing"

	"ds2api/internal/promptcompat"
)

func TestShouldBufferToolContent(t *testing.T) {
	cases := []struct {
		name   string
		policy promptcompat.ToolChoicePolicy
		want   bool
	}{
		{name: "default-auto", policy: promptcompat.DefaultToolChoicePolicy(), want: true},
		{name: "none", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceNone}, want: false},
		{name: "required", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceRequired}, want: true},
		{name: "forced", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceForced, ForcedName: "Bash"}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldBufferToolContent(tc.policy); got != tc.want {
				t.Fatalf("ShouldBufferToolContent(%q)=%v want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestShouldParseToolCalls(t *testing.T) {
	cases := []struct {
		name   string
		policy promptcompat.ToolChoicePolicy
		want   bool
	}{
		{name: "default-auto", policy: promptcompat.DefaultToolChoicePolicy(), want: true},
		{name: "none", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceNone}, want: false},
		{name: "required", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceRequired}, want: true},
		{name: "forced", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceForced, ForcedName: "Bash"}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldParseToolCalls(tc.policy); got != tc.want {
				t.Fatalf("ShouldParseToolCalls(%q)=%v want %v", tc.name, got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run the helper tests to verify RED**

Run:

```bash
go test ./internal/toolpolicy -run 'TestShould(BufferToolContent|ParseToolCalls)$' -count=1
```

Expected: FAIL with compile errors because `ShouldBufferToolContent` and `ShouldParseToolCalls` do not exist yet.

- [ ] **Step 3: Write the minimal helper implementation**

Create `internal/toolpolicy/policy.go` with:

```go
package toolpolicy

import "ds2api/internal/promptcompat"

func ShouldBufferToolContent(policy promptcompat.ToolChoicePolicy) bool {
	return !policy.IsNone()
}

func ShouldParseToolCalls(policy promptcompat.ToolChoicePolicy) bool {
	return !policy.IsNone()
}
```

- [ ] **Step 4: Run the helper tests to verify GREEN**

Run:

```bash
go test ./internal/toolpolicy -run 'TestShould(BufferToolContent|ParseToolCalls)$' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the helper package**

Run:

```bash
git add internal/toolpolicy/policy.go internal/toolpolicy/policy_test.go
git commit -m "refactor: add shared tool policy helper"
```

---

### Task 2: Switch existing OpenAI, Claude, and assistantturn callsites to the shared helper

**Files:**
- Modify: `internal/assistantturn/turn.go`
- Modify: `internal/httpapi/openai/chat/handler_chat.go`
- Modify: `internal/httpapi/openai/chat/empty_retry_runtime.go`
- Modify: `internal/httpapi/openai/chat/chat_stream_runtime.go`
- Modify: `internal/httpapi/openai/responses/responses_handler.go`
- Modify: `internal/httpapi/openai/responses/empty_retry_runtime.go`
- Modify: `internal/httpapi/openai/responses/responses_stream_runtime_core.go`
- Modify: `internal/httpapi/claude/stream_runtime_core.go`
- Test: `internal/assistantturn/turn_test.go`
- Test: `internal/httpapi/openai/chat/handler_toolcall_test.go`
- Test: `internal/httpapi/openai/responses/responses_stream_test.go`
- Test: `internal/httpapi/claude/handler_stream_test.go`

- [ ] **Step 1: Run the current focused regressions before the mechanical refactor**

Run:

```bash
go test ./internal/assistantturn ./internal/httpapi/openai/chat ./internal/httpapi/openai/responses ./internal/httpapi/claude -run 'Test(BuildTurnFromCollectedToolChoiceRequired|BuildTurnFromStreamSnapshotUsesVisibleTextAndRawToolDetection|HandleStreamDoesNotLeakToolTextWhenReplacementFlushCompletesToolCall|HandleStreamHonorsExplicitToolChoiceNone|PrepareResponsesStreamRuntimeBuffersToolSyntaxWithoutDeclaredTools|HandleResponsesStreamWithRetryDoesNotLeakToolTextWithoutDeclaredTools|HandleClaudeStreamRealtimeDetectsToolUseWithoutDeclaredTools|HandleClaudeStreamRealtimeHonorsExplicitToolChoiceNone)$' -count=1
```

Expected: PASS. This is the safety net for the no-behavior-change refactor.

- [ ] **Step 2: Replace inline `ToolChoice.IsNone()` checks with the shared helper**

Update `internal/assistantturn/turn.go` imports and gating logic to:

```go
import (
	"net/http"
	"strings"

	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/promptcompat"
	"ds2api/internal/sse"
	"ds2api/internal/toolcall"
	"ds2api/internal/toolpolicy"
	"ds2api/internal/util"
)
```

```go
preserveToolMarkup := !toolpolicy.ShouldParseToolCalls(opts.ToolChoice)
thinking := shared.CleanVisibleOutputWithPolicy(result.Thinking, opts.StripReferenceMarkers, preserveToolMarkup)
text := shared.CleanVisibleOutputWithPolicy(result.Text, opts.StripReferenceMarkers, preserveToolMarkup)
```

```go
parsed := toolcall.ToolCallParseResult{}
var calls []toolcall.ParsedToolCall
if toolpolicy.ShouldParseToolCalls(opts.ToolChoice) {
	parsed = shared.DetectAssistantToolCalls(result.Text, text, result.Thinking, result.ToolDetectionThinking, opts.ToolNames)
	calls = toolcall.NormalizeParsedToolCallsForSchemas(parsed.Calls, opts.ToolsRaw)
	parsed.Calls = calls
}
```

```go
preserveToolMarkup := !toolpolicy.ShouldParseToolCalls(opts.ToolChoice)
thinking := shared.CleanVisibleOutputWithPolicy(snapshot.VisibleThinking, opts.StripReferenceMarkers, preserveToolMarkup)
text := shared.CleanVisibleOutputWithPolicy(snapshot.VisibleText, opts.StripReferenceMarkers, preserveToolMarkup)
```

```go
parsed := toolcall.ToolCallParseResult{}
var calls []toolcall.ParsedToolCall
if toolpolicy.ShouldParseToolCalls(opts.ToolChoice) {
	parsed = shared.DetectAssistantToolCalls(snapshot.RawText, text, snapshot.RawThinking, snapshot.DetectionThinking, opts.ToolNames)
	calls = parsed.Calls
	if len(calls) == 0 && len(snapshot.AdditionalToolCalls) > 0 {
		calls = snapshot.AdditionalToolCalls
	}
	calls = toolcall.NormalizeParsedToolCallsForSchemas(calls, opts.ToolsRaw)
	parsed.Calls = calls
}
```

Update the buffering and markup-preservation callsites to:

```go
// internal/httpapi/openai/chat/handler_chat.go
bufferToolContent := toolpolicy.ShouldBufferToolContent(toolChoice)
```

```go
// internal/httpapi/openai/chat/empty_retry_runtime.go
streamRuntime := newChatStreamRuntime(
	w, rc, canFlush, completionID, time.Now().Unix(), model, finalPrompt,
	thinkingEnabled, searchEnabled, stripReferenceMarkersEnabled(), toolNames, toolsRaw,
	toolChoice,
	toolpolicy.ShouldBufferToolContent(toolChoice), h.toolcallFeatureMatchEnabled() && h.toolcallEarlyEmitHighConfidence(),
	responserewrite.NewStreamReplacer(h.responseReplacementRules()),
)
```

```go
// internal/httpapi/openai/chat/chat_stream_runtime.go
accumulator: shared.StreamAccumulator{
	ThinkingEnabled:       thinkingEnabled,
	SearchEnabled:         searchEnabled,
	StripReferenceMarkers: stripReferenceMarkers,
	PreserveToolMarkup:    !toolpolicy.ShouldParseToolCalls(toolChoice),
	ResponseReplacer:      responseReplacer,
},
```

```go
// internal/httpapi/openai/responses/responses_handler.go
bufferToolContent := toolpolicy.ShouldBufferToolContent(toolChoice)
```

```go
// internal/httpapi/openai/responses/empty_retry_runtime.go
streamRuntime := newResponsesStreamRuntime(
	w, rc, canFlush, responseID, model, finalPrompt, thinkingEnabled, searchEnabled,
	stripReferenceMarkersEnabled(), toolNames, toolsRaw, toolpolicy.ShouldBufferToolContent(toolChoice),
	h.toolcallFeatureMatchEnabled() && h.toolcallEarlyEmitHighConfidence(),
	toolChoice, traceID, func(obj map[string]any) {
		h.getResponseStore().put(owner, responseID, obj)
	}, historySession, nil,
)
```

```go
// internal/httpapi/openai/responses/responses_stream_runtime_core.go
accumulator: shared.StreamAccumulator{
	ThinkingEnabled:       thinkingEnabled,
	SearchEnabled:         searchEnabled,
	StripReferenceMarkers: stripReferenceMarkers,
	PreserveToolMarkup:    !toolpolicy.ShouldParseToolCalls(toolChoice),
	ResponseReplacer:      responseReplacer,
},
```

```go
// internal/httpapi/claude/stream_runtime_core.go
bufferToolContent:     toolpolicy.ShouldBufferToolContent(toolChoice),
```

Add the `internal/toolpolicy` import to each edited file.

- [ ] **Step 3: Format the edited Go files**

Run:

```bash
gofmt -w \
  internal/assistantturn/turn.go \
  internal/httpapi/openai/chat/handler_chat.go \
  internal/httpapi/openai/chat/empty_retry_runtime.go \
  internal/httpapi/openai/chat/chat_stream_runtime.go \
  internal/httpapi/openai/responses/responses_handler.go \
  internal/httpapi/openai/responses/empty_retry_runtime.go \
  internal/httpapi/openai/responses/responses_stream_runtime_core.go \
  internal/httpapi/claude/stream_runtime_core.go
```

- [ ] **Step 4: Re-run the focused regressions after the refactor**

Run:

```bash
go test ./internal/assistantturn ./internal/httpapi/openai/chat ./internal/httpapi/openai/responses ./internal/httpapi/claude -run 'Test(BuildTurnFromCollectedToolChoiceRequired|BuildTurnFromStreamSnapshotUsesVisibleTextAndRawToolDetection|HandleStreamDoesNotLeakToolTextWhenReplacementFlushCompletesToolCall|HandleStreamHonorsExplicitToolChoiceNone|PrepareResponsesStreamRuntimeBuffersToolSyntaxWithoutDeclaredTools|HandleResponsesStreamWithRetryDoesNotLeakToolTextWithoutDeclaredTools|HandleClaudeStreamRealtimeDetectsToolUseWithoutDeclaredTools|HandleClaudeStreamRealtimeHonorsExplicitToolChoiceNone)$' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit the callsite refactor**

Run:

```bash
git add \
  internal/assistantturn/turn.go \
  internal/httpapi/openai/chat/handler_chat.go \
  internal/httpapi/openai/chat/empty_retry_runtime.go \
  internal/httpapi/openai/chat/chat_stream_runtime.go \
  internal/httpapi/openai/responses/responses_handler.go \
  internal/httpapi/openai/responses/empty_retry_runtime.go \
  internal/httpapi/openai/responses/responses_stream_runtime_core.go \
  internal/httpapi/claude/stream_runtime_core.go
git commit -m "refactor: reuse tool policy helper in stream runtimes"
```

---

### Task 3: Apply the unified policy chain to native Gemini streaming

**Files:**
- Modify: `internal/httpapi/gemini/convert_request.go`
- Modify: `internal/httpapi/gemini/handler_generate.go`
- Modify: `internal/httpapi/gemini/handler_stream_runtime.go`
- Modify: `internal/httpapi/gemini/convert_request_test.go`
- Modify: `internal/httpapi/gemini/handler_test.go`

- [ ] **Step 1: Write the failing Gemini normalization and stream-policy regressions**

Extend `internal/httpapi/gemini/convert_request_test.go` with:

```go
package gemini

import (
	"testing"

	"ds2api/internal/promptcompat"
)

func TestNormalizeGeminiRequestSetsDefaultToolChoicePolicy(t *testing.T) {
	req := map[string]any{
		"contents": []any{
			map[string]any{
				"role":  "user",
				"parts": []any{map[string]any{"text": "hello"}},
			},
		},
	}
	out, err := normalizeGeminiRequest(testGeminiConfig{}, "gemini-2.5-pro", req, false)
	if err != nil {
		t.Fatalf("normalizeGeminiRequest error: %v", err)
	}
	if out.ToolChoice.Mode != promptcompat.ToolChoiceAuto {
		t.Fatalf("expected default ToolChoiceAuto, got %#v", out.ToolChoice)
	}
}
```

Extend `internal/httpapi/gemini/handler_test.go` with:

```go
func TestNativeStreamGenerateContentDetectsToolUseWithoutDeclaredTools(t *testing.T) {
	h := &Handler{}
	resp := makeGeminiUpstreamResponse(
		`data: {"p":"response/content","v":"<|DSML|tool_calls>\n<|DSML|invoke name=\"Bash\">\n<|DSML|parameter name=\"command\"><![CDATA[pwd]]></|DSML|parameter>\n</|DSML|invoke>\n</|DSML|tool_calls>"}`,
		`data: [DONE]`,
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:streamGenerateContent", nil)

	h.handleStreamGenerateContent(rec, req, resp, "gemini-2.5-pro", "prompt", false, false, nil, nil, promptcompat.DefaultToolChoicePolicy())

	frames := extractGeminiSSEFrames(t, rec.Body.String())
	var text strings.Builder
	foundFunctionCall := false
	for _, frame := range frames {
		for _, part := range geminiPartsFromFrame(frame) {
			if fn, ok := part["functionCall"].(map[string]any); ok {
				foundFunctionCall = true
				if fn["name"] != "Bash" {
					t.Fatalf("expected functionCall name Bash, got %#v", fn)
				}
			}
			if chunk, ok := part["text"].(string); ok {
				text.WriteString(chunk)
			}
		}
	}
	if leaked := text.String(); strings.Contains(strings.ToLower(leaked), "dsml") || strings.Contains(leaked, "<|") {
		t.Fatalf("native Gemini stream leaked tool markup: %q body=%s", leaked, rec.Body.String())
	}
	if !foundFunctionCall {
		t.Fatalf("expected Gemini functionCall output, body=%s", rec.Body.String())
	}
}

func TestNativeStreamGenerateContentHonorsExplicitToolChoiceNone(t *testing.T) {
	h := &Handler{}
	resp := makeGeminiUpstreamResponse(
		`data: {"p":"response/content","v":"<|DSML|tool_calls>\n<|DSML|invoke name=\"Bash\">\n<|DSML|parameter name=\"command\"><![CDATA[pwd]]></|DSML|parameter>\n</|DSML|invoke>\n</|DSML|tool_calls>"}`,
		`data: [DONE]`,
	)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-2.5-pro:streamGenerateContent", nil)

	h.handleStreamGenerateContent(rec, req, resp, "gemini-2.5-pro", "prompt", false, false, nil, nil, promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceNone})

	frames := extractGeminiSSEFrames(t, rec.Body.String())
	var text strings.Builder
	for _, frame := range frames {
		for _, part := range geminiPartsFromFrame(frame) {
			if _, ok := part["functionCall"]; ok {
				t.Fatalf("did not expect Gemini functionCall output when tool_choice is none, body=%s", rec.Body.String())
			}
			if chunk, ok := part["text"].(string); ok {
				text.WriteString(chunk)
			}
		}
	}
	if got := text.String(); !strings.Contains(got, "DSML|tool_calls") {
		t.Fatalf("expected raw tool markup to remain visible for tool_choice none, got %q body=%s", got, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run the Gemini-focused tests to verify RED**

Run:

```bash
go test ./internal/httpapi/gemini -run 'Test(NormalizeGeminiRequestSetsDefaultToolChoicePolicy|NativeStreamGenerateContentDetectsToolUseWithoutDeclaredTools|NativeStreamGenerateContentHonorsExplicitToolChoiceNone)$' -count=1
```

Expected: FAIL because Gemini normalization still leaves `ToolChoice` implicit, Gemini native stream entrypoints do not accept `ToolChoice`, and finalize-time Gemini turn building still ignores the policy.

- [ ] **Step 3: Implement explicit Gemini `ToolChoice` plumbing and shared policy usage**

Update `internal/httpapi/gemini/convert_request.go` so the returned `promptcompat.StandardRequest` includes an explicit default tool policy:

```go
return promptcompat.StandardRequest{
	Surface:         "google_gemini",
	RequestedModel:  requestedModel,
	ResolvedModel:   resolvedModel,
	ResponseModel:   requestedModel,
	Messages:        messagesRaw,
	PromptTokenText: finalPrompt,
	ToolsRaw:        toolsRaw,
	FinalPrompt:     finalPrompt,
	ToolNames:       toolNames,
	ToolChoice:      promptcompat.DefaultToolChoicePolicy(),
	Stream:          stream,
	Thinking:        thinkingEnabled,
	Search:          searchEnabled,
	PassThrough:     passThrough,
}, nil
```

Update `internal/httpapi/gemini/handler_generate.go` to thread `ToolChoice` into native stream setup:

```go
h.handleStreamGenerateContentWithRetry(
	w, r, a, start.Response, start.Payload, start.Pow,
	streamReq, streamReq.ResponseModel, streamReq.PromptTokenText,
	streamReq.Thinking, streamReq.Search, streamReq.ToolNames, streamReq.ToolsRaw,
	streamReq.ToolChoice, historySession,
)
```

Update `internal/httpapi/gemini/handler_stream_runtime.go` imports, signatures, and runtime state to:

```go
import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"ds2api/internal/assistantturn"
	"ds2api/internal/auth"
	"ds2api/internal/completionruntime"
	dsprotocol "ds2api/internal/deepseek/protocol"
	"ds2api/internal/promptcompat"
	"ds2api/internal/responsehistory"
	"ds2api/internal/responserewrite"
	"ds2api/internal/sse"
	streamengine "ds2api/internal/stream"
	"ds2api/internal/toolpolicy"
)
```

```go
func (h *Handler) handleStreamGenerateContent(w http.ResponseWriter, r *http.Request, resp *http.Response, model, finalPrompt string, thinkingEnabled, searchEnabled bool, toolNames []string, toolsRaw any, toolChoice promptcompat.ToolChoicePolicy, historySessions ...*responsehistory.Session)
```

Inside `geminiStreamRuntime`, add the explicit policy field alongside the existing prompt and buffering state:

```go
model       string
finalPrompt string
toolChoice  promptcompat.ToolChoicePolicy

thinkingEnabled       bool
searchEnabled         bool
bufferContent         bool
stripReferenceMarkers bool
toolNames             []string
toolsRaw              any
```

```go
func (h *Handler) handleStreamGenerateContentWithRetry(w http.ResponseWriter, r *http.Request, a *auth.RequestAuth, resp *http.Response, payload map[string]any, pow string, stdReq promptcompat.StandardRequest, model, finalPrompt string, thinkingEnabled, searchEnabled bool, toolNames []string, toolsRaw any, toolChoice promptcompat.ToolChoicePolicy, historySession *responsehistory.Session)
```

```go
func newGeminiStreamRuntime(
	w http.ResponseWriter,
	rc *http.ResponseController,
	canFlush bool,
	model string,
	finalPrompt string,
	thinkingEnabled bool,
	searchEnabled bool,
	stripReferenceMarkers bool,
	toolNames []string,
	toolsRaw any,
	toolChoice promptcompat.ToolChoicePolicy,
	history *responsehistory.Session,
	responseReplacer *responserewrite.StreamReplacer,
) *geminiStreamRuntime {
	return &geminiStreamRuntime{
		w:                     w,
		rc:                    rc,
		canFlush:              canFlush,
		model:                 model,
		finalPrompt:           finalPrompt,
		toolChoice:            toolChoice,
		thinkingEnabled:       thinkingEnabled,
		searchEnabled:         searchEnabled,
		bufferContent:         toolpolicy.ShouldBufferToolContent(toolChoice),
		stripReferenceMarkers: stripReferenceMarkers,
		toolNames:             toolNames,
		toolsRaw:              toolsRaw,
		history:               history,
		responseReplacer:      responseReplacer,
		accumulator: assistantturn.NewAccumulator(assistantturn.AccumulatorOptions{
			ThinkingEnabled:       thinkingEnabled,
			SearchEnabled:         searchEnabled,
			StripReferenceMarkers: stripReferenceMarkers,
			ResponseReplacer:      responseReplacer,
		}),
	}
}
```

Pass `toolChoice` through both runtime constructor callsites in the same file:

```go
runtime := newGeminiStreamRuntime(
	w, rc, canFlush, model, finalPrompt,
	thinkingEnabled, searchEnabled, stripReferenceMarkersEnabled(),
	toolNames, toolsRaw, toolChoice, historySession,
	responserewrite.NewStreamReplacer(h.responseReplacementRules()),
)
```

```go
runtime := newGeminiStreamRuntime(
	w, rc, canFlush, model, finalPrompt,
	thinkingEnabled, searchEnabled, stripReferenceMarkersEnabled(),
	toolNames, toolsRaw, toolChoice, historySession,
	responserewrite.NewStreamReplacer(h.responseReplacementRules()),
)
```

Finally, update Gemini finalize-time turn building to pass the real policy through:

```go
turn := assistantturn.BuildTurnFromStreamSnapshot(assistantturn.StreamSnapshot{
	RawText:           rawText,
	VisibleText:       text,
	RawThinking:       rawThinking,
	VisibleThinking:   thinking,
	DetectionThinking: detectionThinking,
	ContentFilter:     s.contentFilter,
	ResponseMessageID: s.responseMessageID,
}, assistantturn.BuildOptions{
	Model:                 s.model,
	Prompt:                s.finalPrompt,
	SearchEnabled:         s.searchEnabled,
	StripReferenceMarkers: s.stripReferenceMarkers,
	ToolNames:             s.toolNames,
	ToolsRaw:              s.toolsRaw,
	ToolChoice:            s.toolChoice,
})
```

- [ ] **Step 4: Format the Gemini Go files**

Run:

```bash
gofmt -w \
  internal/httpapi/gemini/convert_request.go \
  internal/httpapi/gemini/handler_generate.go \
  internal/httpapi/gemini/handler_stream_runtime.go \
  internal/httpapi/gemini/convert_request_test.go \
  internal/httpapi/gemini/handler_test.go
```

- [ ] **Step 5: Run the Gemini-focused tests to verify GREEN**

Run:

```bash
go test ./internal/httpapi/gemini -run 'Test(NormalizeGeminiRequestSetsDefaultToolChoicePolicy|NativeStreamGenerateContentDetectsToolUseWithoutDeclaredTools|NativeStreamGenerateContentHonorsExplicitToolChoiceNone)$' -count=1
```

Expected: PASS.

- [ ] **Step 6: Run the wider cross-surface regression and full suite**

Run:

```bash
go test ./internal/assistantturn ./internal/httpapi/openai/chat ./internal/httpapi/openai/responses ./internal/httpapi/claude ./internal/httpapi/gemini -count=1
```

Then run:

```bash
go test ./... -count=1
```

Expected: both commands PASS.

- [ ] **Step 7: Commit the Gemini policy fix**

Run:

```bash
git add \
  internal/httpapi/gemini/convert_request.go \
  internal/httpapi/gemini/handler_generate.go \
  internal/httpapi/gemini/handler_stream_runtime.go \
  internal/httpapi/gemini/convert_request_test.go \
  internal/httpapi/gemini/handler_test.go
git commit -m "fix: unify native Gemini tool interception policy"
```
