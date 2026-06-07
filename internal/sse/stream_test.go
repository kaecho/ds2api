package sse

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"ds2api/internal/config"
	"ds2api/internal/responserewrite"
)

func makeLargeContentSSEBody(t *testing.T, payload string) string {
	t.Helper()
	line, err := json.Marshal(map[string]any{
		"p": "response/content",
		"v": payload,
	})
	if err != nil {
		t.Fatalf("marshal SSE line failed: %v", err)
	}
	return "data: " + string(line) + "\n" + "data: [DONE]\n"
}

func TestStartParsedLinePumpParsesAndStops(t *testing.T) {
	body := strings.NewReader("data: {\"p\":\"response/content\",\"v\":\"hi\"}\n\ndata: [DONE]\n")
	results, done := StartParsedLinePump(context.Background(), body, false, "text")

	collected := make([]LineResult, 0, 2)
	for r := range results {
		collected = append(collected, r)
	}
	if err := <-done; err != nil {
		t.Fatalf("unexpected scanner error: %v", err)
	}
	if len(collected) < 2 {
		t.Fatalf("expected at least 2 parsed results, got %d", len(collected))
	}
	if !collected[0].Parsed || len(collected[0].Parts) == 0 {
		t.Fatalf("expected first line to contain parsed content")
	}
	last := collected[len(collected)-1]
	if !last.Parsed || !last.Stop {
		t.Fatalf("expected last line to stop stream, got parsed=%v stop=%v", last.Parsed, last.Stop)
	}
}

func TestStartParsedLinePumpHandlesLongSingleSSELine(t *testing.T) {
	payload := strings.Repeat("x", 5*1024*1024+4096)
	results, done := StartParsedLinePump(context.Background(), strings.NewReader(makeLargeContentSSEBody(t, payload)), false, "text")

	var got strings.Builder
	var sawDone bool
	for r := range results {
		for _, p := range r.Parts {
			got.WriteString(p.Text)
		}
		if r.Stop {
			sawDone = true
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("unexpected long-line read error: %v", err)
	}
	if got.String() != payload {
		t.Fatalf("long SSE line payload mismatch: got len=%d want len=%d", got.Len(), len(payload))
	}
	if !sawDone {
		t.Fatal("expected DONE after long SSE line")
	}
}

func TestStartParsedLinePumpWithReplacementsDedupesSourceSnapshots(t *testing.T) {
	rules := []config.ResponseReplacementRule{
		{From: "<|DEML", To: "<|DSML"},
		{From: "</|DEML", To: "</|DSML"},
	}
	text := `<|DEML|tool_calls>
  <|DEML|invoke name="mcp__exa__web_search_exa">
    <|DEML|parameter name="query"><![CDATA[2026年5月17日 原油价格 WTI Brent]]></|DEML|parameter>
    <|DEML|parameter name="numResults"><![CDATA[5]]></|DEML|parameter>
  </|DEML|invoke>
</|DEML|tool_calls>`
	line := strings.TrimSuffix(makeLargeContentSSEBody(t, text), "data: [DONE]\n")
	body := line + line + "data: [DONE]\n"
	results, done := StartParsedLinePumpWithReplacements(context.Background(), strings.NewReader(body), false, "text", rules)

	var got strings.Builder
	for r := range results {
		for _, p := range r.Parts {
			got.WriteString(p.Text)
		}
	}
	if err := <-done; err != nil {
		t.Fatalf("unexpected scanner error: %v", err)
	}
	want := responserewrite.Apply(text, rules)
	if got.String() != want {
		t.Fatalf("normalized text = %q, want %q", got.String(), want)
	}
}
