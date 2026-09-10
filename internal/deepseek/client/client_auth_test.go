package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/config"
	dsprotocol "ds2api/internal/deepseek/protocol"
)

func TestExtractCreateSessionIDSupportsLegacyShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "legacy-session-id",
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "legacy-session-id" {
		t.Fatalf("expected legacy session id, got %q", got)
	}
}

func TestExtractCreateSessionIDSupportsNestedChatSessionShape(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"chat_session": map[string]any{
					"id":         "nested-session-id",
					"model_type": "default",
				},
			},
		},
	}

	if got := extractCreateSessionID(resp); got != "nested-session-id" {
		t.Fatalf("expected nested session id, got %q", got)
	}
}

func TestLoginSendsNativeClientIdentityHeaders(t *testing.T) {
	var (
		seenURL     string
		seenHeaders http.Header
		seenBody    map[string]any
	)
	primary := doerFunc(func(req *http.Request) (*http.Response, error) {
		seenURL = req.URL.String()
		seenHeaders = req.Header.Clone()
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read login body: %v", err)
		}
		if err := json.Unmarshal(raw, &seenBody); err != nil {
			t.Fatalf("decode login body: %v", err)
		}
		body := `{"code":0,"data":{"biz_code":0,"biz_data":{"user":{"token":"login-token"}}}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    req,
		}, nil
	})
	c := &Client{
		regular: primary,
	}
	token, err := c.Login(context.Background(), config.Account{
		Email:     "user@example.com",
		Password:  "secret",
		DeviceID:  "BNlM4B07tkAd9/BDOrew4lrBnS6RPQZfDZczOl8c94iSKWdOj9pN3BGLJEkfd0a0FKg7oO0/65VIvtFLf4O8X1A==",
		RangersID: "rangers-1",
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token != "login-token" {
		t.Fatalf("token=%q", token)
	}
	if seenURL != dsprotocol.DeepSeekLoginURL {
		t.Fatalf("url=%q", seenURL)
	}
	if got := seenHeaders.Get("x-rangers-id"); got != "rangers-1" {
		t.Fatalf("x-rangers-id=%q", got)
	}
	if got := seenHeaders.Get("x-client-bundle-id"); got != "com.deepseek.chat" {
		t.Fatalf("x-client-bundle-id=%q", got)
	}
	if got := seenHeaders.Get("x-client-timezone-offset"); got != "28800" {
		t.Fatalf("x-client-timezone-offset=%q", got)
	}
	if got := seenHeaders.Get("x-client-version"); got != "2.4.5" {
		t.Fatalf("x-client-version=%q", got)
	}
	if got := seenHeaders.Get("User-Agent"); got != "DeepSeek/2.4.5 Android/35" {
		t.Fatalf("User-Agent=%q", got)
	}
	if got := seenHeaders.Get("accept-charset"); got != "" {
		t.Fatalf("accept-charset=%q", got)
	}
	if got := seenBody["os"]; got != "android" {
		t.Fatalf("os=%#v", got)
	}
	if got := seenBody["device_id"]; got != "BNlM4B07tkAd9/BDOrew4lrBnS6RPQZfDZczOl8c94iSKWdOj9pN3BGLJEkfd0a0FKg7oO0/65VIvtFLf4O8X1A==" {
		t.Fatalf("device_id=%#v", got)
	}
}

func TestLoginReplacesUUIDWithShumeiDeviceID(t *testing.T) {
	old := generateShumeiDeviceID
	t.Cleanup(func() { generateShumeiDeviceID = old })
	want := "Baaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa=="
	generateShumeiDeviceID = func(context.Context) (string, error) {
		return want, nil
	}
	var seen any
	c := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			raw, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatalf("read login body: %v", err)
			}
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Fatalf("decode login body: %v", err)
			}
			seen = body["device_id"]
			resp := `{"code":0,"data":{"biz_code":0,"biz_data":{"user":{"token":"t"}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(resp)),
				Request:    req,
			}, nil
		}),
	}
	if _, err := c.Login(context.Background(), config.Account{
		Email:     "user@example.com",
		Password:  "secret",
		DeviceID:  "7d2a1c4e-9b50-4a6f-8e21-3c9f0a17b6d2",
		RangersID: "rangers-1",
	}); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if seen != want {
		t.Fatalf("device_id=%#v want %q", seen, want)
	}
}
