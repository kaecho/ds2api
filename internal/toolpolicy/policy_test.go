package toolpolicy

import (
	"testing"

	"ds2api/internal/promptcompat"
)

func TestShouldBufferToolContent(t *testing.T) {
	tests := []struct {
		name   string
		policy promptcompat.ToolChoicePolicy
		want   bool
	}{
		{name: "default auto", policy: promptcompat.DefaultToolChoicePolicy(), want: true},
		{name: "none", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceNone}, want: false},
		{name: "required", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceRequired}, want: true},
		{name: "forced", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceForced}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldBufferToolContent(tc.policy); got != tc.want {
				t.Fatalf("ShouldBufferToolContent() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldParseToolCalls(t *testing.T) {
	tests := []struct {
		name   string
		policy promptcompat.ToolChoicePolicy
		want   bool
	}{
		{name: "default auto", policy: promptcompat.DefaultToolChoicePolicy(), want: true},
		{name: "none", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceNone}, want: false},
		{name: "required", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceRequired}, want: true},
		{name: "forced", policy: promptcompat.ToolChoicePolicy{Mode: promptcompat.ToolChoiceForced}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldParseToolCalls(tc.policy); got != tc.want {
				t.Fatalf("ShouldParseToolCalls() = %v, want %v", got, tc.want)
			}
		})
	}
}
