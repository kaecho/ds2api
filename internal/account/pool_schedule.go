package account

import (
	"math/rand/v2"

	"ds2api/internal/config"
)

func (p *Pool) ApplySchedule(schedule string, dailyLimit int) {
	schedule = config.NormalizeAccountSchedule(schedule)
	if schedule == "" {
		schedule = config.ScheduleRoundRobin
	}
	if dailyLimit < 0 {
		dailyLimit = 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.schedule != schedule {
		p.fillCursor = ""
	}
	p.schedule = schedule
	p.dailyLimit = dailyLimit
	p.notifyWaiterLocked()
}

func (p *Pool) accountCappedLocked(accountID string) bool {
	if p.dailyLimit <= 0 || p.store == nil || accountID == "" {
		return false
	}
	return p.store.AccountDailyUses(accountID) >= p.dailyLimit
}

func (p *Pool) accountSchedulableLocked(accountID string, exclude map[string]bool) bool {
	if accountID == "" || exclude[accountID] {
		return false
	}
	if p.accountUnavailableLocked(accountID) || p.accountCappedLocked(accountID) {
		return false
	}
	if p.store == nil {
		return false
	}
	_, ok := p.store.FindAccount(accountID)
	return ok
}

func (p *Pool) tryAcquireFill(exclude map[string]bool) (config.Account, bool) {
	id := p.fillCursor
	if !p.accountSchedulableLocked(id, exclude) {
		id = p.nextFillLocked(exclude)
		p.fillCursor = id
	}
	if id == "" {
		return config.Account{}, false
	}
	if !p.canAcquireIDLocked(id) {
		return config.Account{}, false
	}
	acc, ok := p.takeLocked(id)
	if !ok {
		return config.Account{}, false
	}
	if p.accountCappedLocked(id) {
		p.fillCursor = ""
	}
	return acc, true
}
func (p *Pool) nextFillLocked(exclude map[string]bool) string {
	start := 0
	if p.fillCursor != "" {
		for i, id := range p.queue {
			if id == p.fillCursor {
				start = i + 1
				break
			}
		}
	}
	n := len(p.queue)
	for i := range n {
		id := p.queue[(start+i)%n]
		if p.accountSchedulableLocked(id, exclude) {
			return id
		}
	}
	return ""
}

func (p *Pool) tryAcquireLeastUsed(exclude map[string]bool) (config.Account, bool) {
	bestID := ""
	bestUses := int(^uint(0) >> 1)
	for _, id := range p.queue {
		if exclude[id] || !p.canAcquireIDLocked(id) {
			continue
		}
		uses := 0
		if p.store != nil {
			uses = p.store.AccountDailyUses(id)
		}
		if bestID == "" || uses < bestUses {
			bestID = id
			bestUses = uses
		}
	}
	if bestID == "" {
		return config.Account{}, false
	}
	return p.takeLocked(bestID)
}

func (p *Pool) tryAcquireRandom(exclude map[string]bool) (config.Account, bool) {
	candidates := make([]string, 0, len(p.queue))
	for _, id := range p.queue {
		if exclude[id] || !p.canAcquireIDLocked(id) {
			continue
		}
		candidates = append(candidates, id)
	}
	if len(candidates) == 0 {
		return config.Account{}, false
	}
	return p.takeLocked(candidates[rand.IntN(len(candidates))])
}
