package toolpolicy

import "ds2api/internal/promptcompat"

func ShouldBufferToolContent(policy promptcompat.ToolChoicePolicy) bool {
	return policy.Mode != promptcompat.ToolChoiceNone
}

func ShouldParseToolCalls(policy promptcompat.ToolChoicePolicy) bool {
	return policy.Mode != promptcompat.ToolChoiceNone
}
