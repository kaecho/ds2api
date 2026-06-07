package responserewrite

import (
	"testing"
	"unicode/utf8"

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

func TestApplyPreservesNonEmptyFromWhitespace(t *testing.T) {
	rules := []config.ResponseReplacementRule{{From: " a ", To: "x"}}
	if got := Apply("a a ", rules); got != "ax" {
		t.Fatalf("Apply()=%q want=ax", got)
	}
}

func TestReverseRulesSwapsFromAndTo(t *testing.T) {
	rules := []config.ResponseReplacementRule{
		{From: "<|DEML", To: "<|DSML"},
		{From: "</|DEML", To: "</|DSML"},
	}

	got := ReverseRules(rules)

	want := []config.ResponseReplacementRule{
		{From: "<|DSML", To: "<|DEML"},
		{From: "</|DSML", To: "</|DEML"},
	}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rule[%d]=%#v want %#v", i, got[i], want[i])
		}
	}
}

func TestReverseRulesSkipsEmptyTo(t *testing.T) {
	rules := []config.ResponseReplacementRule{
		{From: "x", To: ""},
		{From: "a", To: "b"},
	}

	got := ReverseRules(rules)

	if len(got) != 1 {
		t.Fatalf("len=%d want 1: %#v", len(got), got)
	}
	if got[0] != (config.ResponseReplacementRule{From: "b", To: "a"}) {
		t.Fatalf("got %#v", got[0])
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

func TestStreamReplacerHandlesBoundarySplitAtEnd(t *testing.T) {
	r := NewStreamReplacer([]config.ResponseReplacementRule{{From: "<|DEML", To: "<|DSML"}})
	parts := []string{
		r.Push("<|DE"),
		r.Push("ML"),
		r.Flush(),
	}
	got := parts[0] + parts[1] + parts[2]
	want := "<|DSML"
	if got != want {
		t.Fatalf("stream replacement=%q want=%q parts=%#v", got, want, parts)
	}
}

func TestStreamReplacerDoesNotSplitClosingMatchAcrossEmitBoundary(t *testing.T) {
	r := NewStreamReplacer([]config.ResponseReplacementRule{
		{From: "<|DEML", To: "<|DSML"},
		{From: "</|DEML", To: "</|DSML"},
	})

	parts := []string{
		r.Push("</|DEML|tool_"),
		r.Flush(),
	}
	got := parts[0] + parts[1]
	want := "</|DSML|tool_"
	if got != want {
		t.Fatalf("stream replacement=%q want=%q parts=%#v", got, want, parts)
	}
}

func TestStreamReplacerDoesNotSplitOpeningMatchAcrossEmitBoundary(t *testing.T) {
	r := NewStreamReplacer([]config.ResponseReplacementRule{
		{From: "<|DEML", To: "<|DSML"},
		{From: "</|DEML", To: "</|DSML"},
	})

	parts := []string{
		r.Push("<|DEML|i"),
		r.Flush(),
	}
	got := parts[0] + parts[1]
	want := "<|DSML|i"
	if got != want {
		t.Fatalf("stream replacement=%q want=%q parts=%#v", got, want, parts)
	}
}

func TestStreamReplacerKeepsUTF8ChunkBoundaries(t *testing.T) {
	r := NewStreamReplacer([]config.ResponseReplacementRule{{From: "ABCDEFG", To: "HI"}})
	parts := []string{
		r.Push("你好世界"),
		r.Flush(),
	}
	got := parts[0] + parts[1]
	want := "你好世界"
	if got != want {
		t.Fatalf("stream replacement=%q want=%q parts=%#v", got, want, parts)
	}
	for i, part := range parts {
		if !utf8.ValidString(part) {
			t.Fatalf("part[%d] invalid utf-8: %q", i, part)
		}
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
