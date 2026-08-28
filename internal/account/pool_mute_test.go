package account

import (
	"testing"
	"time"
)

func TestPoolSkipsMutedAccount(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.store.UpdateAccountMuteUntil("acc1@example.com", time.Now().Unix()+3600)

	acc, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected acquire to skip muted acc1")
	}
	if acc.Identifier() != "acc2@example.com" {
		t.Fatalf("expected acc2, got %q", acc.Identifier())
	}
}

func TestPoolSkipsBannedAccount(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.store.UpdateAccountBannedStatus("acc1@example.com", true)

	acc, ok := pool.Acquire("", nil)
	if !ok {
		t.Fatal("expected acquire to skip banned acc1")
	}
	if acc.Identifier() != "acc2@example.com" {
		t.Fatalf("expected acc2, got %q", acc.Identifier())
	}
}

func TestPoolAcquireFailsWhenAllPaused(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.store.UpdateAccountBannedStatus("acc1@example.com", true)
	pool.store.UpdateAccountMuteUntil("acc2@example.com", time.Now().Unix()+3600)
	if _, ok := pool.Acquire("", nil); ok {
		t.Fatal("expected acquire to fail when every account is paused")
	}
}

func TestPoolReacquiresAfterMuteExpires(t *testing.T) {
	pool := newSingleAccountPoolForTest(t, "1")
	pool.store.UpdateAccountMuteUntil("acc1@example.com", time.Now().Unix()-5)
	acc, ok := pool.Acquire("", nil)
	if !ok || acc.Identifier() != "acc1@example.com" {
		t.Fatalf("expected acc1 after mute expired, ok=%v id=%q", ok, acc.Identifier())
	}
}

func TestPoolStatusExcludesPausedFromAvailable(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.store.UpdateAccountMuteUntil("acc1@example.com", time.Now().Unix()+3600)
	status := pool.Status()
	available, _ := status["available"].(int)
	if available != 1 {
		t.Fatalf("expected available=1, got %#v", status["available"])
	}
	ids, _ := status["available_accounts"].([]string)
	if len(ids) != 1 || ids[0] != "acc2@example.com" {
		t.Fatalf("expected only acc2 available, got %#v", ids)
	}
}

func TestPoolTargetMutedIsNotAcquired(t *testing.T) {
	pool := newPoolForTest(t, "2")
	pool.store.UpdateAccountMuteUntil("acc1@example.com", time.Now().Unix()+3600)
	if _, ok := pool.Acquire("acc1@example.com", nil); ok {
		t.Fatal("expected targeted muted account acquire to fail")
	}
}
