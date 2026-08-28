package config

import (
	"os"
	"strings"
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

func TestAccountHealthPersistsAcrossReload(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(`{"accounts":[{"email":"u@example.com","password":"p"}]}`); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp config: %v", err)
	}

	t.Setenv("DS2API_CONFIG_JSON", "")
	t.Setenv("DS2API_CONFIG_PATH", path)

	store := LoadStore()
	until := time.Now().Add(2 * time.Hour).Unix()
	if err := store.UpdateAccountHealth("u@example.com", false, until, "muted"); err != nil {
		t.Fatalf("update health: %v", err)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	raw := string(content)
	if !strings.Contains(raw, `"test_status"`) || !strings.Contains(raw, `"muted"`) {
		t.Fatalf("expected test_status muted in config, got %s", raw)
	}
	if !strings.Contains(raw, `"mute_until"`) {
		t.Fatalf("expected mute_until in config, got %s", raw)
	}

	reloaded := LoadStore()
	if !reloaded.AccountMuted("u@example.com") {
		t.Fatal("expected mute to survive reload")
	}
	if got, ok := reloaded.AccountTestStatus("u@example.com"); !ok || got != "muted" {
		t.Fatalf("expected muted test status after reload, got %q ok=%v", got, ok)
	}
	acc, ok := reloaded.FindAccount("u@example.com")
	if !ok || acc.MuteUntil != until {
		t.Fatalf("expected mute_until %d after reload, got ok=%v until=%d", until, ok, acc.MuteUntil)
	}
}

func TestAccountBannedPersistsAcrossReload(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "config-*.json")
	if err != nil {
		t.Fatalf("create temp config: %v", err)
	}
	path := tmp.Name()
	if _, err := tmp.WriteString(`{"accounts":[{"email":"u@example.com","password":"p"}]}`); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatalf("close temp config: %v", err)
	}

	t.Setenv("DS2API_CONFIG_JSON", "")
	t.Setenv("DS2API_CONFIG_PATH", path)

	store := LoadStore()
	store.UpdateAccountBannedStatus("u@example.com", true)
	if err := store.UpdateAccountTestStatus("u@example.com", "banned"); err != nil {
		t.Fatalf("update test status: %v", err)
	}

	reloaded := LoadStore()
	if !reloaded.AccountBannedStatus("u@example.com") {
		t.Fatal("expected banned status to survive reload")
	}
	if got, ok := reloaded.AccountTestStatus("u@example.com"); !ok || got != "banned" {
		t.Fatalf("expected banned test status after reload, got %q ok=%v", got, ok)
	}
}
