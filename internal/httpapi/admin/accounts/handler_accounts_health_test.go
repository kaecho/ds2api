package accounts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ds2api/internal/account"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
)

func TestCheckAccountStatusPausesMutedAccount(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"pwd"}]}`)
	store := config.LoadStore()
	pool := account.NewPool(store)
	until := time.Now().Add(time.Hour).Unix()
	ds := &testingDSMock{currentUser: &dsclient.CurrentUser{IsMuted: true, MuteUntilUnix: until}}
	h := &Handler{Store: store, Pool: pool, DS: ds}
	acc, ok := store.FindAccount("u@example.com")
	if !ok {
		t.Fatal("expected account")
	}

	result := h.checkAccountStatus(context.Background(), acc)
	if healthOf(result) != "muted" {
		t.Fatalf("expected muted, got %#v", result)
	}
	if ds.createSessionCalls != 0 {
		t.Fatalf("muted check should not create a session, got %d", ds.createSessionCalls)
	}
	if !store.AccountMuted("u@example.com") {
		t.Fatal("expected account to be paused")
	}
	if _, ok := pool.Acquire("u@example.com", nil); ok {
		t.Fatal("muted account should not be acquired")
	}
}

func TestCheckAccountStatusMarksPermanentBan(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"pwd"}]}`)
	store := config.LoadStore()
	pool := account.NewPool(store)
	ds := &testingDSMock{loginErr: &dsclient.RequestFailure{Op: "login", Kind: dsclient.FailureBanned, Message: "user_is_banned"}}
	h := &Handler{Store: store, Pool: pool, DS: ds}
	acc, ok := store.FindAccount("u@example.com")
	if !ok {
		t.Fatal("expected account")
	}

	result := h.checkAccountStatus(context.Background(), acc)
	if healthOf(result) != "banned" {
		t.Fatalf("expected banned, got %#v", result)
	}
	if !store.AccountBannedStatus("u@example.com") {
		t.Fatal("expected banned flag")
	}
	if _, ok := pool.Acquire("u@example.com", nil); ok {
		t.Fatal("banned account should not be acquired")
	}
}

func TestCheckAccountStatusClearsMuteWhenHealthy(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"pwd"}]}`)
	store := config.LoadStore()
	store.UpdateAccountMuteUntil("u@example.com", time.Now().Add(time.Hour).Unix())
	ds := &testingDSMock{}
	h := &Handler{Store: store, Pool: account.NewPool(store), DS: ds}
	acc, _ := store.FindAccount("u@example.com")

	result := h.checkAccountStatus(context.Background(), acc)
	if healthOf(result) != "healthy" {
		t.Fatalf("expected healthy, got %#v", result)
	}
	if store.AccountMuted("u@example.com") {
		t.Fatal("expected mute cleared")
	}
}

func TestTestAccountSkipsSessionWhenMuted(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"pwd"}]}`)
	store := config.LoadStore()
	until := time.Now().Add(2 * time.Hour).Unix()
	ds := &testingDSMock{currentUser: &dsclient.CurrentUser{IsMuted: true, MuteUntilUnix: until}}
	h := &Handler{Store: store, Pool: account.NewPool(store), DS: ds}
	acc, _ := store.FindAccount("u@example.com")

	result := h.testAccount(context.Background(), acc, "deepseek-v4-flash", "")
	if healthOf(result) != "muted" {
		t.Fatalf("expected muted, got %#v", result)
	}
	if ok, _ := result["success"].(bool); ok {
		t.Fatal("muted account should not report success")
	}
	if ds.createSessionCalls != 0 {
		t.Fatalf("expected no CreateSession, got %d", ds.createSessionCalls)
	}
	status, _ := store.AccountTestStatus("u@example.com")
	if status != "muted" {
		t.Fatalf("test_status=%q", status)
	}
}

func TestListAccountsIncludesMuteFields(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"accounts":[{"email":"u@example.com","password":"pwd","token":"abcdefgh"}]}`)
	store := config.LoadStore()
	until := time.Now().Add(time.Hour).Unix()
	store.UpdateAccountMuteUntil("u@example.com", until)
	h := &Handler{Store: store}

	req := httptest.NewRequest(http.MethodGet, "/admin/accounts?page=1&page_size=10", nil)
	rec := httptest.NewRecorder()
	h.listAccounts(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, _ := payload["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items=%d", len(items))
	}
	first, _ := items[0].(map[string]any)
	if health, _ := first["health"].(string); health != "muted" {
		t.Fatalf("health=%v", first["health"])
	}
	if muted, _ := first["muted"].(bool); !muted {
		t.Fatal("expected muted=true")
	}
}

func TestCheckAllAccountStatusRoute(t *testing.T) {
	until := time.Now().Add(time.Hour).Unix()
	ds := &testingDSMock{currentUser: &dsclient.CurrentUser{IsMuted: true, MuteUntilUnix: until}}
	router := newHTTPAdminHarness(t, `{"accounts":[{"email":"u@example.com","password":"pwd"}]}`, ds)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, adminReq(http.MethodPost, "/accounts/check-status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if muted, _ := payload["muted"].(float64); muted != 1 {
		t.Fatalf("muted count=%v payload=%#v", payload["muted"], payload)
	}
}
