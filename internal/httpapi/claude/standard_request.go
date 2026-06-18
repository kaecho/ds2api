package claude

import (
	"fmt"
	"strings"

	"ds2api/internal/config"
	"ds2api/internal/prompt"
	"ds2api/internal/promptcompat"
	"ds2api/internal/util"
)

type claudeNormalizedRequest struct {
	Standard           promptcompat.StandardRequest
	NormalizedMessages []any
}

func normalizeClaudeRequest(store ConfigReader, req map[string]any) (claudeNormalizedRequest, error) {
	model, _ := req["model"].(string)
	messagesRaw, _ := req["messages"].([]any)
	if strings.TrimSpace(model) == "" || len(messagesRaw) == 0 {
		return claudeNormalizedRequest{}, fmt.Errorf("request must include 'model' and 'messages'")
	}
	if _, ok := req["max_tokens"]; !ok {
		req["max_tokens"] = 8192
	}
	normalizedMessages := normalizeClaudeMessages(messagesRaw)
	payload := cloneMap(req)
	payload["messages"] = normalizedMessages
	toolsRequested, _ := req["tools"].([]any)
	payload["messages"] = injectClaudeToolPrompt(payload, normalizedMessages, toolsRequested)

	dsPayload := convertClaudeToDeepSeek(payload, store)
	dsModel, _ := dsPayload["model"].(string)
	defaultThinkingEnabled, searchEnabled, ok := config.GetModelConfig(dsModel)
	if !ok {
		searchEnabled = false
	}
	thinkingEnabled := util.ResolveThinkingEnabled(req, defaultThinkingEnabled)
	if config.IsNoThinkingModel(dsModel) {
		thinkingEnabled = false
	}
	finalPrompt := prompt.MessagesPrepareWithThinking(toMessageMaps(dsPayload["messages"]), thinkingEnabled)
	toolNames := extractClaudeToolNames(toolsRequested)
	if len(toolNames) == 0 && len(toolsRequested) > 0 {
		toolNames = []string{"__any_tool__"}
	}
	refFileIDs := collectRefFileIDsFromRequest(req)

	return claudeNormalizedRequest{
		Standard: promptcompat.StandardRequest{
			Surface:         "anthropic_messages",
			RequestedModel:  strings.TrimSpace(model),
			ResolvedModel:   dsModel,
			ResponseModel:   strings.TrimSpace(model),
			Messages:        normalizedMessages,
			PromptTokenText: finalPrompt,
			ToolsRaw:        toolsRequested,
			FinalPrompt:     finalPrompt,
			ToolNames:       toolNames,
			Stream:          util.ToBool(req["stream"]),
			Thinking:        thinkingEnabled,
			Search:          searchEnabled,
			RefFileIDs:      refFileIDs,
		},
		NormalizedMessages: normalizedMessages,
	}, nil
}

func injectClaudeToolPrompt(payload map[string]any, normalizedMessages []any, tools []any) []any {
	if len(tools) == 0 {
		return normalizedMessages
	}
	toolPrompt := strings.TrimSpace(buildClaudeToolPrompt(tools))
	if toolPrompt == "" {
		return normalizedMessages
	}

	// Prefer top-level Anthropic-style system prompt when available.
	if systemText, ok := payload["system"].(string); ok && strings.TrimSpace(systemText) != "" {
		payload["system"] = mergeSystemPrompt(systemText, toolPrompt)
		return normalizedMessages
	}

	messages := cloneAnySlice(normalizedMessages)
	for i := range messages {
		msg, ok := messages[i].(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if !strings.EqualFold(strings.TrimSpace(role), "system") {
			continue
		}
		copied := cloneMap(msg)
		copied["content"] = mergeSystemPrompt(strings.TrimSpace(fmt.Sprintf("%v", copied["content"])), toolPrompt)
		messages[i] = copied
		return messages
	}

	return append([]any{map[string]any{"role": "system", "content": toolPrompt}}, messages...)
}

// collectRefFileIDsFromRequest extracts file IDs from the request's ref_file_ids
// field (populated by preprocessClaudeImageInputs) and from any input_image blocks
// in the messages.
func collectRefFileIDsFromRequest(req map[string]any) []string {
	if len(req) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	addID := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	// Collect from ref_file_ids set by preprocessClaudeImageInputs
	if raw, ok := req["ref_file_ids"].([]any); ok {
		for _, v := range raw {
			if id := strings.TrimSpace(fmt.Sprintf("%v", v)); id != "" {
				addID(id)
			}
		}
	}
	// Collect from input_image blocks in messages
	if messages, ok := req["messages"].([]any); ok {
		collectFileIDsFromMessages(messages, addID)
	}
	return out
}

func collectFileIDsFromMessages(messages []any, addID func(string)) {
	for _, item := range messages {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			blockType := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", b["type"])))
			if blockType == "input_image" {
				if fileID := strings.TrimSpace(fmt.Sprintf("%v", b["file_id"])); fileID != "" {
					addID(fileID)
				}
			}
		}
	}
}

func mergeSystemPrompt(base, extra string) string {
	base = strings.TrimSpace(base)
	extra = strings.TrimSpace(extra)
	switch {
	case base == "":
		return extra
	case extra == "":
		return base
	default:
		return base + "\n\n" + extra
	}
}

func cloneAnySlice(in []any) []any {
	if len(in) == 0 {
		return nil
	}
	out := make([]any, len(in))
	copy(out, in)
	return out
}
