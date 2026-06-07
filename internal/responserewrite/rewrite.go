package responserewrite

import (
	"strings"
	"unicode/utf8"

	"ds2api/internal/config"
)

type Rule = config.ResponseReplacementRule

func Apply(text string, rules []Rule) string {
	for _, rule := range rules {
		if strings.TrimSpace(rule.From) == "" {
			continue
		}
		text = strings.ReplaceAll(text, rule.From, rule.To)
	}
	return text
}

func ReverseRules(rules []Rule) []Rule {
	if len(rules) == 0 {
		return nil
	}
	out := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		from := strings.TrimSpace(rule.To)
		if from == "" {
			continue
		}
		out = append(out, Rule{From: from, To: rule.From})
	}
	return out
}

type StreamReplacer struct {
	rules   []Rule
	pending string
	keep    int
}

func NewStreamReplacer(rules []Rule) *StreamReplacer {
	clean := make([]Rule, 0, len(rules))
	keep := 0
	for _, rule := range rules {
		if strings.TrimSpace(rule.From) == "" {
			continue
		}
		clean = append(clean, rule)
		if n := len(rule.From); n > keep {
			keep = n
		}
	}
	return &StreamReplacer{rules: clean, keep: keep}
}

func (r *StreamReplacer) Push(chunk string) string {
	if r == nil || len(r.rules) == 0 {
		return chunk
	}
	r.pending += chunk
	if len(r.pending) <= r.keep {
		return ""
	}
	emitLen := r.safeEmitLen()
	emitLen = floorUTF8Boundary(r.pending, emitLen)
	if emitLen <= 0 {
		return ""
	}
	emit := r.pending[:emitLen]
	r.pending = r.pending[emitLen:]
	return Apply(emit, r.rules)
}

func (r *StreamReplacer) safeEmitLen() int {
	emitLen := len(r.pending) - r.keep
	if emitLen <= 0 {
		return 0
	}
	for _, rule := range r.rules {
		from := rule.From
		if from == "" {
			continue
		}
		startMin := emitLen - len(from) + 1
		if startMin < 0 {
			startMin = 0
		}
		for start := startMin; start < emitLen && start < len(r.pending); start++ {
			prefixLen := len(r.pending) - start
			if prefixLen > len(from) {
				prefixLen = len(from)
			}
			if prefixLen > 0 && strings.HasPrefix(from, r.pending[start:start+prefixLen]) {
				return start
			}
		}
	}
	return emitLen
}

func (r *StreamReplacer) Flush() string {
	if r == nil || r.pending == "" {
		return ""
	}
	out := Apply(r.pending, r.rules)
	r.pending = ""
	return out
}

func floorUTF8Boundary(text string, limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit >= len(text) {
		return len(text)
	}
	for limit > 0 && !utf8.RuneStart(text[limit]) {
		limit--
	}
	return limit
}
