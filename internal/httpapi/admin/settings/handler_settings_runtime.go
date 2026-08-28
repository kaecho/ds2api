package settings

import "ds2api/internal/config"

func validateMergedRuntimeSettings(current config.RuntimeConfig, incoming *config.RuntimeConfig) error {
	merged := current
	if incoming != nil {
		if incoming.AccountMaxInflight > 0 {
			merged.AccountMaxInflight = incoming.AccountMaxInflight
		}
		if incoming.AccountMaxQueue > 0 {
			merged.AccountMaxQueue = incoming.AccountMaxQueue
		}
		if incoming.GlobalMaxInflight > 0 {
			merged.GlobalMaxInflight = incoming.GlobalMaxInflight
		}
		if incoming.TokenRefreshIntervalHours > 0 {
			merged.TokenRefreshIntervalHours = incoming.TokenRefreshIntervalHours
		}
		if incoming.AccountSchedule != "" {
			merged.AccountSchedule = incoming.AccountSchedule
		}
		if incoming.AccountDailyLimit != nil {
			merged.AccountDailyLimit = incoming.AccountDailyLimit
		}
	}
	return validateRuntimeSettings(merged)
}

func (h *Handler) applyRuntimeSettings() {
	if h == nil || h.Store == nil || h.Pool == nil {
		return
	}
	accountCount := len(h.Store.Accounts())
	maxPer := h.Store.RuntimeAccountMaxInflight()
	recommended := defaultRuntimeRecommended(accountCount, maxPer)
	maxQueue := h.Store.RuntimeAccountMaxQueue(recommended)
	global := h.Store.RuntimeGlobalMaxInflight(recommended)
	h.Pool.ApplyRuntimeLimits(maxPer, maxQueue, global)
	h.Pool.ApplySchedule(h.Store.RuntimeAccountSchedule(), h.Store.RuntimeAccountDailyLimit())
}

func defaultRuntimeRecommended(accountCount, maxPer int) int {
	if maxPer <= 0 {
		maxPer = 1
	}
	if accountCount <= 0 {
		return maxPer
	}
	return accountCount * maxPer
}
