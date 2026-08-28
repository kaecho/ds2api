package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	dsprotocol "ds2api/internal/deepseek/protocol"
)

func TestParseCurrentUserMutedShape(t *testing.T) {
	user := parseCurrentUser(map[string]any{
		"code": 0,
		"data": map[string]any{
			"biz_code": 0,
			"biz_data": map[string]any{
				"id":     "user-1",
				"email":  "u****@example.com",
				"status": 0,
				"chat": map[string]any{
					"is_muted":   float64(1),
					"mute_until": 1788144318.623,
				},
			},
		},
	})
	if user == nil {
		t.Fatal("expected parsed user")
	}
	if !user.IsMuted {
		t.Fatal("expected is_muted")
	}
	if user.MuteUntilUnix != 1788144318 {
		t.Fatalf("mute_until=%d", user.MuteUntilUnix)
	}
	if user.ID != "user-1" {
		t.Fatalf("id=%q", user.ID)
	}
}

func TestParseCurrentUserHealthyWhenChatNotMuted(t *testing.T) {
	user := parseCurrentUser(map[string]any{
		"data": map[string]any{
			"biz_data": map[string]any{
				"id": "user-2",
				"chat": map[string]any{
					"is_muted":   float64(0),
					"mute_until": float64(0),
				},
			},
		},
	})
	if user == nil || user.IsMuted || user.MuteUntilUnix != 0 {
		t.Fatalf("expected healthy user, got %#v", user)
	}
}

func TestGetCurrentUserUsesBearerToken(t *testing.T) {
	var gotAuth, gotURL string
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			gotAuth = req.Header.Get("authorization")
			gotURL = req.URL.String()
			body := `{"code":0,"data":{"biz_code":0,"biz_data":{"id":"u1","chat":{"is_muted":1,"mute_until":1788144318}}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
	}
	user, err := client.GetCurrentUser(context.Background(), "tok-1")
	if err != nil {
		t.Fatalf("GetCurrentUser: %v", err)
	}
	if gotAuth != "Bearer tok-1" {
		t.Fatalf("authorization=%q", gotAuth)
	}
	if gotURL != dsprotocol.DeepSeekCurrentUserURL {
		t.Fatalf("url=%q", gotURL)
	}
	if user == nil || !user.IsMuted || user.MuteUntilUnix != 1788144318 {
		t.Fatalf("unexpected user %#v", user)
	}
}

func TestGetCurrentUserBannedBizMsg(t *testing.T) {
	client := &Client{
		regular: doerFunc(func(req *http.Request) (*http.Response, error) {
			body := `{"code":0,"data":{"biz_code":1,"biz_msg":"user_is_banned"}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    req,
			}, nil
		}),
	}
	_, err := client.GetCurrentUser(context.Background(), "tok-1")
	if !IsBannedError(err) {
		t.Fatalf("expected banned error, got %v", err)
	}
}
