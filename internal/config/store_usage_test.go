package config

import "testing"

func TestNoteAccountUseIncrements(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"p"}]}`)
	store := LoadStore()
	if store.AccountDailyUses("u@example.com") != 0 {
		t.Fatal("expected zero uses")
	}
	store.NoteAccountUse("u@example.com")
	store.NoteAccountUse("u@example.com")
	if got := store.AccountDailyUses("u@example.com"); got != 2 {
		t.Fatalf("uses=%d", got)
	}
}
