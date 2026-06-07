# Prompt Defaults and Response Replacements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show backend-provided default prompt text in WebUI and add configurable literal remote-response replacements that affect both tool parsing and final API output, including streaming.

**Architecture:** Keep prompt defaults in the backend packages that already own the text, then expose those defaults from `/admin/settings`. Add a small response replacement subsystem with a stateless full-text replacer and a stateful streaming replacer; apply it immediately after DeepSeek SSE content parsing and before tool-call parsing/output accumulation.

**Tech Stack:** Go backend (`internal/config`, `internal/sse`, `internal/httpapi/*`, `internal/toolcall`), React WebUI (`webui/src/features/settings`), JSON config codec, Go tests.

---

## File Structure

- Modify `internal/toolcall/tool_prompt.go` — expose backend-owned default tool-call instruction template.
- Modify `internal/httpapi/admin/settings/handler_settings_read.go` — include `default_text` values for prompt settings in `/admin/settings`.
- Modify `webui/src/features/settings/useSettingsForm.js` — consume backend `default_text` instead of hardcoded empty default.
- Modify `internal/config/config.go` — add `ResponseReplacementsConfig` and `ResponseReplacementRule`.
- Modify `internal/config/codec.go` — marshal, unmarshal, and clone response replacement config.
- Modify `internal/config/store_accessors.go` — expose response replacement accessors.
- Modify `internal/httpapi/admin/shared/deps.go` and `internal/httpapi/openai/shared/deps.go` — extend config interfaces.
- Modify `internal/httpapi/admin/settings/handler_settings_parse.go` — parse replacement rules from settings PUT.
- Modify `internal/httpapi/admin/settings/handler_settings_write.go` — persist replacement rules and hot-reload runtime state.
- Modify `internal/httpapi/admin/settings/handler_settings_read.go` — return replacement rules to WebUI.
- Create `internal/responserewrite/rewrite.go` — literal full-text and streaming replacement engine.
- Modify `internal/sse/consumer.go` — apply replacement in non-stream collection.
- Modify `internal/httpapi/openai/shared/stream_accumulator.go` — apply replacement before accumulation/output/tool parsing for streaming paths using this accumulator.
- Update stream runtime constructors/call sites as needed to pass replacer configuration into `StreamAccumulator`.
- Modify `webui/src/features/settings/PromptSection.jsx` — add response replacement UI.
- Modify `webui/src/features/settings/useSettingsForm.js` — map replacement rules from/to server payload.
- Modify `webui/src/locales/zh.json` and `webui/src/locales/en.json` — add labels/help text.
- Add/update tests in `internal/config/config_edge_test.go`, `internal/responserewrite/rewrite_test.go`, `internal/sse/consumer_test.go` or focused package tests, and admin settings tests.

---

## Task 1: Backend Default Text for Tool Call Instructions

**Files:**
- Modify: `internal/toolcall/tool_prompt.go`
- Modify: `internal/httpapi/admin/settings/handler_settings_read.go`
- Modify: `webui/src/features/settings/useSettingsForm.js`
- Test: `internal/toolcall/tool_prompt_test.go`
- Test: `internal/httpapi/admin/handler_settings_test.go`

- [ ] **Step 1: Write failing test for backend default template**

Add to `internal/toolcall/tool_prompt_test.go`:

```go
func TestDefaultToolCallInstructionsTemplateContainsRulesAndExamples(t *testing.T) {
	out := DefaultToolCallInstructionsTemplate()
	for _, want := range []string{
		"TOOL CALL FORMAT — FOLLOW EXACTLY:",
		"RULES:",
		"PARAMETER SHAPES:",
		"【WRONG — Do NOT do these】:",
		"【CORRECT EXAMPLES】:",
		`<|DSML|invoke name="Bash">`,
		`<|DSML|invoke name="Edit">`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("default template missing %q in:\n%s", want, out)
		}
	}
}
```

Ensure `strings` is imported in that test file if not already present.

- [ ] **Step 2: Run test and verify RED**

Run:

```bash
go test ./internal/toolcall -run TestDefaultToolCallInstructionsTemplateContainsRulesAndExamples -count=1
```

Expected: FAIL because `DefaultToolCallInstructionsTemplate` is undefined.

- [ ] **Step 3: Implement default template function**

In `internal/toolcall/tool_prompt.go`, refactor `BuildToolCallInstructions` so the default branch delegates to a helper:

```go
func DefaultToolCallInstructionsTemplate() string {
	return defaultToolCallInstructions([]string{"Bash", "Edit"})
}

func BuildToolCallInstructions(toolNames []string) string {
	if !ToolCallInstructionsEnabled {
		return ""
	}
	if strings.TrimSpace(ToolCallInstructionsText) != "" {
		return strings.TrimSpace(ToolCallInstructionsText)
	}
	return defaultToolCallInstructions(toolNames)
}

func defaultToolCallInstructions(toolNames []string) string {
	return `TOOL CALL FORMAT — FOLLOW EXACTLY:

<|DSML|tool_calls>
  <|DSML|invoke name="TOOL_NAME_HERE">
    <|DSML|parameter name="PARAMETER_NAME"><![CDATA[PARAMETER_VALUE]]></|DSML|parameter>
  </|DSML|invoke>
</|DSML|tool_calls>

RULES:
1) Use the <|DSML|tool_calls> wrapper format.
2) Put one or more <|DSML|invoke> entries under a single <|DSML|tool_calls> root.
3) Put the tool name in the invoke name attribute: <|DSML|invoke name="TOOL_NAME">.
3a) Tag punctuation alphabet: ASCII < > / = " plus the halfwidth pipe |.
4) All string values must use <![CDATA[...]]>, even short ones. This includes code, scripts, file contents, prompts, paths, names, and queries.
5) Every top-level argument must be a <|DSML|parameter name="ARG_NAME">...</|DSML|parameter> node.
6) Objects use nested XML elements inside the parameter body. Arrays may repeat <item> children.
7) Numbers, booleans, and null stay plain text.
8) Use only the parameter names in the tool schema. Do not invent fields.
9) Fill parameters with the actual values required for this call. Do not emit placeholder, blank, or whitespace-only parameters.
10) If a required parameter value is unknown, ask the user or answer normally instead of outputting an empty tool call.
11) For shell tools such as Bash / execute_command, the command/script must be inside the command parameter. Never call them with an empty command.
12) Do NOT wrap XML in markdown fences. Do NOT output explanations, role markers, or internal monologue.
13) If you call a tool, the first non-whitespace characters of that tool block must be exactly <|DSML|tool_calls>.
14) Never omit the opening <|DSML|tool_calls> tag, even if you already plan to close with </|DSML|tool_calls>.
15) Compatibility note: the runtime also accepts the legacy XML tags <tool_calls> / <invoke> / <parameter>, but prefer the DSML-prefixed form above.

PARAMETER SHAPES:
- string => <|DSML|parameter name="x"><![CDATA[value]]></|DSML|parameter>
- object => <|DSML|parameter name="x"><field>...</field></|DSML|parameter>
- array => <|DSML|parameter name="x"><item>...</item><item>...</item></|DSML|parameter>
- number/bool/null => <|DSML|parameter name="x">plain_text</|DSML|parameter>

【WRONG — Do NOT do these】:

Wrong 1 — mixed text after XML:
  <|DSML|tool_calls>...</|DSML|tool_calls> I hope this helps.
Wrong 2 — Markdown code fences:
  ` + "```xml" + `
  <|DSML|tool_calls>...</|DSML|tool_calls>
  ` + "```" + `
Wrong 3 — missing opening wrapper:
  <|DSML|invoke name="TOOL_NAME">...</|DSML|invoke>
  </|DSML|tool_calls>
Wrong 4 — empty parameters:
  <|DSML|tool_calls>
    <|DSML|invoke name="Bash">
      <|DSML|parameter name="command"></|DSML|parameter>
    </|DSML|invoke>
  </|DSML|tool_calls>

Remember: The ONLY valid way to use tools is the <|DSML|tool_calls>...</|DSML|tool_calls> block at the end of your response.
` + buildCorrectToolExamples(toolNames)
}
```

This preserves existing runtime behavior because `BuildToolCallInstructions` still returns the same string when custom text is empty.

- [ ] **Step 4: Verify toolcall test GREEN**

Run:

```bash
go test ./internal/toolcall -run TestDefaultToolCallInstructionsTemplateContainsRulesAndExamples -count=1
```

Expected: PASS.

- [ ] **Step 5: Write failing admin settings test for default_text**

Add to `internal/httpapi/admin/handler_settings_test.go`:

```go
func TestGetSettingsReturnsToolCallInstructionsDefaultText(t *testing.T) {
	h := newAdminTestHandler(t, `{"keys":["k1"]}`)
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	rec := httptest.NewRecorder()
	h.getSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	promptBody, _ := body["prompt"].(map[string]any)
	toolBody, _ := promptBody["tool_call_instructions"].(map[string]any)
	defaultText, _ := toolBody["default_text"].(string)
	if !strings.Contains(defaultText, "TOOL CALL FORMAT — FOLLOW EXACTLY:") || !strings.Contains(defaultText, "【CORRECT EXAMPLES】:") {
		t.Fatalf("unexpected default tool instruction text: %q", defaultText)
	}
}
```

Add `strings` to imports.

- [ ] **Step 6: Run admin test and verify RED**

Run:

```bash
go test ./internal/httpapi/admin -run TestGetSettingsReturnsToolCallInstructionsDefaultText -count=1
```

Expected: FAIL because response lacks `default_text`.

- [ ] **Step 7: Add default_text to settings response**

In `internal/httpapi/admin/settings/handler_settings_read.go`, import `ds2api/internal/toolcall` and change the map:

```go
"tool_call_instructions": map[string]any{
	"enabled":      h.Store.ToolCallInstructionsEnabled(),
	"text":         h.Store.ToolCallInstructionsText(),
	"default_text": toolcall.DefaultToolCallInstructionsTemplate(),
},
```

- [ ] **Step 8: Update WebUI to use backend default_text**

In `webui/src/features/settings/useSettingsForm.js`, change:

```js
default_text: '',
```

inside `tool_call_instructions` to:

```js
default_text: data.prompt?.tool_call_instructions?.default_text || '',
```

- [ ] **Step 9: Verify admin test and relevant frontend syntax**

Run:

```bash
go test ./internal/httpapi/admin -run TestGetSettingsReturnsToolCallInstructionsDefaultText -count=1
node -e "JSON.parse(require('fs').readFileSync('webui/src/locales/zh.json','utf8')); JSON.parse(require('fs').readFileSync('webui/src/locales/en.json','utf8')); console.log('locale json ok')"
```

Expected: Go test PASS and `locale json ok`.

---

## Task 2: Config Model and Codec for Response Replacements

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/codec.go`
- Modify: `internal/config/store_accessors.go`
- Test: `internal/config/config_edge_test.go`
- Test: `internal/config/store_accessors_test.go`

- [ ] **Step 1: Write failing config round-trip test**

Add to `internal/config/config_edge_test.go`:

```go
func TestConfigJSONRoundtripPreservesResponseReplacements(t *testing.T) {
	enabled := true
	cfg := Config{
		Keys: []string{"k1"},
		ResponseReplacements: ResponseReplacementsConfig{
			Enabled: &enabled,
			Rules: []ResponseReplacementRule{
				{From: "<|DEML", To: "<|DSML"},
				{From: "</|DEML", To: "</|DSML"},
			},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	if !strings.Contains(string(data), `"response_replacements"`) {
		t.Fatalf("expected response_replacements to marshal, got %s", data)
	}

	var decoded Config
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.ResponseReplacements.Enabled == nil || !*decoded.ResponseReplacements.Enabled {
		t.Fatalf("expected replacements enabled, got %#v", decoded.ResponseReplacements.Enabled)
	}
	if len(decoded.ResponseReplacements.Rules) != 2 || decoded.ResponseReplacements.Rules[0].From != "<|DEML" || decoded.ResponseReplacements.Rules[0].To != "<|DSML" {
		t.Fatalf("unexpected replacement rules: %#v", decoded.ResponseReplacements.Rules)
	}
}
```

- [ ] **Step 2: Run config test and verify RED**

Run:

```bash
go test ./internal/config -run TestConfigJSONRoundtripPreservesResponseReplacements -count=1
```

Expected: FAIL because config types do not exist.

- [ ] **Step 3: Add config types**

In `internal/config/config.go`, add field to `Config`:

```go
ResponseReplacements ResponseReplacementsConfig `json:"response_replacements,omitempty"`
```

Add types near other config structs:

```go
type ResponseReplacementsConfig struct {
	Enabled *bool                     `json:"enabled,omitempty"`
	Rules   []ResponseReplacementRule `json:"rules,omitempty"`
}

type ResponseReplacementRule struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}
```

- [ ] **Step 4: Add codec support**

In `internal/config/codec.go`, add MarshalJSON emission after prompt config:

```go
if hasResponseReplacementsConfig(c.ResponseReplacements) {
	m["response_replacements"] = c.ResponseReplacements
}
```

Add helper:

```go
func hasResponseReplacementsConfig(c ResponseReplacementsConfig) bool {
	return c.Enabled != nil || len(c.Rules) > 0
}
```

Add UnmarshalJSON case:

```go
case "response_replacements":
	if err := json.Unmarshal(v, &c.ResponseReplacements); err != nil {
		return fmt.Errorf("invalid field %q: %w", k, err)
	}
```

Add clone field in `Clone()`:

```go
ResponseReplacements: cloneResponseReplacementsConfig(c.ResponseReplacements),
```

Add clone helper:

```go
func cloneResponseReplacementsConfig(in ResponseReplacementsConfig) ResponseReplacementsConfig {
	return ResponseReplacementsConfig{
		Enabled: cloneBoolPtr(in.Enabled),
		Rules:   slices.Clone(in.Rules),
	}
}
```

- [ ] **Step 5: Verify config round-trip GREEN**

Run:

```bash
go test ./internal/config -run TestConfigJSONRoundtripPreservesResponseReplacements -count=1
```

Expected: PASS.

- [ ] **Step 6: Write accessor test**

Add to `internal/config/store_accessors_test.go`:

```go
func TestResponseReplacementsAccessors(t *testing.T) {
	enabled := true
	store := &Store{cfg: Config{ResponseReplacements: ResponseReplacementsConfig{
		Enabled: &enabled,
		Rules: []ResponseReplacementRule{
			{From: "<|DEML", To: "<|DSML"},
			{From: "", To: "ignored"},
		},
	}}}
	if !store.ResponseReplacementsEnabled() {
		t.Fatal("expected response replacements enabled")
	}
	rules := store.ResponseReplacementRules()
	if len(rules) != 1 || rules[0].From != "<|DEML" || rules[0].To != "<|DSML" {
		t.Fatalf("unexpected sanitized rules: %#v", rules)
	}
	rules[0].From = "mutated"
	if got := store.ResponseReplacementRules()[0].From; got != "<|DEML" {
		t.Fatalf("rules accessor returned mutable backing slice, got %q", got)
	}
}
```

- [ ] **Step 7: Run accessor test and verify RED**

Run:

```bash
go test ./internal/config -run TestResponseReplacementsAccessors -count=1
```

Expected: FAIL because accessors do not exist.

- [ ] **Step 8: Add store accessors**

In `internal/config/store_accessors.go`, add:

```go
func (s *Store) ResponseReplacementsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.ResponseReplacements.Enabled == nil {
		return false
	}
	return *s.cfg.ResponseReplacements.Enabled
}

func (s *Store) ResponseReplacementRules() []ResponseReplacementRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ResponseReplacementRule, 0, len(s.cfg.ResponseReplacements.Rules))
	for _, rule := range s.cfg.ResponseReplacements.Rules {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			continue
		}
		out = append(out, ResponseReplacementRule{From: from, To: rule.To})
	}
	return out
}
```

- [ ] **Step 9: Verify config package GREEN**

Run:

```bash
go test ./internal/config -count=1
```

Expected: PASS.

---

## Task 3: Literal Response Replacement Engine

**Files:**
- Create: `internal/responserewrite/rewrite.go`
- Test: `internal/responserewrite/rewrite_test.go`

- [ ] **Step 1: Write failing tests for full-text and stream replacement**

Create `internal/responserewrite/rewrite_test.go`:

```go
package responserewrite

import (
	"testing"

	"ds2api/internal/config"
)

func TestApplyLiteralReplacements(t *testing.T) {
	rules := []config.ResponseReplacementRule{{From: "<|DEML", To: "<|DSML"}, {From: "</|DEML", To: "</|DSML"}}
	got := Apply("<|DEML|tool_calls></|DEML|tool_calls>", rules)
	want := "<|DSML|tool_calls></|DSML|tool_calls>"
	if got != want {
		t.Fatalf("Apply()=%q want=%q", got, want)
	}
}

func TestApplySkipsEmptyFrom(t *testing.T) {
	rules := []config.ResponseReplacementRule{{From: "", To: "x"}, {From: "a", To: "b"}}
	if got := Apply("a", rules); got != "b" {
		t.Fatalf("Apply()=%q want=b", got)
	}
}

func TestStreamReplacerHandlesBoundarySplit(t *testing.T) {
	r := NewStreamReplacer([]config.ResponseReplacementRule{{From: "<|DEML", To: "<|DSML"}})
	parts := []string{
		r.Push("prefix <|DE"),
		r.Push("ML|tool_calls>"),
		r.Flush(),
	}
	got := parts[0] + parts[1] + parts[2]
	want := "prefix <|DSML|tool_calls>"
	if got != want {
		t.Fatalf("stream replacement=%q want=%q parts=%#v", got, want, parts)
	}
}

func TestStreamReplacerReturnsInputWhenNoRules(t *testing.T) {
	r := NewStreamReplacer(nil)
	if got := r.Push("abc"); got != "abc" {
		t.Fatalf("Push()=%q want=abc", got)
	}
	if got := r.Flush(); got != "" {
		t.Fatalf("Flush()=%q want empty", got)
	}
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```bash
go test ./internal/responserewrite -count=1
```

Expected: FAIL because package/functions do not exist.

- [ ] **Step 3: Implement replacement engine**

Create `internal/responserewrite/rewrite.go`:

```go
package responserewrite

import (
	"strings"

	"ds2api/internal/config"
)

type Rule = config.ResponseReplacementRule

func Apply(text string, rules []Rule) string {
	for _, rule := range rules {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			continue
		}
		text = strings.ReplaceAll(text, from, rule.To)
	}
	return text
}

type StreamReplacer struct {
	rules   []Rule
	pending string
	keep    int
}

func NewStreamReplacer(rules []Rule) *StreamReplacer {
	clean := make([]Rule, 0, len(rules))
	keep := 0
	for _, rule := range rules {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			continue
		}
		clean = append(clean, Rule{From: from, To: rule.To})
		if n := len(from) - 1; n > keep {
			keep = n
		}
	}
	return &StreamReplacer{rules: clean, keep: keep}
}

func (r *StreamReplacer) Push(chunk string) string {
	if r == nil || len(r.rules) == 0 {
		return chunk
	}
	r.pending += chunk
	if len(r.pending) <= r.keep {
		return ""
	}
	emitLen := len(r.pending) - r.keep
	emit := r.pending[:emitLen]
	r.pending = r.pending[emitLen:]
	return Apply(emit, r.rules)
}

func (r *StreamReplacer) Flush() string {
	if r == nil || r.pending == "" {
		return ""
	}
	out := Apply(r.pending, r.rules)
	r.pending = ""
	return out
}
```

- [ ] **Step 4: Verify replacement engine GREEN**

Run:

```bash
go test ./internal/responserewrite -count=1
```

Expected: PASS.

---

## Task 4: Apply Replacements in Non-Stream and Stream Paths

**Files:**
- Modify: `internal/sse/consumer.go`
- Modify: `internal/httpapi/openai/shared/stream_accumulator.go`
- Modify call sites constructing `StreamAccumulator`
- Test: add focused tests in `internal/sse` and `internal/httpapi/openai/shared`

- [ ] **Step 1: Write failing non-stream collection test**

Add to `internal/sse/consumer_test.go` or an existing SSE test file:

```go
func TestCollectStreamAppliesResponseReplacements(t *testing.T) {
	resp := &http.Response{Body: io.NopCloser(strings.NewReader("data: {\"p\":\"response/content\",\"v\":\"<|DEML|tool_calls>\"}\n\ndata: [DONE]\n\n"))}
	rules := []config.ResponseReplacementRule{{From: "<|DEML", To: "<|DSML"}}
	got := CollectStreamWithReplacements(resp, false, true, rules)
	if got.Text != "<|DSML|tool_calls>" {
		t.Fatalf("Text=%q", got.Text)
	}
}
```

Imports needed: `io`, `net/http`, `strings`, `testing`, `ds2api/internal/config`.

- [ ] **Step 2: Run test and verify RED**

Run:

```bash
go test ./internal/sse -run TestCollectStreamAppliesResponseReplacements -count=1
```

Expected: FAIL because `CollectStreamWithReplacements` does not exist.

- [ ] **Step 3: Add non-stream replacement-aware collector**

In `internal/sse/consumer.go`, keep existing signature as wrapper:

```go
func CollectStream(resp *http.Response, thinkingEnabled bool, closeBody bool) CollectResult {
	return CollectStreamWithReplacements(resp, thinkingEnabled, closeBody, nil)
}
```

Add function:

```go
func CollectStreamWithReplacements(resp *http.Response, thinkingEnabled bool, closeBody bool, replacements []config.ResponseReplacementRule) CollectResult {
```

Inside the loop, before writing `trimmed` to `thinking`, `text`, and `toolDetectionThinking`, apply:

```go
trimmed = responserewrite.Apply(trimmed, replacements)
```

Add imports:

```go
"ds2api/internal/config"
"ds2api/internal/responserewrite"
```

- [ ] **Step 4: Pass replacement rules from non-stream runtimes**

Find calls to `sse.CollectStream(...)` in `internal/completionruntime/nonstream.go` and update them to use rules from runtime options. If `completionruntime` already has a config object, add a field:

```go
ResponseReplacements []config.ResponseReplacementRule
```

Then call:

```go
collected := sse.CollectStreamWithReplacements(resp, thinkingEnabled, true, opts.ResponseReplacements)
```

For call sites without config, pass nil to preserve behavior.

- [ ] **Step 5: Write failing stream accumulator test**

Add to `internal/httpapi/openai/shared/stream_accumulator_test.go`:

```go
func TestStreamAccumulatorAppliesResponseReplacementsAcrossChunks(t *testing.T) {
	acc := &StreamAccumulator{
		ResponseReplacer: responserewrite.NewStreamReplacer([]config.ResponseReplacementRule{{From: "<|DEML", To: "<|DSML"}}),
	}
	first := acc.Apply(sse.LineResult{Parts: []sse.ContentPart{{Type: "text", Text: "<|DE"}}})
	second := acc.Apply(sse.LineResult{Parts: []sse.ContentPart{{Type: "text", Text: "ML|tool_calls>"}}})
	flushed := acc.FlushResponseReplacements()
	got := collectVisibleParts(first.Parts) + collectVisibleParts(second.Parts) + flushed.VisibleText
	if got != "<|DSML|tool_calls>" {
		t.Fatalf("got %q", got)
	}
}
```

Add helper in test file:

```go
func collectVisibleParts(parts []StreamPartDelta) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.VisibleText)
	}
	return b.String()
}
```

- [ ] **Step 6: Run stream accumulator test and verify RED**

Run:

```bash
go test ./internal/httpapi/openai/shared -run TestStreamAccumulatorAppliesResponseReplacementsAcrossChunks -count=1
```

Expected: FAIL because accumulator has no replacer/flush support.

- [ ] **Step 7: Add stream replacement to accumulator**

In `internal/httpapi/openai/shared/stream_accumulator.go`, add import:

```go
"ds2api/internal/responserewrite"
```

Add field:

```go
ResponseReplacer *responserewrite.StreamReplacer
```

Add method:

```go
func (a *StreamAccumulator) replaceResponseText(text string) string {
	if a.ResponseReplacer == nil {
		return text
	}
	return a.ResponseReplacer.Push(text)
}

func (a *StreamAccumulator) FlushResponseReplacements() StreamPartDelta {
	if a.ResponseReplacer == nil {
		return StreamPartDelta{Type: "text"}
	}
	flushed := a.ResponseReplacer.Flush()
	if flushed == "" {
		return StreamPartDelta{Type: "text"}
	}
	a.RawText.WriteString(flushed)
	cleaned := CleanVisibleOutput(flushed, a.StripReferenceMarkers)
	trimmed := sse.TrimContinuationOverlapFromBuilder(&a.Text, cleaned)
	if trimmed != "" {
		a.Text.WriteString(trimmed)
	}
	return StreamPartDelta{Type: "text", RawText: flushed, VisibleText: trimmed}
}
```

In `applyTextPart`, after `rawTrimmed` and before writing to `RawText`, add:

```go
rawTrimmed = a.replaceResponseText(rawTrimmed)
if rawTrimmed == "" {
	return StreamPartDelta{Type: "text"}
}
```

Do the same for `applyThinkingPart` if thinking text should also be replaceable. For this feature, apply to both text and thinking so final output is consistently rewritten.

- [ ] **Step 8: Wire streaming flush at stream finalization points**

Search for `StreamAccumulator` usage. At every streaming finalization path, call `FlushResponseReplacements()` before final tool-call detection/final output. The key files are:

- `internal/httpapi/openai/chat/chat_stream_runtime.go`
- `internal/httpapi/openai/responses/responses_stream_runtime_core.go`
- `internal/httpapi/claude/stream_runtime_core.go`
- `internal/httpapi/gemini/handler_stream_runtime.go`

When constructing `StreamAccumulator`, pass:

```go
ResponseReplacer: responserewrite.NewStreamReplacer(responseReplacementRules),
```

If a runtime does not use `StreamAccumulator`, apply `responserewrite.Apply(...)` to the raw text before `toolstream.ProcessChunk(...)` and before appending to final output.

- [ ] **Step 9: Verify stream and non-stream affected tests**

Run:

```bash
go test ./internal/sse ./internal/httpapi/openai/shared ./internal/httpapi/openai/chat ./internal/httpapi/openai/responses ./internal/httpapi/claude ./internal/httpapi/gemini -count=1
```

Expected: PASS.

---

## Task 5: Admin Settings API and WebUI for Response Replacements

**Files:**
- Modify: `internal/httpapi/admin/shared/deps.go`
- Modify: `internal/httpapi/openai/shared/deps.go`
- Modify: `internal/httpapi/admin/settings/handler_settings_parse.go`
- Modify: `internal/httpapi/admin/settings/handler_settings_write.go`
- Modify: `internal/httpapi/admin/settings/handler_settings_read.go`
- Modify: `webui/src/features/settings/useSettingsForm.js`
- Modify: `webui/src/features/settings/PromptSection.jsx`
- Modify: `webui/src/locales/zh.json`
- Modify: `webui/src/locales/en.json`
- Test: `internal/httpapi/admin/handler_settings_test.go`

- [ ] **Step 1: Write failing admin settings persistence test**

Add to `internal/httpapi/admin/handler_settings_test.go`:

```go
func TestUpdateSettingsResponseReplacements(t *testing.T) {
	h := newAdminTestHandler(t, `{"keys":["k1"]}`)
	payload := map[string]any{
		"response_replacements": map[string]any{
			"enabled": true,
			"rules": []any{
				map[string]any{"from": " <|DEML ", "to": "<|DSML"},
				map[string]any{"from": "</|DEML", "to": "</|DSML"},
			},
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.updateSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	snap := h.Store.Snapshot()
	if snap.ResponseReplacements.Enabled == nil || !*snap.ResponseReplacements.Enabled {
		t.Fatalf("expected replacements enabled, got %#v", snap.ResponseReplacements.Enabled)
	}
	if len(snap.ResponseReplacements.Rules) != 2 || snap.ResponseReplacements.Rules[0].From != "<|DEML" {
		t.Fatalf("unexpected replacement rules: %#v", snap.ResponseReplacements.Rules)
	}
}
```

- [ ] **Step 2: Run admin test and verify RED**

Run:

```bash
go test ./internal/httpapi/admin -run TestUpdateSettingsResponseReplacements -count=1
```

Expected: FAIL because settings parser/writer does not know `response_replacements`.

- [ ] **Step 3: Extend config interfaces**

In `internal/httpapi/admin/shared/deps.go` and `internal/httpapi/openai/shared/deps.go`, add:

```go
ResponseReplacementsEnabled() bool
ResponseReplacementRules() []config.ResponseReplacementRule
```

- [ ] **Step 4: Parse response replacement settings**

In `internal/httpapi/admin/settings/handler_settings_parse.go`, extend `parseSettingsUpdateRequest` return values with `*config.ResponseReplacementsConfig`.

Add parser:

```go
func parseResponseReplacementsConfig(raw map[string]any) *config.ResponseReplacementsConfig {
	cfg := &config.ResponseReplacementsConfig{}
	if v, exists := raw["enabled"]; exists {
		b := boolFrom(v)
		cfg.Enabled = &b
	}
	if items, ok := raw["rules"].([]any); ok {
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			from := strings.TrimSpace(fmt.Sprintf("%v", m["from"]))
			if from == "" {
				continue
			}
			cfg.Rules = append(cfg.Rules, config.ResponseReplacementRule{From: from, To: fmt.Sprintf("%v", m["to"])})
		}
	}
	return cfg
}
```

In main parse function:

```go
if raw, ok := req["response_replacements"].(map[string]any); ok {
	responseReplacementsCfg = parseResponseReplacementsConfig(raw)
}
```

- [ ] **Step 5: Persist response replacement settings**

In `internal/httpapi/admin/settings/handler_settings_write.go`, unpack `responseReplacementsCfg`, track:

```go
responseReplacementsEnabledSet := hasNestedSettingsKey(req, "response_replacements", "enabled")
responseReplacementsRulesSet := hasNestedSettingsKey(req, "response_replacements", "rules")
```

In `h.Store.Update`:

```go
if responseReplacementsCfg != nil {
	if responseReplacementsEnabledSet {
		c.ResponseReplacements.Enabled = responseReplacementsCfg.Enabled
	}
	if responseReplacementsRulesSet {
		c.ResponseReplacements.Rules = responseReplacementsCfg.Rules
	}
}
```

- [ ] **Step 6: Return response replacement settings from GET**

In `internal/httpapi/admin/settings/handler_settings_read.go`, add top-level field:

```go
"response_replacements": map[string]any{
	"enabled": h.Store.ResponseReplacementsEnabled(),
	"rules":   h.Store.ResponseReplacementRules(),
},
```

- [ ] **Step 7: Verify admin settings GREEN**

Run:

```bash
go test ./internal/httpapi/admin -run 'TestUpdateSettingsResponseReplacements|TestGetSettings' -count=1
```

Expected: PASS.

- [ ] **Step 8: Add WebUI form state mapping**

In `webui/src/features/settings/useSettingsForm.js`, add to `DEFAULT_FORM`:

```js
response_replacements: {
    enabled: false,
    rules: [],
},
```

In `fromServerForm(data)`:

```js
response_replacements: {
    enabled: Boolean(data.response_replacements?.enabled),
    rules: Array.isArray(data.response_replacements?.rules)
        ? data.response_replacements.rules.map((r) => ({ from: r.from || '', to: r.to || '' }))
        : [],
},
```

In `toServerPayload(form, baseHeaders)`:

```js
response_replacements: {
    enabled: Boolean(form.response_replacements?.enabled),
    rules: (form.response_replacements?.rules || [])
        .map((r) => ({ from: String(r.from || '').trim(), to: String(r.to || '') }))
        .filter((r) => r.from),
},
```

- [ ] **Step 9: Add WebUI controls**

In `webui/src/features/settings/PromptSection.jsx`, add a section after empty retry suffix:

```jsx
<div className="grid grid-cols-1 md:grid-cols-2 gap-4">
    <ToggleRow
        t={t}
        label={t('settings.responseReplacementsEnabled')}
        desc={t('settings.responseReplacementsDesc')}
        checked={Boolean(form.response_replacements?.enabled)}
        onChange={(v) => setForm((prev) => ({
            ...prev,
            response_replacements: { ...prev.response_replacements, enabled: v },
        }))}
    />
    {Boolean(form.response_replacements?.enabled) && (
        <div className="md:col-span-2 space-y-2">
            {(form.response_replacements?.rules || []).map((rule, idx) => (
                <div key={idx} className="grid grid-cols-1 md:grid-cols-[1fr_1fr_auto] gap-2 items-end">
                    <TextInput
                        label={t('settings.responseReplacementFrom')}
                        value={rule.from || ''}
                        placeholder="<|DEML"
                        onChange={(v) => setForm((prev) => {
                            const rules = [...(prev.response_replacements?.rules || [])]
                            rules[idx] = { ...rules[idx], from: v }
                            return { ...prev, response_replacements: { ...prev.response_replacements, rules } }
                        })}
                    />
                    <TextInput
                        label={t('settings.responseReplacementTo')}
                        value={rule.to || ''}
                        placeholder="<|DSML"
                        onChange={(v) => setForm((prev) => {
                            const rules = [...(prev.response_replacements?.rules || [])]
                            rules[idx] = { ...rules[idx], to: v }
                            return { ...prev, response_replacements: { ...prev.response_replacements, rules } }
                        })}
                    />
                    <button
                        type="button"
                        onClick={() => setForm((prev) => ({
                            ...prev,
                            response_replacements: {
                                ...prev.response_replacements,
                                rules: (prev.response_replacements?.rules || []).filter((_, i) => i !== idx),
                            },
                        }))}
                        className="px-3 py-2 rounded-lg border border-border text-xs hover:bg-muted"
                    >
                        {t('settings.responseReplacementRemove')}
                    </button>
                </div>
            ))}
            <button
                type="button"
                onClick={() => setForm((prev) => ({
                    ...prev,
                    response_replacements: {
                        ...prev.response_replacements,
                        rules: [...(prev.response_replacements?.rules || []), { from: '', to: '' }],
                    },
                }))}
                className="px-3 py-2 rounded-lg border border-border text-xs hover:bg-muted"
            >
                {t('settings.responseReplacementAdd')}
            </button>
        </div>
    )}
</div>
```

- [ ] **Step 10: Add locale strings**

In `webui/src/locales/zh.json` settings block add:

```json
"responseReplacementsEnabled": "远端回复替换",
"responseReplacementsDesc": "默认关闭。对 DeepSeek 返回内容执行普通字符串替换，替换后的内容会用于工具解析和最终 API 输出，支持流式。",
"responseReplacementFrom": "原始文本",
"responseReplacementTo": "替换为",
"responseReplacementAdd": "添加替换规则",
"responseReplacementRemove": "删除"
```

In `webui/src/locales/en.json` settings block add:

```json
"responseReplacementsEnabled": "Remote Response Replacements",
"responseReplacementsDesc": "Disabled by default. Applies literal string replacements to DeepSeek response text before tool parsing and final API output, including streaming.",
"responseReplacementFrom": "From",
"responseReplacementTo": "To",
"responseReplacementAdd": "Add replacement rule",
"responseReplacementRemove": "Remove"
```

- [ ] **Step 11: Verify WebUI data files**

Run:

```bash
node -e "JSON.parse(require('fs').readFileSync('webui/src/locales/zh.json','utf8')); JSON.parse(require('fs').readFileSync('webui/src/locales/en.json','utf8')); console.log('locale json ok')"
```

Expected: `locale json ok`.

---

## Task 6: Runtime Wiring and Final Verification

**Files:**
- Modify runtime handlers as required by Task 4 call-site discoveries.
- Test: package tests touched by runtime wiring.

- [ ] **Step 1: Ensure every runtime has replacement rules available**

Where handlers already hold `Store shared.ConfigReader`, pass rules into stream/non-stream runtime structs:

```go
responseReplacementRules := h.Store.ResponseReplacementRules()
if !h.Store.ResponseReplacementsEnabled() {
	responseReplacementRules = nil
}
```

Use nil rules to preserve existing behavior.

- [ ] **Step 2: Add a parser-before-output regression**

Add a focused test where text containing DEML is rewritten before parsing:

```go
func TestResponseReplacementAllowsDEMLToolCallsToParseAsDSML(t *testing.T) {
	rules := []config.ResponseReplacementRule{{From: "<|DEML", To: "<|DSML"}, {From: "</|DEML", To: "</|DSML"}}
	text := responserewrite.Apply(`<|DEML|tool_calls><|DEML|invoke name="Bash"><|DEML|parameter name="command"><![CDATA[pwd]]></|DEML|parameter></|DEML|invoke></|DEML|tool_calls>`, rules)
	calls, ok := toolcall.ParseStandaloneToolCallsDetailed(text)
	if !ok || len(calls) != 1 || calls[0].Name != "Bash" {
		t.Fatalf("expected rewritten DEML tool call to parse, ok=%v calls=%#v", ok, calls)
	}
}
```

Place this in `internal/responserewrite/rewrite_test.go` or `internal/toolcall` depending on import cycle; `responserewrite` can import `toolcall`, so `internal/responserewrite/rewrite_test.go` is fine.

- [ ] **Step 3: Run targeted tests**

Run:

```bash
go test ./internal/responserewrite ./internal/sse ./internal/httpapi/openai/shared ./internal/httpapi/openai/chat ./internal/httpapi/openai/responses ./internal/httpapi/claude ./internal/httpapi/gemini ./internal/httpapi/admin ./internal/config -count=1
```

Expected: PASS.

- [ ] **Step 4: Run full Go build and tests**

Run:

```bash
go build ./...
go test ./...
```

Expected: both commands exit 0.

- [ ] **Step 5: Validate frontend syntax/assets**

If `webui/node_modules` exists, run:

```bash
cd webui && npm run build
```

Expected: Vite build succeeds.

If `webui/node_modules` does not exist, run:

```bash
node -e "JSON.parse(require('fs').readFileSync('webui/src/locales/zh.json','utf8')); JSON.parse(require('fs').readFileSync('webui/src/locales/en.json','utf8')); console.log('locale json ok')"
node -e "for (const f of ['webui/src/features/settings/PromptSection.jsx','webui/src/features/settings/useSettingsForm.js']) { const s=require('fs').readFileSync(f,'utf8'); const opens=(s.match(/\\{/g)||[]).length; const closes=(s.match(/\\}/g)||[]).length; if(opens!==closes){throw new Error(f+' brace mismatch')} } console.log('basic jsx brace check ok')"
```

Expected: both scripts print ok messages.

- [ ] **Step 6: Review uncommitted diff**

Run:

```bash
git diff --stat
git diff -- internal/config/config.go internal/config/codec.go internal/httpapi/admin/settings/handler_settings_read.go webui/src/features/settings/PromptSection.jsx
```

Expected: diff only includes prompt defaults and response replacement feature changes.

- [ ] **Step 7: Commit**

Run:

```bash
git add internal webui docs/superpowers/plans/2026-05-15-prompt-defaults-response-replacements.md
git commit -m "$(cat <<'EOF'
add prompt defaults and response replacement settings

Expose backend-owned default prompt text to the WebUI and add configurable literal response replacements that apply before tool parsing and final output, including streaming paths.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

Expected: commit created.

---

## Self-Review

- **Spec coverage:** Backend default text, WebUI placeholder, response replacement config, literal-only rules, parser/output scope, and streaming support all have tasks.
- **Placeholder scan:** No TBD/TODO placeholders. Every task names files and commands.
- **Type consistency:** Config type names are `ResponseReplacementsConfig` and `ResponseReplacementRule`; accessors are `ResponseReplacementsEnabled()` and `ResponseReplacementRules()`; WebUI payload key is `response_replacements`.
- **Scope note:** Race-safe atomic prompt config globals are outside this feature. Do not refactor global prompt state in this plan.
