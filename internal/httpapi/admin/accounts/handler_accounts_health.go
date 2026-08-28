package accounts

import (
	"context"
	"fmt"
	"net/http"
	"time"

	authn "ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
)

func (h *Handler) checkAllAccountStatus(w http.ResponseWriter, r *http.Request) {
	accounts := h.Store.Snapshot().Accounts
	if len(accounts) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"total": 0, "healthy": 0, "muted": 0, "banned": 0, "failed": 0, "results": []any{},
		})
		return
	}
	const maxConcurrency = 5
	results := runAccountTestsConcurrently(accounts, maxConcurrency, func(_ int, account config.Account) map[string]any {
		return h.checkAccountStatus(r.Context(), account)
	})
	healthy, muted, banned, failed := 0, 0, 0, 0
	for _, res := range results {
		switch healthOf(res) {
		case "healthy":
			healthy++
		case "muted":
			muted++
		case "banned":
			banned++
		default:
			failed++
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":   len(accounts),
		"healthy": healthy,
		"muted":   muted,
		"banned":  banned,
		"failed":  failed,
		"results": results,
	})
}

func (h *Handler) checkAccountStatus(ctx context.Context, acc config.Account) map[string]any {
	start := time.Now()
	identifier := acc.Identifier()
	result := map[string]any{
		"account":       identifier,
		"success":       false,
		"health":        "failed",
		"message":       "",
		"mute_until":    0,
		"response_time": 0,
	}
	token, err := h.DS.Login(ctx, acc)
	if err != nil {
		if dsclient.IsBannedError(err) {
			h.markAccountBanned(identifier)
			result["health"] = "banned"
			result["success"] = true
			result["message"] = "永久封禁: " + err.Error()
			result["response_time"] = int(time.Since(start).Milliseconds())
			return result
		}
		result["message"] = "登录失败: " + err.Error()
		result["response_time"] = int(time.Since(start).Milliseconds())
		return result
	}
	if err := h.Store.UpdateAccountToken(identifier, token); err != nil {
		result["config_warning"] = "登录成功，但 token 持久化失败（仅保存在内存，重启后会丢失）: " + err.Error()
	}
	authCtx := &authn.RequestAuth{UseConfigToken: false, DeepSeekToken: token, AccountID: identifier, Account: acc}
	proxyCtx := authn.WithAuth(ctx, authCtx)
	health, until, msg, _ := h.inspectLoggedInAccount(proxyCtx, identifier, token)
	result["health"] = health
	result["message"] = msg
	if until > 0 {
		result["mute_until"] = until
	}
	if health == "healthy" {
		result["success"] = true
		_ = h.Store.UpdateAccountTestStatus(identifier, "ok")
	}
	if health == "muted" || health == "banned" {
		result["success"] = true
	}
	if warning, _ := result["config_warning"].(string); warning != "" {
		result["message"] = result["message"].(string) + "；" + warning
	}
	result["response_time"] = int(time.Since(start).Milliseconds())
	return result
}

func (h *Handler) inspectLoggedInAccount(ctx context.Context, identifier, token string) (health string, muteUntil int64, message string, stop bool) {
	user, err := h.DS.GetCurrentUser(ctx, token)
	if err != nil {
		if dsclient.IsBannedError(err) {
			h.markAccountBanned(identifier)
			return "banned", 0, "永久封禁: " + err.Error(), true
		}
		return "failed", 0, "查询账号状态失败: " + err.Error(), false
	}
	health, muteUntil = h.applyCurrentUserHealth(identifier, user)
	if health == "muted" {
		return health, muteUntil, muteMessage(muteUntil), true
	}
	return "healthy", 0, "账号健康", false
}

func (h *Handler) applyCurrentUserHealth(identifier string, user *dsclient.CurrentUser) (health string, muteUntil int64) {
	now := time.Now().Unix()
	if user != nil && user.IsMuted {
		until := user.MuteUntilUnix
		if until > now {
			h.Store.UpdateAccountMuteUntil(identifier, until)
			muteUntil = until
		} else {
			h.Store.UpdateAccountMuteUntil(identifier, -1)
		}
		h.Store.UpdateAccountBannedStatus(identifier, false)
		_ = h.Store.UpdateAccountTestStatus(identifier, "muted")
		h.wakePool()
		return "muted", muteUntil
	}
	wasMuted := h.Store.AccountMuted(identifier)
	wasBanned := h.Store.AccountBannedStatus(identifier)
	h.Store.UpdateAccountMuteUntil(identifier, 0)
	h.Store.UpdateAccountBannedStatus(identifier, false)
	if wasMuted || wasBanned {
		h.wakePool()
	}
	return "healthy", 0
}

func (h *Handler) markAccountBanned(identifier string) {
	h.Store.UpdateAccountBannedStatus(identifier, true)
	h.Store.UpdateAccountMuteUntil(identifier, 0)
	_ = h.Store.UpdateAccountTestStatus(identifier, "banned")
	h.wakePool()
}

func (h *Handler) wakePool() {
	if h.Pool == nil {
		return
	}
	h.Pool.WakeWaiters()
}

func muteMessage(until int64) string {
	if until > 0 {
		return fmt.Sprintf("短暂封禁，已暂停调度，冷却至 %s", time.Unix(until, 0).Local().Format("2006-01-02 15:04:05"))
	}
	return "短暂封禁，已暂停调度"
}

func healthOf(res map[string]any) string {
	health, _ := res["health"].(string)
	return health
}

func accountListHealth(store interface {
	AccountBannedStatus(string) bool
	AccountMuteUntil(string) int64
	AccountTestStatus(string) (string, bool)
}, identifier, testStatus string) (health string, muted bool, muteUntil int64) {
	if store.AccountBannedStatus(identifier) || testStatus == "banned" {
		return "banned", false, 0
	}
	muteUntil = store.AccountMuteUntil(identifier)
	if muteUntil != 0 {
		if muteUntil < 0 {
			muteUntil = 0
		}
		return "muted", true, muteUntil
	}
	switch testStatus {
	case "ok":
		return "healthy", false, 0
	case "muted":
		return "muted", true, 0
	case "failed":
		return "failed", false, 0
	default:
		return "", false, 0
	}
}
