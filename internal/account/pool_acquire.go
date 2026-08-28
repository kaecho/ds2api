package account

import (
	"context"

	"ds2api/internal/config"
)

func (p *Pool) Acquire(target string, exclude map[string]bool) (config.Account, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.acquireLocked(target, normalizeExclude(exclude))
}

func (p *Pool) AcquireWait(ctx context.Context, target string, exclude map[string]bool) (config.Account, bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	exclude = normalizeExclude(exclude)
	for {
		if ctx.Err() != nil {
			return config.Account{}, false
		}

		p.mu.Lock()
		if acc, ok := p.acquireLocked(target, exclude); ok {
			p.mu.Unlock()
			return acc, true
		}
		if !p.canQueueLocked(target, exclude) {
			p.mu.Unlock()
			return config.Account{}, false
		}
		waiter := make(chan struct{})
		p.waiters = append(p.waiters, waiter)
		p.mu.Unlock()

		select {
		case <-ctx.Done():
			p.mu.Lock()
			p.removeWaiterLocked(waiter)
			p.mu.Unlock()
			return config.Account{}, false
		case <-waiter:
		}
	}
}

func (p *Pool) acquireLocked(target string, exclude map[string]bool) (config.Account, bool) {
	if target != "" {
		if exclude[target] || !p.canAcquireIDLocked(target) {
			return config.Account{}, false
		}
		acc, ok := p.store.FindAccount(target)
		if !ok {
			return config.Account{}, false
		}
		p.inUse[target]++
		p.bumpQueue(target)
		return acc, true
	}

	return p.tryAcquire(exclude)
}

func (p *Pool) tryAcquire(exclude map[string]bool) (config.Account, bool) {
	switch p.schedule {
	case config.ScheduleFill:
		return p.tryAcquireFill(exclude)
	case config.ScheduleLeastUsed:
		return p.tryAcquireLeastUsed(exclude)
	case config.ScheduleRandom:
		return p.tryAcquireRandom(exclude)
	default:
		return p.tryAcquireRoundRobin(exclude)
	}
}

func (p *Pool) tryAcquireRoundRobin(exclude map[string]bool) (config.Account, bool) {
	for i := range p.queue {
		id := p.queue[i]
		if exclude[id] || !p.canAcquireIDLocked(id) {
			continue
		}
		return p.takeLocked(id)
	}
	return config.Account{}, false
}

func (p *Pool) takeLocked(accountID string) (config.Account, bool) {
	acc, ok := p.store.FindAccount(accountID)
	if !ok {
		return config.Account{}, false
	}
	p.inUse[accountID]++
	if p.schedule == config.ScheduleRoundRobin || p.schedule == "" {
		p.bumpQueue(accountID)
	}
	if p.store != nil {
		p.store.NoteAccountUse(accountID)
	}
	return acc, true
}

func (p *Pool) bumpQueue(accountID string) {
	for i, id := range p.queue {
		if id != accountID {
			continue
		}
		p.queue = append(p.queue[:i], p.queue[i+1:]...)
		p.queue = append(p.queue, accountID)
		return
	}
}

func normalizeExclude(exclude map[string]bool) map[string]bool {
	if exclude == nil {
		return map[string]bool{}
	}
	return exclude
}
