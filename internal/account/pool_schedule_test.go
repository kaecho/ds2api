package account

import (
	"testing"

	"ds2api/internal/config"
)

func TestPoolFillSticksUntilDailyLimit(t *testing.T) {
	pool := newPoolForTest(t, "4")
	pool.ApplySchedule(config.ScheduleFill, 2)

	first, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected first acquire")
	}
	second, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected second acquire")
	}
	if first.Identifier() != second.Identifier() {
		t.Fatalf("fill should stick, got %q then %q", first.Identifier(), second.Identifier())
	}
	pool.Release(first.Identifier())
	pool.Release(second.Identifier())

	third, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected third acquire after cap")
	}
	if third.Identifier() == first.Identifier() {
		t.Fatalf("expected next account after daily cap, still %q", third.Identifier())
	}
}

func TestPoolFillWaitsOnBusyStickyAccount(t *testing.T) {
	pool := newSingleAccountPoolForTest(t, "1")
	pool.ApplySchedule(config.ScheduleFill, 10)
	first, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected first acquire")
	}
	if _, ok := pool.Acquire("", nil); ok {
		t.Fatal("expected second acquire to fail while sticky account is busy")
	}
	pool.Release(first.Identifier())
	second, ok := pool.Acquire("", nil)
	if !ok || second.Identifier() != first.Identifier() {
		t.Fatalf("expected same sticky account after release, ok=%v id=%q", ok, second.Identifier())
	}
}

func TestPoolLeastUsedPrefersLowerCount(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.ApplySchedule(config.ScheduleLeastUsed, 0)

	first, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected first acquire")
	}
	pool.Release(first.Identifier())

	second, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected second acquire")
	}
	if second.Identifier() == first.Identifier() {
		t.Fatalf("least_used should pick the unused account, got %q twice", second.Identifier())
	}
}

func TestPoolRoundRobinSkipsDailyCappedAccount(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.ApplySchedule(config.ScheduleRoundRobin, 1)

	first, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected first acquire")
	}
	pool.Release(first.Identifier())

	second, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected second acquire")
	}
	if second.Identifier() == first.Identifier() {
		t.Fatalf("capped account should be skipped, still %q", second.Identifier())
	}
}

func TestPoolRandomPicksEligibleAccounts(t *testing.T) {
	pool := newPoolForTest(t, "4")
	pool.ApplySchedule(config.ScheduleRandom, 0)
	seen := map[string]int{}
	var held []string
	for range 20 {
		acc, ok := pool.Acquire("", nil)
		if !ok {
			break
		}
		seen[acc.Identifier()]++
		held = append(held, acc.Identifier())
	}
	for _, id := range held {
		pool.Release(id)
	}
	if len(seen) == 0 {
		t.Fatal("expected random acquires")
	}
	for id := range seen {
		if id != "acc1@example.com" && id != "acc2@example.com" {
			t.Fatalf("unexpected account %q", id)
		}
	}
}
