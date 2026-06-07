package settings

import (
	"net/http"
	"strings"

	authn "ds2api/internal/auth"
	"ds2api/internal/config"
	"ds2api/internal/promptcompat"
	"ds2api/internal/toolcall"
)

func (h *Handler) getSettings(w http.ResponseWriter, _ *http.Request) {
	snap := h.Store.Snapshot()
	recommended := defaultRuntimeRecommended(len(snap.Accounts), h.Store.RuntimeAccountMaxInflight())
	needsSync := config.IsVercel() && snap.VercelSyncHash != "" && snap.VercelSyncHash != h.computeSyncHash()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"admin": map[string]any{
			"has_password_hash":        strings.TrimSpace(snap.Admin.PasswordHash) != "",
			"jwt_expire_hours":         h.Store.AdminJWTExpireHours(),
			"jwt_valid_after_unix":     snap.Admin.JWTValidAfterUnix,
			"default_password_warning": authn.UsingDefaultAdminKey(h.Store),
		},
		"runtime": map[string]any{
			"account_max_inflight":         h.Store.RuntimeAccountMaxInflight(),
			"account_max_queue":            h.Store.RuntimeAccountMaxQueue(recommended),
			"global_max_inflight":          h.Store.RuntimeGlobalMaxInflight(recommended),
			"token_refresh_interval_hours": h.Store.RuntimeTokenRefreshIntervalHours(),
			"auto_clean_banned":            h.Store.RuntimeAutoCleanBanned(),
		},
		"responses":   snap.Responses,
		"embeddings":  snap.Embeddings,
		"auto_delete": snap.AutoDelete,
		"current_input_file": map[string]any{
			"enabled":   h.Store.CurrentInputFileEnabled(),
			"min_chars": h.Store.CurrentInputFileMinChars(),
		},
		"thinking_injection": map[string]any{
			"enabled":        h.Store.ThinkingInjectionEnabled(),
			"prompt":         h.Store.ThinkingInjectionPrompt(),
			"default_prompt": promptcompat.DefaultThinkingInjectionPrompt,
		},
		"prompt": map[string]any{
			"output_integrity_guard":      h.Store.OutputIntegrityGuardEnabled(),
			"output_integrity_guard_text": h.Store.OutputIntegrityGuardText(),
			"sentinels":                   sentinelSettingsMap(snap.Prompt.Sentinels, h.Store.SentinelsEnabled()),
			"tool_call_instructions": map[string]any{
				"enabled":      h.Store.ToolCallInstructionsEnabled(),
				"text":         h.Store.ToolCallInstructionsText(),
				"default_text": toolcall.DefaultToolCallInstructionsTemplate(),
			},
			"read_tool_cache_guard": map[string]any{
				"enabled": h.Store.ReadToolCacheGuardEnabled(),
				"text":    h.Store.ReadToolCacheGuardText(),
			},
			"empty_output_retry_suffix": map[string]any{
				"enabled": h.Store.EmptyOutputRetrySuffixEnabled(),
				"text":    h.Store.EmptyOutputRetrySuffixText(),
			},
		},
		"response_replacements": map[string]any{
			"enabled": h.Store.ResponseReplacementsEnabled(),
			"rules":   h.Store.ResponseReplacementRules(),
		},
		"model_aliases": snap.ModelAliases,
		"client": map[string]any{
			"name":              snap.Client.Name,
			"platform":          snap.Client.Platform,
			"version":           snap.Client.Version,
			"android_api_level": snap.Client.AndroidAPILevel,
			"locale":            snap.Client.Locale,
			"base_headers":      baseHeadersMap(snap.Client.BaseHeaders),
		},
		"env_backed":        h.Store.IsEnvBacked(),
		"needs_vercel_sync": needsSync,
	})
}

func sentinelSettingsMap(sc *config.SentinelConfig, enabled bool) map[string]any {
	m := map[string]any{"enabled": enabled}
	if sc != nil {
		m["begin_sentence"] = sc.BeginSentence
		m["system"] = sc.System
		m["user"] = sc.User
		m["assistant"] = sc.Assistant
		m["tool"] = sc.Tool
		m["end_sentence"] = sc.EndSentence
		m["end_tool_results"] = sc.EndToolResults
		m["end_instructions"] = sc.EndInstructions
	} else {
		for _, k := range []string{"begin_sentence", "system", "user", "assistant", "tool", "end_sentence", "end_tool_results", "end_instructions"} {
			m[k] = ""
		}
	}
	return m
}

func baseHeadersMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
