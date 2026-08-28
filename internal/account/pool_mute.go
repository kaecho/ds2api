package account

import (
	"context"
	"time"

	"ds2api/internal/config"
)

const muteExpiryInterval = 5 * time.Second

func (p *Pool) accountUnavailableLocked(accountID string) bool {
	if p.store == nil || accountID == "" {
		return false
	}
	return p.store.AccountBannedStatus(accountID) || p.store.AccountMuted(accountID)
}

func (p *Pool) hasUsableAccountLocked(target string, exclude map[string]bool) bool {
	if target != "" {
		if exclude[target] || p.accountUnavailableLocked(target) || p.accountCappedLocked(target) {
			return false
		}
		_, ok := p.store.FindAccount(target)
		return ok
	}
	for _, id := range p.queue {
		if exclude[id] || p.accountUnavailableLocked(id) || p.accountCappedLocked(id) {
			continue
		}
		return true
	}
	return false
}

// WakeWaiters unblocks queued acquires so they can skip newly paused accounts
// or pick up accounts whose mute cooldown just ended.
func (p *Pool) WakeWaiters() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.drainWaitersLocked()
}

// StartMuteExpiryLoop re-enables accounts when mute_until has passed.
func (p *Pool) StartMuteExpiryLoop(ctx context.Context) {
	if p == nil || p.store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	go p.muteExpiryLoop(ctx)
}

func (p *Pool) muteExpiryLoop(ctx context.Context) {
	ticker := time.NewTicker(muteExpiryInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids := p.store.ClearExpiredMutes(time.Now().Unix())
			if len(ids) == 0 {
				continue
			}
			config.Logger.Info("[account_mute] cooldown ended, accounts re-enabled", "count", len(ids), "accounts", ids)
			p.WakeWaiters()
		}
	}
}
