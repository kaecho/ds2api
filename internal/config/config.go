package config

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
)

type Config struct {
	Keys                 []string                   `json:"keys,omitempty"`
	APIKeys              []APIKey                   `json:"api_keys,omitempty"`
	Accounts             []Account                  `json:"accounts,omitempty"`
	Proxies              []Proxy                    `json:"proxies,omitempty"`
	ModelAliases         map[string]string          `json:"model_aliases,omitempty"`
	Admin                AdminConfig                `json:"admin,omitempty"`
	Runtime              RuntimeConfig              `json:"runtime,omitempty"`
	Responses            ResponsesConfig            `json:"responses,omitempty"`
	Embeddings           EmbeddingsConfig           `json:"embeddings,omitempty"`
	AutoDelete           AutoDeleteConfig           `json:"auto_delete"`
	CurrentInputFile     CurrentInputFileConfig     `json:"current_input_file,omitempty"`
	ThinkingInjection    ThinkingInjectionConfig    `json:"thinking_injection,omitempty"`
	Prompt               PromptConfig               `json:"prompt,omitempty"`
	ResponseReplacements ResponseReplacementsConfig `json:"response_replacements,omitempty"`
	Client               ClientConfig               `json:"client,omitempty"`
	Vercel               VercelConfig               `json:"vercel,omitempty"`
	VercelSyncHash       string                     `json:"_vercel_sync_hash,omitempty"`
	VercelSyncTime       int64                      `json:"_vercel_sync_time,omitempty"`
	AdditionalFields     map[string]any             `json:"-"`
}

type Account struct {
	Name      string `json:"name,omitempty"`
	Remark    string `json:"remark,omitempty"`
	Email     string `json:"email,omitempty"`
	Mobile    string `json:"mobile,omitempty"`
	Password  string `json:"password,omitempty"`
	Token     string `json:"token,omitempty"`
	ProxyID   string `json:"proxy_id,omitempty"`
	DeviceID  string `json:"device_id,omitempty"`
	RangersID string `json:"rangers_id,omitempty"`
}

type APIKey struct {
	Key    string `json:"key"`
	Name   string `json:"name,omitempty"`
	Remark string `json:"remark,omitempty"`
}

type Proxy struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

func NormalizeProxy(p Proxy) Proxy {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.Type = strings.ToLower(strings.TrimSpace(p.Type))
	p.Host = strings.TrimSpace(p.Host)
	p.Username = strings.TrimSpace(p.Username)
	p.Password = strings.TrimSpace(p.Password)
	if p.ID == "" {
		p.ID = StableProxyID(p)
	}
	if p.Name == "" && p.Host != "" && p.Port > 0 {
		p.Name = fmt.Sprintf("%s:%d", p.Host, p.Port)
	}
	return p
}

func StableProxyID(p Proxy) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(p.Type)) + "|" + strings.ToLower(strings.TrimSpace(p.Host)) + "|" + fmt.Sprintf("%d", p.Port) + "|" + strings.TrimSpace(p.Username)))
	return "proxy_" + hex.EncodeToString(sum[:6])
}

func (c *Config) ClearAccountTokens() {
	if c == nil {
		return
	}
	for i := range c.Accounts {
		c.Accounts[i].Token = ""
	}
}

func (c *Config) NormalizeCredentials() {
	if c == nil {
		return
	}
	normalizedAPIKeys := normalizeAPIKeys(c.APIKeys)
	if len(normalizedAPIKeys) > 0 {
		c.APIKeys = normalizedAPIKeys
		c.Keys = apiKeysToStrings(c.APIKeys)
	} else {
		c.Keys = normalizeKeys(c.Keys)
		c.APIKeys = apiKeysFromStrings(c.Keys, nil)
	}

	for i := range c.Accounts {
		c.Accounts[i].Name = strings.TrimSpace(c.Accounts[i].Name)
		c.Accounts[i].Remark = strings.TrimSpace(c.Accounts[i].Remark)
	}

	c.Vercel = NormalizeVercelConfig(c.Vercel)
	c.normalizeModelAliases()
}

// DropInvalidAccounts removes accounts that cannot be addressed by admin APIs
// (no email and no normalizable mobile). This prevents legacy token-only
// records from becoming orphaned empty entries after token stripping.
func (c *Config) DropInvalidAccounts() {
	if c == nil || len(c.Accounts) == 0 {
		return
	}
	kept := make([]Account, 0, len(c.Accounts))
	for _, acc := range c.Accounts {
		if acc.Identifier() == "" {
			continue
		}
		kept = append(kept, acc)
	}
	c.Accounts = kept
}

func (c *Config) normalizeModelAliases() {
	if c == nil {
		return
	}

	aliases := map[string]string{}
	for k, v := range c.ModelAliases {
		key := strings.TrimSpace(lower(k))
		val := strings.TrimSpace(lower(v))
		if key == "" || val == "" {
			continue
		}
		aliases[key] = val
	}
	if len(aliases) == 0 {
		c.ModelAliases = nil
	} else {
		c.ModelAliases = aliases
	}
}

type AdminConfig struct {
	PasswordHash      string `json:"password_hash,omitempty"`
	JWTExpireHours    int    `json:"jwt_expire_hours,omitempty"`
	JWTValidAfterUnix int64  `json:"jwt_valid_after_unix,omitempty"`
}

type RuntimeConfig struct {
	AccountMaxInflight        int   `json:"account_max_inflight,omitempty"`
	AccountMaxQueue           int   `json:"account_max_queue,omitempty"`
	GlobalMaxInflight         int   `json:"global_max_inflight,omitempty"`
	TokenRefreshIntervalHours int   `json:"token_refresh_interval_hours,omitempty"`
	AutoCleanBanned           *bool `json:"auto_clean_banned,omitempty"`
}

type ResponsesConfig struct {
	StoreTTLSeconds int `json:"store_ttl_seconds,omitempty"`
}

type EmbeddingsConfig struct {
	Provider string `json:"provider,omitempty"`
}

type AutoDeleteConfig struct {
	Mode     string `json:"mode,omitempty"`
	Sessions bool   `json:"sessions,omitempty"`
}

type CurrentInputFileConfig struct {
	Enabled  *bool `json:"enabled,omitempty"`
	MinChars int   `json:"min_chars,omitempty"`
}

type ThinkingInjectionConfig struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Prompt  string `json:"prompt,omitempty"`
}

type PromptConfig struct {
	OutputIntegrityGuard     *bool            `json:"output_integrity_guard,omitempty"`
	OutputIntegrityGuardText string           `json:"output_integrity_guard_text,omitempty"`
	Sentinels                *SentinelConfig  `json:"sentinels,omitempty"`
	ToolCallInstructions     *TextBlockConfig `json:"tool_call_instructions,omitempty"`
	ReadToolCacheGuard       *TextBlockConfig `json:"read_tool_cache_guard,omitempty"`
	EmptyOutputRetrySuffix   *TextBlockConfig `json:"empty_output_retry_suffix,omitempty"`
}

type SentinelConfig struct {
	Enabled         *bool  `json:"enabled,omitempty"`
	BeginSentence   string `json:"begin_sentence,omitempty"`
	System          string `json:"system,omitempty"`
	User            string `json:"user,omitempty"`
	Assistant       string `json:"assistant,omitempty"`
	Tool            string `json:"tool,omitempty"`
	EndSentence     string `json:"end_sentence,omitempty"`
	EndToolResults  string `json:"end_tool_results,omitempty"`
	EndInstructions string `json:"end_instructions,omitempty"`
}

type TextBlockConfig struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Text    string `json:"text,omitempty"`
}

type ResponseReplacementsConfig struct {
	Enabled *bool                     `json:"enabled,omitempty"`
	Rules   []ResponseReplacementRule `json:"rules,omitempty"`
}

type ResponseReplacementRule struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

type ClientConfig struct {
	Name            string            `json:"name,omitempty"`
	Platform        string            `json:"platform,omitempty"`
	Version         string            `json:"version,omitempty"`
	AndroidAPILevel string            `json:"android_api_level,omitempty"`
	Locale          string            `json:"locale,omitempty"`
	BaseHeaders     map[string]string `json:"base_headers,omitempty"`
}

type VercelConfig struct {
	Token     string `json:"token,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	TeamID    string `json:"team_id,omitempty"`
}

func NormalizeVercelConfig(v VercelConfig) VercelConfig {
	return VercelConfig{
		Token:     strings.TrimSpace(v.Token),
		ProjectID: strings.TrimSpace(v.ProjectID),
		TeamID:    strings.TrimSpace(v.TeamID),
	}
}

func (c *Config) ClearVercelCredentials() {
	if c == nil {
		return
	}
	c.Vercel = VercelConfig{}
}
