package config

import (
	"testing"
	"time"
)

func TestAccountMuteUntilExpiresAndClears(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"p"}]}`)
	store := LoadStore()

	store.UpdateAccountMuteUntil("u@example.com", time.Now().Unix()+3600)
	if !store.AccountMuted("u@example.com") {
		t.Fatal("expected account to be muted")
	}
	if got := store.AccountMuteUntil("u@example.com"); got <= time.Now().Unix() {
		t.Fatalf("expected future mute deadline, got %d", got)
	}

	store.UpdateAccountMuteUntil("u@example.com", time.Now().Unix()-5)
	if store.AccountMuted("u@example.com") {
		t.Fatal("expected expired mute to clear")
	}
	if got := store.AccountMuteUntil("u@example.com"); got != 0 {
		t.Fatalf("expected cleared mute until, got %d", got)
	}
}

func TestClearExpiredMutesLeavesUnknownEndAndFuture(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[
		{"email":"a@example.com","password":"p"},
		{"email":"b@example.com","password":"p"},
		{"email":"c@example.com","password":"p"}
	]}`)
	store := LoadStore()
	now := time.Now().Unix()
	store.UpdateAccountMuteUntil("a@example.com", now-10)
	store.UpdateAccountMuteUntil("b@example.com", now+3600)
	store.UpdateAccountMuteUntil("c@example.com", -1)

	expired := store.ClearExpiredMutes(now)
	if len(expired) != 1 || expired[0] != "a@example.com" {
		t.Fatalf("expected only a@example.com expired, got %#v", expired)
	}
	if store.AccountMuted("a@example.com") {
		t.Fatal("expected a to be unmuted")
	}
	if !store.AccountMuted("b@example.com") || !store.AccountMuted("c@example.com") {
		t.Fatal("expected b and c to stay muted")
	}
}

func TestUpdateAccountMuteUntilZeroClears(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"p"}]}`)
	store := LoadStore()
	store.UpdateAccountMuteUntil("u@example.com", -1)
	if !store.AccountMuted("u@example.com") {
		t.Fatal("expected unknown-end mute")
	}
	store.UpdateAccountMuteUntil("u@example.com", 0)
	if store.AccountMuted("u@example.com") {
		t.Fatal("expected mute cleared")
	}
}
