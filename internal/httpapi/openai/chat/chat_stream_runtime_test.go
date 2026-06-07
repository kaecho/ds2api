package chat

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ds2api/internal/promptcompat"
	"ds2api/internal/sse"
)

func TestChatStreamKeepAliveUsesCommentOnly(t *testing.T) {
	rec := httptest.NewRecorder()
	runtime := newChatStreamRuntime(
		rec,
		http.NewResponseController(rec),
		true,
		"chatcmpl-test",
		time.Now().Unix(),
		"deepseek-v4-flash",
		"prompt",
		false,
		false,
		true,
		nil,
		nil,
		promptcompat.DefaultToolChoicePolicy(),
		false,
		false,
	)

	runtime.sendKeepAlive()

	body := rec.Body.String()
	if !strings.Contains(body, ": keep-alive\n\n") {
		t.Fatalf("expected keep-alive comment, got %q", body)
	}
	frames, done := parseSSEDataFrames(t, body)
	if done {
		t.Fatalf("keep-alive must not emit [DONE], body=%q", body)
	}
	if len(frames) != 0 {
		t.Fatalf("keep-alive must not emit JSON data frames, got %#v body=%q", frames, body)
	}
}

func TestChatStreamSuppressesLateReasoningAfterContent(t *testing.T) {
	rec := httptest.NewRecorder()
	runtime := newChatStreamRuntime(
		rec,
		http.NewResponseController(rec),
		true,
		"chatcmpl-test",
		time.Now().Unix(),
		"deepseek-v4-flash",
		"prompt",
		true,
		false,
		true,
		nil,
		nil,
		promptcompat.DefaultToolChoicePolicy(),
		false,
		false,
	)

	runtime.onParsed(sse.LineResult{Parsed: true, Parts: []sse.ContentPart{
		{Type: "thinking", Text: "先想"},
		{Type: "text", Text: "你好"},
	}})
	runtime.onParsed(sse.LineResult{Parsed: true, Parts: []sse.ContentPart{
		{Type: "thinking", Text: "补想"},
		{Type: "text", Text: "吗"},
	}})

	frames, _ := parseSSEDataFrames(t, rec.Body.String())
	var reasoning strings.Builder
	var content strings.Builder
	for _, frame := range frames {
		choices, _ := frame["choices"].([]any)
		for _, item := range choices {
			choice, _ := item.(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			reasoning.WriteString(asString(delta["reasoning_content"]))
			content.WriteString(asString(delta["content"]))
		}
	}
	if got := reasoning.String(); got != "先想" {
		t.Fatalf("unexpected reasoning stream: got %q body=%s", got, rec.Body.String())
	}
	if got := content.String(); got != "你好吗" {
		t.Fatalf("unexpected content stream: got %q body=%s", got, rec.Body.String())
	}
	if got := runtime.accumulator.RawThinking.String(); got != "先想补想" {
		t.Fatalf("late raw thinking should be preserved, got %q", got)
	}
}

func TestChatStreamFinalizeEnforcesRequiredToolChoice(t *testing.T) {
	rec := httptest.NewRecorder()
	runtime := newChatStreamRuntime(
		rec,
		http.NewResponseController(rec),
		true,
		"chatcmpl-test",
		time.Now().Unix(),
		"deepseek-v4-flash",
		"prompt",
		false,
		false,
		true,
		[]string{"Write"},
		nil,
		promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceRequired},
		true,
		false,
	)

	if !runtime.finalize("stop", false) {
		t.Fatalf("expected terminal error to be written")
	}
	if runtime.finalErrorCode != "tool_choice_violation" {
		t.Fatalf("expected tool_choice_violation, got %q body=%s", runtime.finalErrorCode, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "tool_choice requires") {
		t.Fatalf("expected tool choice error in stream body, got %s", rec.Body.String())
	}
}
