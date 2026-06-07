package sse

import (
	"strings"

	"ds2api/internal/config"
	"ds2api/internal/responserewrite"
)

type ContentNormalizer struct {
	textSource                  strings.Builder
	thinkingSource              strings.Builder
	toolDetectionThinkingSource strings.Builder

	textReplacer                  *responserewrite.StreamReplacer
	thinkingReplacer              *responserewrite.StreamReplacer
	toolDetectionThinkingReplacer *responserewrite.StreamReplacer
}

func NewContentNormalizer(replacements []config.ResponseReplacementRule) *ContentNormalizer {
	return &ContentNormalizer{
		textReplacer:                  responserewrite.NewStreamReplacer(replacements),
		thinkingReplacer:              responserewrite.NewStreamReplacer(replacements),
		toolDetectionThinkingReplacer: responserewrite.NewStreamReplacer(replacements),
	}
}

func (n *ContentNormalizer) Apply(result LineResult) LineResult {
	if n == nil || !result.Parsed {
		return result
	}
	out := result
	out.Parts = nil
	out.ToolDetectionThinkingParts = nil
	for _, p := range result.Parts {
		text := n.normalizePart(p.Type, p.Text)
		if text == "" {
			continue
		}
		p.Text = text
		out.Parts = append(out.Parts, p)
	}
	for _, p := range result.ToolDetectionThinkingParts {
		text := n.normalizeToolDetectionThinking(p.Text)
		if text == "" {
			continue
		}
		p.Text = text
		out.ToolDetectionThinkingParts = append(out.ToolDetectionThinkingParts, p)
	}
	return out
}

func (n *ContentNormalizer) Flush() LineResult {
	if n == nil {
		return LineResult{}
	}
	out := LineResult{Parsed: true}
	if text := n.thinkingReplacer.Flush(); text != "" {
		out.Parts = append(out.Parts, ContentPart{Type: "thinking", Text: text})
	}
	if text := n.textReplacer.Flush(); text != "" {
		out.Parts = append(out.Parts, ContentPart{Type: "text", Text: text})
	}
	if text := n.toolDetectionThinkingReplacer.Flush(); text != "" {
		out.ToolDetectionThinkingParts = append(out.ToolDetectionThinkingParts, ContentPart{Type: "thinking", Text: text})
	}
	return out
}

func (n *ContentNormalizer) normalizePart(kind, text string) string {
	if kind == "thinking" {
		return n.normalize(&n.thinkingSource, n.thinkingReplacer, text)
	}
	return n.normalize(&n.textSource, n.textReplacer, text)
}

func (n *ContentNormalizer) normalizeToolDetectionThinking(text string) string {
	return n.normalize(&n.toolDetectionThinkingSource, n.toolDetectionThinkingReplacer, text)
}

func (n *ContentNormalizer) normalize(source *strings.Builder, replacer *responserewrite.StreamReplacer, text string) string {
	trimmed := TrimContinuationOverlapFromBuilder(source, text)
	if trimmed == "" {
		return ""
	}
	source.WriteString(trimmed)
	if replacer == nil {
		return trimmed
	}
	return replacer.Push(trimmed)
}
