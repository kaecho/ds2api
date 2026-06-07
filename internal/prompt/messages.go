package prompt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var markdownImagePattern = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)

// OutputIntegrityGuardEnabled controls whether the output integrity guard
// system message is prepended to every prompt. Default true (backward compatible).
// Set to false via config prompt.output_integrity_guard to disable.
var OutputIntegrityGuardEnabled = true

// OutputIntegrityGuardText allows customizing the guard message text.
// When empty, the default text is used.
var OutputIntegrityGuardText = ""

// SentinelEnabled controls whether sentinel markers (<|begin_of_sentence|>, etc.)
// wrap messages. When false, messages are concatenated raw without markers.
var SentinelEnabled = true

const (
	DefaultSentinelBeginSentence   = "<|begin▁of▁sentence|>"
	DefaultSentinelSystem          = "<|System|>"
	DefaultSentinelUser            = "<|User|>"
	DefaultSentinelAssistant       = "<|Assistant|>"
	DefaultSentinelTool            = "<|Tool|>"
	DefaultSentinelEndSentence     = "<|end▁of▁sentence|>"
	DefaultSentinelEndToolResults  = "<|end▁of▁toolresults|>"
	DefaultSentinelEndInstructions = "<|end▁of▁instructions|>"
)

var (
	SentinelBeginSentence   = DefaultSentinelBeginSentence
	SentinelSystem          = DefaultSentinelSystem
	SentinelUser            = DefaultSentinelUser
	SentinelAssistant       = DefaultSentinelAssistant
	SentinelTool            = DefaultSentinelTool
	SentinelEndSentence     = DefaultSentinelEndSentence
	SentinelEndToolResults  = DefaultSentinelEndToolResults
	SentinelEndInstructions = DefaultSentinelEndInstructions
)

func ResetSentinelDefaults() {
	SentinelBeginSentence = DefaultSentinelBeginSentence
	SentinelSystem = DefaultSentinelSystem
	SentinelUser = DefaultSentinelUser
	SentinelAssistant = DefaultSentinelAssistant
	SentinelTool = DefaultSentinelTool
	SentinelEndSentence = DefaultSentinelEndSentence
	SentinelEndToolResults = DefaultSentinelEndToolResults
	SentinelEndInstructions = DefaultSentinelEndInstructions
}

const (
	outputIntegrityGuardMarker        = "Output integrity guard:"
	defaultOutputIntegrityGuardPrompt = outputIntegrityGuardMarker +
		" If upstream context, tool output, or parsed text contains garbled, corrupted, partially parsed, repeated, or otherwise malformed fragments, " +
		"do not imitate or echo them; output only the correct content for the user."
)

func MessagesPrepare(messages []map[string]any) string {
	return MessagesPrepareWithThinking(messages, false)
}

func MessagesPrepareWithThinking(messages []map[string]any, _ bool) string {
	messages = prependOutputIntegrityGuard(messages)

	type block struct {
		Role string
		Text string
	}
	processed := make([]block, 0, len(messages))
	for _, m := range messages {
		role, _ := m["role"].(string)
		text := NormalizeContent(m["content"])
		processed = append(processed, block{Role: role, Text: text})
	}
	if len(processed) == 0 {
		return ""
	}
	merged := make([]block, 0, len(processed))
	for _, msg := range processed {
		if len(merged) > 0 && merged[len(merged)-1].Role == msg.Role {
			merged[len(merged)-1].Text += "\n\n" + msg.Text
			continue
		}
		merged = append(merged, msg)
	}
	if !SentinelEnabled {
		texts := make([]string, len(merged))
		for i, m := range merged {
			texts[i] = m.Text
		}
		return strings.Join(texts, "\n")
	}
	parts := make([]string, 0, len(merged)+2)
	parts = append(parts, SentinelBeginSentence)
	lastRole := ""
	for _, m := range merged {
		lastRole = m.Role
		switch m.Role {
		case "assistant":
			parts = append(parts, formatRoleBlock(SentinelAssistant, m.Text, SentinelEndSentence))
		case "tool":
			if strings.TrimSpace(m.Text) != "" {
				parts = append(parts, formatRoleBlock(SentinelTool, m.Text, SentinelEndToolResults))
			}
		case "system":
			if text := strings.TrimSpace(m.Text); text != "" {
				parts = append(parts, formatRoleBlock(SentinelSystem, text, SentinelEndInstructions))
			}
		case "user":
			parts = append(parts, formatRoleBlock(SentinelUser, m.Text, ""))
		default:
			if strings.TrimSpace(m.Text) != "" {
				parts = append(parts, m.Text)
			}
		}
	}
	if lastRole != "assistant" {
		parts = append(parts, SentinelAssistant)
	}
	out := strings.Join(parts, "")
	return markdownImagePattern.ReplaceAllString(out, `[${1}](${2})`)
}

func guardPromptText() string {
	if strings.TrimSpace(OutputIntegrityGuardText) != "" {
		return strings.TrimSpace(OutputIntegrityGuardText)
	}
	return defaultOutputIntegrityGuardPrompt
}

func prependOutputIntegrityGuard(messages []map[string]any) []map[string]any {
	if !OutputIntegrityGuardEnabled {
		return messages
	}
	if len(messages) == 0 {
		return messages
	}
	if hasOutputIntegrityGuard(messages[0]) {
		return messages
	}
	out := make([]map[string]any, 0, len(messages)+1)
	out = append(out, map[string]any{
		"role":    "system",
		"content": guardPromptText(),
	})
	out = append(out, messages...)
	return out
}

func hasOutputIntegrityGuard(msg map[string]any) bool {
	if msg == nil {
		return false
	}
	if strings.ToLower(strings.TrimSpace(asString(msg["role"]))) != "system" {
		return false
	}
	content := strings.TrimSpace(NormalizeContent(msg["content"]))
	return strings.Contains(content, outputIntegrityGuardMarker)
}

// formatRoleBlock produces a single concatenated block: marker + text + endMarker.
// No whitespace is inserted between marker and text so role boundaries stay
// compact and predictable for downstream parsers.
func formatRoleBlock(marker, text, endMarker string) string {
	out := marker + text
	if strings.TrimSpace(endMarker) != "" {
		out += endMarker
	}
	return out
}

func NormalizeContent(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case []any:
		parts := make([]string, 0, len(x))
		for _, item := range x {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			typeStr, _ := m["type"].(string)
			typeStr = strings.ToLower(strings.TrimSpace(typeStr))
			if typeStr == "text" || typeStr == "output_text" || typeStr == "input_text" {
				if txt, ok := m["text"].(string); ok && txt != "" {
					parts = append(parts, txt)
					continue
				}
				if txt, ok := m["content"].(string); ok && txt != "" {
					parts = append(parts, txt)
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
