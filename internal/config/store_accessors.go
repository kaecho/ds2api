package config

import (
	"os"
	"strconv"
	"strings"
)

func (s *Store) ModelAliases() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := DefaultModelAliases()
	for k, v := range s.cfg.ModelAliases {
		key := strings.TrimSpace(lower(k))
		val := strings.TrimSpace(lower(v))
		if key == "" || val == "" {
			continue
		}
		out[key] = val
	}
	return out
}

func (s *Store) ToolcallMode() string {
	return "feature_match"
}

func (s *Store) ToolcallEarlyEmitConfidence() string {
	return "high"
}

func (s *Store) ResponsesStoreTTLSeconds() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Responses.StoreTTLSeconds > 0 {
		return s.cfg.Responses.StoreTTLSeconds
	}
	return 900
}

func (s *Store) EmbeddingsProvider() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.cfg.Embeddings.Provider)
}

func (s *Store) AutoDeleteMode() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mode := strings.ToLower(strings.TrimSpace(s.cfg.AutoDelete.Mode))
	switch mode {
	case "none", "single", "all":
		return mode
	}
	if s.cfg.AutoDelete.Sessions {
		return "all"
	}
	return "none"
}

func (s *Store) AdminPasswordHash() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.cfg.Admin.PasswordHash)
}

func (s *Store) AdminJWTExpireHours() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Admin.JWTExpireHours > 0 {
		return s.cfg.Admin.JWTExpireHours
	}
	if raw := strings.TrimSpace(os.Getenv("DS2API_JWT_EXPIRE_HOURS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return 24
}

func (s *Store) AdminJWTValidAfterUnix() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.Admin.JWTValidAfterUnix
}

func (s *Store) RuntimeAccountMaxInflight() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Runtime.AccountMaxInflight > 0 {
		return s.cfg.Runtime.AccountMaxInflight
	}
	if raw := strings.TrimSpace(os.Getenv("DS2API_ACCOUNT_MAX_INFLIGHT")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return 2
}

func (s *Store) RuntimeAccountMaxQueue(defaultSize int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Runtime.AccountMaxQueue > 0 {
		return s.cfg.Runtime.AccountMaxQueue
	}
	if raw := strings.TrimSpace(os.Getenv("DS2API_ACCOUNT_MAX_QUEUE")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			return n
		}
	}
	if defaultSize < 0 {
		return 0
	}
	return defaultSize
}

func (s *Store) RuntimeGlobalMaxInflight(defaultSize int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Runtime.GlobalMaxInflight > 0 {
		return s.cfg.Runtime.GlobalMaxInflight
	}
	if raw := strings.TrimSpace(os.Getenv("DS2API_GLOBAL_MAX_INFLIGHT")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	if defaultSize < 0 {
		return 0
	}
	return defaultSize
}

func (s *Store) RuntimeTokenRefreshIntervalHours() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Runtime.TokenRefreshIntervalHours > 0 {
		return s.cfg.Runtime.TokenRefreshIntervalHours
	}
	return 6
}

// RuntimeAutoCleanBanned returns whether banned accounts should be automatically
// removed from the pool when detected during token refresh.
func (s *Store) RuntimeAutoCleanBanned() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Runtime.AutoCleanBanned == nil {
		return false
	}
	return *s.cfg.Runtime.AutoCleanBanned
}

func (s *Store) AutoDeleteSessions() bool {
	return s.AutoDeleteMode() != "none"
}

func (s *Store) CurrentInputFileEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.CurrentInputFile.Enabled == nil {
		return true
	}
	return *s.cfg.CurrentInputFile.Enabled
}

func (s *Store) CurrentInputFileMinChars() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg.CurrentInputFile.MinChars
}

func (s *Store) OutputIntegrityGuardEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.OutputIntegrityGuard == nil {
		return true
	}
	return *s.cfg.Prompt.OutputIntegrityGuard
}

func (s *Store) OutputIntegrityGuardText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.cfg.Prompt.OutputIntegrityGuardText)
}

func (s *Store) SentinelsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.Sentinels == nil || s.cfg.Prompt.Sentinels.Enabled == nil {
		return true
	}
	return *s.cfg.Prompt.Sentinels.Enabled
}

func (s *Store) SentinelOverrides() SentinelConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.Sentinels == nil {
		return SentinelConfig{}
	}
	return *s.cfg.Prompt.Sentinels
}

func (s *Store) ToolCallInstructionsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.ToolCallInstructions == nil || s.cfg.Prompt.ToolCallInstructions.Enabled == nil {
		return true
	}
	return *s.cfg.Prompt.ToolCallInstructions.Enabled
}

func (s *Store) ToolCallInstructionsText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.ToolCallInstructions == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.Prompt.ToolCallInstructions.Text)
}

func (s *Store) ReadToolCacheGuardEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.ReadToolCacheGuard == nil || s.cfg.Prompt.ReadToolCacheGuard.Enabled == nil {
		return true
	}
	return *s.cfg.Prompt.ReadToolCacheGuard.Enabled
}

func (s *Store) ReadToolCacheGuardText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.ReadToolCacheGuard == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.Prompt.ReadToolCacheGuard.Text)
}

func (s *Store) EmptyOutputRetrySuffixEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.EmptyOutputRetrySuffix == nil || s.cfg.Prompt.EmptyOutputRetrySuffix.Enabled == nil {
		return true
	}
	return *s.cfg.Prompt.EmptyOutputRetrySuffix.Enabled
}

func (s *Store) EmptyOutputRetrySuffixText() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Prompt.EmptyOutputRetrySuffix == nil {
		return ""
	}
	return strings.TrimSpace(s.cfg.Prompt.EmptyOutputRetrySuffix.Text)
}

func (s *Store) ResponseReplacementsEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.ResponseReplacements.Enabled == nil {
		return false
	}
	return *s.cfg.ResponseReplacements.Enabled
}

func (s *Store) ResponseReplacementRules() []ResponseReplacementRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.cfg.ResponseReplacements.Rules) == 0 {
		return nil
	}
	out := make([]ResponseReplacementRule, 0, len(s.cfg.ResponseReplacements.Rules))
	for _, rule := range s.cfg.ResponseReplacements.Rules {
		from := strings.TrimSpace(rule.From)
		if from == "" {
			continue
		}
		out = append(out, ResponseReplacementRule{From: from, To: rule.To})
	}
	return out
}

func (s *Store) ThinkingInjectionEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.ThinkingInjection.Enabled == nil {
		return true
	}
	return *s.cfg.ThinkingInjection.Enabled
}

func (s *Store) ThinkingInjectionPrompt() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return strings.TrimSpace(s.cfg.ThinkingInjection.Prompt)
}

func (s *Store) ClientConfigSnapshot() ClientConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	headers := cloneStringMap(s.cfg.Client.BaseHeaders)
	if len(headers) == 0 {
		headers = nil
	}
	return ClientConfig{
		Name:            strings.TrimSpace(s.cfg.Client.Name),
		Platform:        strings.TrimSpace(s.cfg.Client.Platform),
		Version:         strings.TrimSpace(s.cfg.Client.Version),
		AndroidAPILevel: strings.TrimSpace(s.cfg.Client.AndroidAPILevel),
		Locale:          strings.TrimSpace(s.cfg.Client.Locale),
		BaseHeaders:     headers,
	}
}
