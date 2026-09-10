package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestPostJSONWithStatusUsesProvidedFallbackClient(t *testing.T) {
	var fallbackCalled bool
	client := &Client{}
	primary := failingDoer{err: errors.New("primary failed")}
	fallbackDoer := doerFunc(func(req *http.Request) (*http.Response, error) {
		fallbackCalled = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    req,
		}, nil
	})

	resp, status, err := client.postJSONWithStatus(
		context.Background(),
		primary,
		fallbackDoer,
		"https://example.com/api",
		map[string]string{"x-test": "1"},
		map[string]any{"foo": "bar"},
	)
	if err != nil {
		t.Fatalf("postJSONWithStatus error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("status=%d want=%d", status, http.StatusOK)
	}
	if !fallbackCalled {
		t.Fatal("expected provided fallback doer to be called")
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("unexpected response body: %#v", resp)
	}
}

func TestPostJSONWithStatusMarshalsNilPayloadAsEmptyObject(t *testing.T) {
	var seenBody string
	client := &Client{}
	primary := doerFunc(func(req *http.Request) (*http.Response, error) {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		seenBody = string(raw)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
			Request:    req,
		}, nil
	})
	_, _, err := client.postJSONWithStatus(
		context.Background(),
		primary,
		failingDoer{err: errors.New("unused")},
		"https://example.com/api",
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("postJSONWithStatus error: %v", err)
	}
	if seenBody != "{}" {
		t.Fatalf("body=%q want={}", seenBody)
	}
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}
