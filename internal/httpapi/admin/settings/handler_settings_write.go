package settings

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	authn "ds2api/internal/auth"
	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/prompt"
	"ds2api/internal/promptcompat"
	"ds2api/internal/toolcall"
)

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	adminCfg, runtimeCfg, responsesCfg, embeddingsCfg, autoDeleteCfg, currentInputCfg, thinkingInjCfg, promptCfg, aliasMap, clientCfg, err := parseSettingsUpdateRequest(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	if runtimeCfg != nil {
		if err := validateMergedRuntimeSettings(h.Store.Snapshot().Runtime, runtimeCfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
			return
		}
	}
	currentInputEnabledSet := hasNestedSettingsKey(req, "current_input_file", "enabled")
	currentInputMinCharsSet := hasNestedSettingsKey(req, "current_input_file", "min_chars")
	thinkingInjectionEnabledSet := hasNestedSettingsKey(req, "thinking_injection", "enabled")
	thinkingInjectionPromptSet := hasNestedSettingsKey(req, "thinking_injection", "prompt")
	promptOutputIntegrityGuardSet := hasNestedSettingsKey(req, "prompt", "output_integrity_guard")
	promptOutputIntegrityGuardTextSet := hasNestedSettingsKey(req, "prompt", "output_integrity_guard_text")
	promptSentinelsSet := hasNestedSettingsKey(req, "prompt", "sentinels")
	promptToolInstrSet := hasNestedSettingsKey(req, "prompt", "tool_call_instructions")
	promptReadCacheSet := hasNestedSettingsKey(req, "prompt", "read_tool_cache_guard")
	promptEmptyRetrySet := hasNestedSettingsKey(req, "prompt", "empty_output_retry_suffix")
	responseReplEnabledSet := hasNestedSettingsKey(req, "response_replacements", "enabled")
	responseReplRulesSet := hasNestedSettingsKey(req, "response_replacements", "rules")

	responseReplacementsCfg := parseResponseReplacementsConfig(req["response_replacements"])

	if err := h.Store.Update(func(c *config.Config) error {
		if adminCfg != nil {
			if adminCfg.JWTExpireHours > 0 {
				c.Admin.JWTExpireHours = adminCfg.JWTExpireHours
			}
		}
		if runtimeCfg != nil {
			if runtimeCfg.AccountMaxInflight > 0 {
				c.Runtime.AccountMaxInflight = runtimeCfg.AccountMaxInflight
			}
			if runtimeCfg.AccountMaxQueue > 0 {
				c.Runtime.AccountMaxQueue = runtimeCfg.AccountMaxQueue
			}
			if runtimeCfg.GlobalMaxInflight > 0 {
				c.Runtime.GlobalMaxInflight = runtimeCfg.GlobalMaxInflight
			}
			if runtimeCfg.TokenRefreshIntervalHours > 0 {
				c.Runtime.TokenRefreshIntervalHours = runtimeCfg.TokenRefreshIntervalHours
			}
			if runtimeCfg.AutoCleanBanned != nil {
				c.Runtime.AutoCleanBanned = runtimeCfg.AutoCleanBanned
			}
		}
		if responsesCfg != nil && responsesCfg.StoreTTLSeconds > 0 {
			c.Responses.StoreTTLSeconds = responsesCfg.StoreTTLSeconds
		}
		if embeddingsCfg != nil && strings.TrimSpace(embeddingsCfg.Provider) != "" {
			c.Embeddings.Provider = strings.TrimSpace(embeddingsCfg.Provider)
		}
		if autoDeleteCfg != nil {
			c.AutoDelete.Mode = autoDeleteCfg.Mode
			c.AutoDelete.Sessions = autoDeleteCfg.Sessions
		}
		if currentInputCfg != nil {
			if currentInputEnabledSet {
				c.CurrentInputFile.Enabled = currentInputCfg.Enabled
			}
			if currentInputMinCharsSet {
				c.CurrentInputFile.MinChars = currentInputCfg.MinChars
			}
		}
		if thinkingInjCfg != nil {
			if thinkingInjectionEnabledSet {
				c.ThinkingInjection.Enabled = thinkingInjCfg.Enabled
			}
			if thinkingInjectionPromptSet {
				c.ThinkingInjection.Prompt = thinkingInjCfg.Prompt
			}
		}
		if promptCfg != nil {
			if promptOutputIntegrityGuardSet {
				c.Prompt.OutputIntegrityGuard = promptCfg.OutputIntegrityGuard
			}
			if promptOutputIntegrityGuardTextSet {
				c.Prompt.OutputIntegrityGuardText = promptCfg.OutputIntegrityGuardText
			}
			if promptSentinelsSet {
				c.Prompt.Sentinels = promptCfg.Sentinels
			}
			if promptToolInstrSet {
				c.Prompt.ToolCallInstructions = promptCfg.ToolCallInstructions
			}
			if promptReadCacheSet {
				c.Prompt.ReadToolCacheGuard = promptCfg.ReadToolCacheGuard
			}
			if promptEmptyRetrySet {
				c.Prompt.EmptyOutputRetrySuffix = promptCfg.EmptyOutputRetrySuffix
			}
		}
		if responseReplacementsCfg != nil {
			if responseReplEnabledSet {
				c.ResponseReplacements.Enabled = responseReplacementsCfg.Enabled
			}
			if responseReplRulesSet {
				c.ResponseReplacements.Rules = responseReplacementsCfg.Rules
			}
		}
		if aliasMap != nil {
			c.ModelAliases = aliasMap
		}
		if clientCfg != nil {
			if clientCfg.Name != "" {
				c.Client.Name = clientCfg.Name
			}
			if clientCfg.Platform != "" {
				c.Client.Platform = clientCfg.Platform
			}
			if clientCfg.Version != "" {
				c.Client.Version = clientCfg.Version
			}
			if clientCfg.AndroidAPILevel != "" {
				c.Client.AndroidAPILevel = clientCfg.AndroidAPILevel
			}
			if clientCfg.Locale != "" {
				c.Client.Locale = clientCfg.Locale
			}
			if clientCfg.BaseHeaders != nil {
				c.Client.BaseHeaders = clientCfg.BaseHeaders
			}
		}
		return nil
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	h.applyClientConfig()
	h.applyPromptConfig()
	h.applyRuntimeSettings()
	needsSync := config.IsVercel() || h.Store.IsEnvBacked()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":             true,
		"message":             "settings updated and hot reloaded",
		"env_backed":          h.Store.IsEnvBacked(),
		"needs_vercel_sync":   needsSync,
		"manual_sync_message": "配置已保存。Vercel 部署请在 Vercel Sync 页面手动同步。",
	})
}

func (h *Handler) updateSettingsPassword(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}
	newPassword := strings.TrimSpace(fieldString(req, "new_password"))
	if newPassword == "" {
		newPassword = strings.TrimSpace(fieldString(req, "password"))
	}
	if len(newPassword) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "new password must be at least 4 characters"})
		return
	}

	now := time.Now().Unix()
	hash := authn.HashAdminPassword(newPassword)
	if err := h.Store.Update(func(c *config.Config) error {
		c.Admin.PasswordHash = hash
		c.Admin.JWTValidAfterUnix = now
		return nil
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":              true,
		"message":              "password updated",
		"force_relogin":        true,
		"jwt_valid_after_unix": now,
	})
}

func (h *Handler) applyClientConfig() {
	if h == nil || h.Store == nil {
		return
	}
	cfg := h.Store.ClientConfigSnapshot()
	dsprotocol.ApplyClientConfigOverrides(dsprotocol.ClientConfigOverride{
		Name:            cfg.Name,
		Platform:        cfg.Platform,
		Version:         cfg.Version,
		AndroidAPILevel: cfg.AndroidAPILevel,
		Locale:          cfg.Locale,
		BaseHeaders:     cfg.BaseHeaders,
	})
}

func (h *Handler) applyPromptConfig() {
	if h == nil || h.Store == nil {
		return
	}
	prompt.OutputIntegrityGuardEnabled = h.Store.OutputIntegrityGuardEnabled()
	prompt.OutputIntegrityGuardText = h.Store.OutputIntegrityGuardText()

	prompt.SentinelEnabled = h.Store.SentinelsEnabled()
	prompt.ResetSentinelDefaults()
	overrides := h.Store.SentinelOverrides()
	if overrides.BeginSentence != "" {
		prompt.SentinelBeginSentence = overrides.BeginSentence
	}
	if overrides.System != "" {
		prompt.SentinelSystem = overrides.System
	}
	if overrides.User != "" {
		prompt.SentinelUser = overrides.User
	}
	if overrides.Assistant != "" {
		prompt.SentinelAssistant = overrides.Assistant
	}
	if overrides.Tool != "" {
		prompt.SentinelTool = overrides.Tool
	}
	if overrides.EndSentence != "" {
		prompt.SentinelEndSentence = overrides.EndSentence
	}
	if overrides.EndToolResults != "" {
		prompt.SentinelEndToolResults = overrides.EndToolResults
	}
	if overrides.EndInstructions != "" {
		prompt.SentinelEndInstructions = overrides.EndInstructions
	}

	toolcall.ToolCallInstructionsEnabled = h.Store.ToolCallInstructionsEnabled()
	toolcall.ToolCallInstructionsText = h.Store.ToolCallInstructionsText()

	promptcompat.ReadToolCacheGuardEnabled = h.Store.ReadToolCacheGuardEnabled()
	promptcompat.ReadToolCacheGuardText = h.Store.ReadToolCacheGuardText()

	shared.EmptyOutputRetrySuffixEnabled = h.Store.EmptyOutputRetrySuffixEnabled()
	shared.EmptyOutputRetrySuffixText = h.Store.EmptyOutputRetrySuffixText()
}

func hasNestedSettingsKey(req map[string]any, section, key string) bool {
	raw, ok := req[section].(map[string]any)
	if !ok {
		return false
	}
	_, exists := raw[key]
	return exists
}

func parseResponseReplacementsConfig(v any) *config.ResponseReplacementsConfig {
	raw, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	cfg := &config.ResponseReplacementsConfig{}
	if v, exists := raw["enabled"]; exists {
		b := BoolFrom(v)
		cfg.Enabled = &b
	}
	if items, ok := raw["rules"].([]any); ok {
		for _, item := range items {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			from := strings.TrimSpace(fmt.Sprintf("%v", m["from"]))
			if from == "" {
				continue
			}
			cfg.Rules = append(cfg.Rules, config.ResponseReplacementRule{From: from, To: fmt.Sprintf("%v", m["to"])})
		}
	}
	return cfg
}
