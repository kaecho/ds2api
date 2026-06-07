package sse

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/config"
	"ds2api/internal/responserewrite"
)

func TestCollectStreamDedupesContinueSnapshotReplay(t *testing.T) {
	prefix := "我们被问到：这是一个很长的续答快照前缀，用来验证去重逻辑不会误伤正常 token。"
	body := strings.Join([]string{
		`data: {"v":{"response":{"fragments":[{"id":2,"type":"THINK","content":"` + prefix + `","references":[],"stage_id":1}]}}}`,
		``,
		`data: {"p":"response/status","v":"INCOMPLETE"}`,
		``,
		`data: {"v":{"response":{"fragments":[{"id":2,"type":"THINK","content":"` + prefix + `继续","references":[],"stage_id":1}]}}}`,
		``,
		`data: {"v":"分析"}`,
		``,
		`data: {"p":"response/status","v":"FINISHED"}`,
		``,
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	got := CollectStream(resp, true, true)
	if got.Thinking != prefix+"继续分析" {
		t.Fatalf("unexpected thinking after dedupe: %q", got.Thinking)
	}
}

func TestCollectStreamWithReplacementsDedupesSourceSnapshots(t *testing.T) {
	rules := []config.ResponseReplacementRule{
		{From: "<|DEML", To: "<|DSML"},
		{From: "</|DEML", To: "</|DSML"},
	}
	text := `<|DEML|tool_calls><|DEML|invoke name="mcp__exa__web_search_exa"><|DEML|parameter name="query"><![CDATA[2026年5月17日 原油价格 WTI Brent]]></|DEML|parameter></|DEML|invoke></|DEML|tool_calls>`
	line := func(v string) string {
		b, err := json.Marshal(map[string]any{"p": "response/content", "v": v})
		if err != nil {
			t.Fatal(err)
		}
		return "data: " + string(b)
	}
	body := strings.Join([]string{
		line(text),
		``,
		line(text),
		``,
		`data: [DONE]`,
		``,
	}, "\n")

	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	got := CollectStreamWithReplacements(resp, false, true, rules)
	want := responserewrite.Apply(text, rules)
	if got.Text != want {
		t.Fatalf("unexpected text after replacement dedupe: %q, want %q", got.Text, want)
	}
}
