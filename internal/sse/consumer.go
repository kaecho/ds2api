package sse

import (
	"net/http"
	"strings"

	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
	"ds2api/internal/util"
)

// CollectResult holds the aggregated text and thinking content from a
// DeepSeek SSE stream, consumed to completion (non-streaming use case).
type CollectResult struct {
	Text                  string
	Thinking              string
	ToolDetectionThinking string
	ContentFilter         bool
	CitationLinks         map[int]string
	ResponseMessageID     int
}

// CollectStream fully consumes a DeepSeek SSE response and separates
// thinking content from text content. This replaces the duplicated
// stream-collection logic in openai.handleNonStream, claude.collectDeepSeek,
// and admin.testAccount.
//
// The caller is responsible for closing resp.Body unless closeBody is true.
func CollectStream(resp *http.Response, thinkingEnabled bool, closeBody bool) CollectResult {
	return CollectStreamWithReplacements(resp, thinkingEnabled, closeBody, nil)
}

// CollectStreamWithReplacements is like CollectStream but applies
// response-replacement rules to each text/thinking chunk before accumulating.
// Pass nil or an empty slice for replacements to skip rewriting.
func CollectStreamWithReplacements(resp *http.Response, thinkingEnabled bool, closeBody bool, replacements []config.ResponseReplacementRule) CollectResult {
	if closeBody {
		defer func() { _ = resp.Body.Close() }()
	}
	text := strings.Builder{}
	thinking := strings.Builder{}
	toolDetectionThinking := strings.Builder{}
	contentFilter := false
	stopped := false
	collector := newCitationLinkCollector()
	normalizer := NewContentNormalizer(replacements)
	responseMessageID := 0
	currentType := "text"
	if thinkingEnabled {
		currentType = "thinking"
	}
	appendResult := func(result LineResult) {
		for _, p := range result.Parts {
			if p.Type == "thinking" {
				thinking.WriteString(p.Text)
			} else {
				text.WriteString(p.Text)
			}
		}
		for _, p := range result.ToolDetectionThinkingParts {
			toolDetectionThinking.WriteString(p.Text)
		}
	}
	flushNormalizer := func() {
		appendResult(normalizer.Flush())
	}
	_ = dsprotocol.ScanSSELines(resp, func(line []byte) bool {
		chunk, done, parsed := ParseDeepSeekSSELine(line)
		if parsed && !done {
			collector.ingestChunk(chunk)
			observeResponseMessageID(chunk, &responseMessageID)
		}
		if done {
			flushNormalizer()
			return false
		}
		if stopped {
			return true
		}
		result := ParseDeepSeekContentLine(line, thinkingEnabled, currentType)
		currentType = result.NextType
		if !result.Parsed {
			return true
		}
		result = normalizer.Apply(result)
		if result.Stop {
			if result.ContentFilter {
				contentFilter = true
			}
			flushNormalizer()
			// Keep scanning to collect late-arriving citation metadata lines
			// that can appear after response/status=FINISHED, but stop as soon
			// as [DONE] arrives.
			stopped = true
			return true
		}
		appendResult(result)
		return true
	})
	flushNormalizer()
	return CollectResult{
		Text:                  text.String(),
		Thinking:              thinking.String(),
		ToolDetectionThinking: toolDetectionThinking.String(),
		ContentFilter:         contentFilter,
		CitationLinks:         collector.build(),
		ResponseMessageID:     responseMessageID,
	}
}

// observeResponseMessageID extracts the response_message_id from a parsed SSE
// chunk. It mirrors the extraction logic in client_continue.go's observe
// method, checking top-level response_message_id, v.response.message_id, and
// message.response.message_id.
func observeResponseMessageID(chunk map[string]any, out *int) {
	if chunk == nil || out == nil {
		return
	}
	if id := util.IntFrom(chunk["response_message_id"]); id > 0 {
		*out = id
	}
	v, _ := chunk["v"].(map[string]any)
	if response, _ := v["response"].(map[string]any); response != nil {
		if id := util.IntFrom(response["message_id"]); id > 0 {
			*out = id
		}
	}
	if message, _ := chunk["message"].(map[string]any); message != nil {
		if response, _ := message["response"].(map[string]any); response != nil {
			if id := util.IntFrom(response["message_id"]); id > 0 {
				*out = id
			}
		}
	}
}
