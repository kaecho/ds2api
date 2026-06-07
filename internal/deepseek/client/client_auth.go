package client

import (
	"context"
	"crypto/rand"
	dsprotocol "ds2api/internal/deepseek/protocol"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"ds2api/internal/auth"
	"ds2api/internal/config"
)

func (c *Client) Login(ctx context.Context, acc config.Account) (string, error) {
	clients := c.requestClientsForAccount(acc)
	deviceID := strings.TrimSpace(acc.DeviceID)
	if deviceID == "" {
		deviceID = uuid.New().String()
		if id := acc.Identifier(); id != "" {
			if err := c.Store.UpdateAccountDeviceID(id, deviceID); err != nil {
				config.Logger.Warn("[login] persist device_id failed", "account", id, "error", err)
			}
		}
	}
	rangersID := strings.TrimSpace(acc.RangersID)
	if rangersID == "" {
		rangersID = generateRangersID()
		if id := acc.Identifier(); id != "" {
			if err := c.Store.UpdateAccountRangersID(id, rangersID); err != nil {
				config.Logger.Warn("[login] persist rangers_id failed", "account", id, "error", err)
			}
		}
	}
	payload := map[string]any{
		"password":  strings.TrimSpace(acc.Password),
		"device_id": deviceID,
		"os":        "android",
	}
	if email := strings.TrimSpace(acc.Email); email != "" {
		payload["email"] = email
	} else if mobile := strings.TrimSpace(acc.Mobile); mobile != "" {
		loginMobile, areaCode := normalizeMobileForLogin(mobile)
		payload["mobile"] = loginMobile
		payload["area_code"] = areaCode
	} else {
		return "", errors.New("missing email/mobile")
	}
	resp, err := c.postJSON(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekLoginURL, dsprotocol.BaseHeaders, payload)
	if err != nil {
		return "", err
	}
	code := intFrom(resp["code"])
	if code != 0 {
		msg, _ := resp["msg"].(string)
		if isBannedError(msg) {
			return "", &RequestFailure{Op: "login", Kind: FailureBanned, Message: msg}
		}
		return "", fmt.Errorf("login failed: %v", resp["msg"])
	}
	data, _ := resp["data"].(map[string]any)
	if intFrom(data["biz_code"]) != 0 {
		bizMsg, _ := data["biz_msg"].(string)
		if isBannedError(bizMsg) {
			return "", &RequestFailure{Op: "login", Kind: FailureBanned, Message: bizMsg}
		}
		return "", fmt.Errorf("login failed: %v", data["biz_msg"])
	}
	bizData, _ := data["biz_data"].(map[string]any)
	user, _ := bizData["user"].(map[string]any)
	token, _ := user["token"].(string)
	if strings.TrimSpace(token) == "" {
		return "", errors.New("missing login token")
	}
	return token, nil
}

func (c *Client) CreateSession(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = c.maxRetries
	}
	clients := c.requestClientsForAuth(ctx, a)
	attempts := 0
	refreshed := false
	for attempts < maxAttempts {
		headers := c.authHeaders(a.DeepSeekToken, a)
		resp, status, err := c.postJSONWithStatus(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekCreateSessionURL, headers, nil)
		if err != nil {
			config.Logger.Warn("[create_session] request error", "error", err, "account", a.AccountID)
			attempts++
			continue
		}
		code, bizCode, msg, bizMsg := extractResponseStatus(resp)
		if status == http.StatusOK && code == 0 && bizCode == 0 {
			sessionID := extractCreateSessionID(resp)
			if sessionID != "" {
				return sessionID, nil
			}
		}
		config.Logger.Warn("[create_session] failed", "status", status, "code", code, "biz_code", bizCode, "msg", msg, "biz_msg", bizMsg, "use_config_token", a.UseConfigToken, "account", a.AccountID)
		if a.UseConfigToken {
			if !refreshed && shouldAttemptRefresh(status, code, bizCode, msg, bizMsg) {
				if c.Auth.RefreshToken(ctx, a) {
					refreshed = true
					continue
				}
			}
			if c.Auth.SwitchAccount(ctx, a) {
				refreshed = false
				attempts++
				continue
			}
		}
		attempts++
	}
	return "", errors.New("create session failed")
}

func (c *Client) GetPow(ctx context.Context, a *auth.RequestAuth, maxAttempts int) (string, error) {
	return c.GetPowForTarget(ctx, a, dsprotocol.DeepSeekCompletionTargetPath, maxAttempts)
}

func (c *Client) GetPowForTarget(ctx context.Context, a *auth.RequestAuth, targetPath string, maxAttempts int) (string, error) {
	if maxAttempts <= 0 {
		maxAttempts = c.maxRetries
	}
	targetPath = strings.TrimSpace(targetPath)
	if targetPath == "" {
		targetPath = dsprotocol.DeepSeekCompletionTargetPath
	}
	clients := c.requestClientsForAuth(ctx, a)
	attempts := 0
	refreshed := false
	lastFailureKind := FailureUnknown
	lastFailureMessage := ""
	for attempts < maxAttempts {
		headers := c.authHeaders(a.DeepSeekToken, a)
		resp, status, err := c.postJSONWithStatus(ctx, clients.regular, clients.fallback, dsprotocol.DeepSeekCreatePowURL, headers, map[string]any{"target_path": targetPath})
		if err != nil {
			config.Logger.Warn("[get_pow] request error", "error", err, "account", a.AccountID, "target_path", targetPath)
			lastFailureKind = FailureUnknown
			lastFailureMessage = err.Error()
			attempts++
			continue
		}
		code, bizCode, msg, bizMsg := extractResponseStatus(resp)
		if status == http.StatusOK && code == 0 && bizCode == 0 {
			data, _ := resp["data"].(map[string]any)
			bizData, _ := data["biz_data"].(map[string]any)
			challenge, _ := bizData["challenge"].(map[string]any)
			answer, err := ComputePow(ctx, challenge)
			if err != nil {
				attempts++
				continue
			}
			return BuildPowHeader(challenge, answer)
		}
		config.Logger.Warn("[get_pow] failed", "status", status, "code", code, "biz_code", bizCode, "msg", msg, "biz_msg", bizMsg, "use_config_token", a.UseConfigToken, "account", a.AccountID, "target_path", targetPath)
		lastFailureMessage = failureMessage(msg, bizMsg, "get pow failed")
		if isTokenInvalid(status, code, bizCode, msg, bizMsg) || isAuthIndicativeBizFailure(msg, bizMsg) {
			lastFailureKind = authFailureKind(a.UseConfigToken)
		} else {
			lastFailureKind = FailureUnknown
		}
		if a.UseConfigToken {
			if !refreshed && shouldAttemptRefresh(status, code, bizCode, msg, bizMsg) {
				if c.Auth.RefreshToken(ctx, a) {
					refreshed = true
					continue
				}
			}
			if c.Auth.SwitchAccount(ctx, a) {
				refreshed = false
				attempts++
				continue
			}
		}
		attempts++
	}
	if lastFailureKind != FailureUnknown {
		return "", &RequestFailure{Op: "get pow", Kind: lastFailureKind, Message: lastFailureMessage}
	}
	return "", errors.New("get pow failed")
}

func (c *Client) authHeaders(token string, a *auth.RequestAuth) map[string]string {
	headers := make(map[string]string, len(dsprotocol.BaseHeaders)+3)
	for k, v := range dsprotocol.BaseHeaders {
		headers[k] = v
	}
	headers["authorization"] = "Bearer " + token
	if a != nil && a.Account.RangersID != "" {
		headers["x-rangers-id"] = a.Account.RangersID
	}
	return headers
}

func generateRangersID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return strconv.FormatUint(binary.BigEndian.Uint64(b[:]), 10)
}

func isTokenInvalid(status int, code int, bizCode int, msg string, bizMsg string) bool {
	msg = strings.ToLower(strings.TrimSpace(msg) + " " + strings.TrimSpace(bizMsg))
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return true
	}
	if code == 40001 || code == 40002 || code == 40003 || bizCode == 40001 || bizCode == 40002 || bizCode == 40003 {
		return true
	}
	return strings.Contains(msg, "token") ||
		strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "expired") ||
		strings.Contains(msg, "not login") ||
		strings.Contains(msg, "login required") ||
		strings.Contains(msg, "invalid jwt")
}

func shouldAttemptRefresh(status int, code int, bizCode int, msg string, bizMsg string) bool {
	if isTokenInvalid(status, code, bizCode, msg, bizMsg) {
		return true
	}
	// Some DeepSeek failures come back as HTTP 200/code=0 but with non-zero biz_code.
	// Only attempt refresh when these biz failures still look auth-related.
	return status == http.StatusOK &&
		code == 0 &&
		bizCode != 0 &&
		isAuthIndicativeBizFailure(msg, bizMsg)
}

func isAuthIndicativeBizFailure(msg string, bizMsg string) bool {
	combined := strings.ToLower(strings.TrimSpace(msg) + " " + strings.TrimSpace(bizMsg))
	authKeywords := []string{
		"auth",
		"authorization",
		"credential",
		"expired",
		"invalid jwt",
		"jwt",
		"login",
		"not login",
		"session expired",
		"token",
		"unauthorized",
		"登录",
		"未登录",
		"认证",
		"凭证",
		"会话过期",
		"令牌",
	}
	for _, keyword := range authKeywords {
		if strings.Contains(combined, keyword) {
			return true
		}
	}
	return false
}

// isBannedError checks whether the login response indicates the account is banned.
func isBannedError(msg string) bool {
	combined := strings.ToLower(strings.TrimSpace(msg))
	bannedKeywords := []string{
		"user_is_banned",
		"user is banned",
		"banned",
		"封禁",
		"被封禁",
		"账号已封禁",
	}
	for _, keyword := range bannedKeywords {
		if strings.Contains(combined, keyword) {
			return true
		}
	}
	return false
}

func authFailureKind(useConfigToken bool) FailureKind {
	if useConfigToken {
		return FailureManagedUnauthorized
	}
	return FailureDirectUnauthorized
}

func failureMessage(msg string, bizMsg string, fallback string) string {
	if trimmed := strings.TrimSpace(bizMsg); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(msg); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}

// DeepSeek has returned create-session ids in both biz_data.id and
// biz_data.chat_session.id across observed response variants; accept either.
func extractCreateSessionID(resp map[string]any) string {
	data, _ := resp["data"].(map[string]any)
	bizData, _ := data["biz_data"].(map[string]any)
	if sessionID, _ := bizData["id"].(string); strings.TrimSpace(sessionID) != "" {
		return strings.TrimSpace(sessionID)
	}
	if chatSession, ok := bizData["chat_session"].(map[string]any); ok {
		if sessionID, _ := chatSession["id"].(string); strings.TrimSpace(sessionID) != "" {
			return strings.TrimSpace(sessionID)
		}
	}
	return ""
}

func extractResponseStatus(resp map[string]any) (code int, bizCode int, msg string, bizMsg string) {
	code = intFrom(resp["code"])
	msg, _ = resp["msg"].(string)
	data, _ := resp["data"].(map[string]any)
	bizCode = intFrom(data["biz_code"])
	bizMsg, _ = data["biz_msg"].(string)
	if strings.TrimSpace(bizMsg) == "" {
		if bizData, ok := data["biz_data"].(map[string]any); ok {
			bizMsg, _ = bizData["msg"].(string)
		}
	}
	return code, bizCode, msg, bizMsg
}

func normalizeMobileForLogin(raw string) (mobile string, areaCode any) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	hasPlus := strings.HasPrefix(s, "+")
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return "", nil
	}
	if (hasPlus || strings.HasPrefix(digits, "86")) && strings.HasPrefix(digits, "86") && len(digits) == 13 {
		return digits[2:], nil
	}
	return digits, nil
}
