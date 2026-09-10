package smid

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIsValid(t *testing.T) {
	if IsValid("") {
		t.Fatal("empty should be invalid")
	}
	if IsValid("7d2a1c4e-9b50-4a6f-8e21-3c9f0a17b6d2") {
		t.Fatal("uuid should be invalid")
	}
	if IsValid("Bshort") {
		t.Fatal("too-short B prefix should be invalid")
	}
	ok := "BNlM4B07tkAd9/BDOrew4lrBnS6RPQZfDZczOl8c94iSKWdOj9pN3BGLJEkfd0a0FKg7oO0/65VIvtFLf4O8X1A=="
	if !IsValid(ok) {
		t.Fatalf("expected valid smid: %s", ok)
	}
}

func TestDESFieldMatchesJS(t *testing.T) {
	cases := []struct {
		key, value, want string
	}{
		{"kd97xt5z", "default", "Gie7SaV8kEA="},
		{"usosp81m", "web", "6aBkVjOKjTQ="},
		{"wuxhxsj4", "1757531234.123", "UnjvZ3nh0epzvMuJ1vd1ow=="},
		{"huzu3390", "1757531234123", "tHWaQxhZTEdZh7OqVn2lAQ=="},
		{"l3ors8mi", "d41d8cd98f00b204e9800998ecf8427e", "3QokDdrVc17lZOc4cnMWXT1MwSqmmoJQV+oq96fAxGA="},
		{"6210fswr", "480", "fSIz/CL7MXo="},
		{"znkt3gef", "1920_937_1920_1040", "duRRtpeGiybDXdUEvhsSny2cBVp3syFJ"},
	}
	for _, tc := range cases {
		got, err := desField(tc.key, tc.value)
		if err != nil {
			t.Fatalf("desField(%q,%q): %v", tc.key, tc.value, err)
		}
		if got != tc.want {
			t.Fatalf("desField(%q,%q)=%q want %q", tc.key, tc.value, got, tc.want)
		}
	}
}

func TestTNOfMatchesJS(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"simple", map[string]any{"a": 1, "b": "x"}, "b20c52b68c13ca83b0c83603f0b1dfda"},
		{"nested", map[string]any{"a": 1, "battery": map[string]any{"charging": 1, "level": 0.85}}, "1c62eb20b063d3c6eda2dc04b06b1e43"},
		{"time", map[string]any{"time": 1757533356.123, "svm": float64(1757533356123)}, "58919ede8eabd693cc95caf3b534f3d2"},
		{"proto", map[string]any{"protocol": 133, "os": "web"}, "fa2fad53a758c42a76372d2b3634b275"},
		{"zero", map[string]any{"cdp": 0, "status": "true"}, "a44012424a896fb18ae4e869f92a9b9e"},
	}
	for _, tc := range cases {
		if got := tnOf(tc.in); got != tc.want {
			t.Fatalf("%s tnOf=%s want %s", tc.name, got, tc.want)
		}
	}
}

func TestConfuseLeavesEmptyRefererAndRenames(t *testing.T) {
	out, err := confuse([]kv{
		{"referer", ""},
		{"os", "web"},
		{"smid", "abc"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	for _, p := range out {
		got[p.k] = p.v
	}
	if _, ok := got["referer"]; ok {
		t.Fatal("referer should be renamed")
	}
	if got["vu"] != "" {
		t.Fatalf("empty referer should stay empty, got %#v", got["vu"])
	}
	if got["dh"] == "web" || got["dh"] == "" {
		t.Fatalf("os should be DES-encrypted, got %#v", got["dh"])
	}
	if got["smid"] != "abc" {
		t.Fatalf("unknown field smid should pass through, got %#v", got["smid"])
	}
}

func TestLocalSMIDFormat(t *testing.T) {
	id := localSMID(time.Date(2026, 9, 10, 12, 30, 45, 0, time.Local))
	if len(id) != 63 {
		t.Fatalf("len=%d id=%s", len(id), id)
	}
	if !strings.HasPrefix(id, "20260910123045") {
		t.Fatalf("timestamp prefix: %s", id)
	}
	if id[46:48] != "00" {
		t.Fatalf("00 marker: %s", id)
	}
	if !strings.HasSuffix(id, "0") {
		t.Fatalf("suffix: %s", id)
	}
	hexPart := id[14:46] + id[48:62]
	for _, r := range hexPart {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			t.Fatalf("non-hex in smid: %s", id)
		}
	}
}

func TestJoinProfileURL(t *testing.T) {
	if got := joinProfileURL("fp-it-acc.portal101.cn"); got != "https://fp-it-acc.portal101.cn/deviceprofile/v4" {
		t.Fatalf("host: %s", got)
	}
	if got := joinProfileURL("http://127.0.0.1:9"); got != "http://127.0.0.1:9/deviceprofile/v4" {
		t.Fatalf("abs: %s", got)
	}
}

func TestFetchProtocolDeviceID(t *testing.T) {
	var seen struct {
		ua      string
		ctype   string
		origin  string
		payload map[string]any
	}
	wantID := "NlM4B07tkAd9/BDOrew4lrBnS6RPQZfDZczOl8c94iSKWdOj9pN3BGLJEkfd0a0FKg7oO0/65VIvtFLf4O8X1A=="
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/deviceprofile/v4" {
			t.Errorf("path=%s", r.URL.Path)
		}
		seen.ua = r.Header.Get("User-Agent")
		seen.ctype = r.Header.Get("Content-Type")
		seen.origin = r.Header.Get("Origin")
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		if err := json.Unmarshal(raw, &seen.payload); err != nil {
			t.Errorf("json: %v", err)
			return
		}
		_, _ = w.Write([]byte(`{"code":1100,"detail":{"deviceId":"` + wantID + `"}}`))
	}))
	t.Cleanup(srv.Close)

	old := profileURL
	profileURL = srv.URL + "/deviceprofile/v4"
	t.Cleanup(func() { profileURL = old })

	got, err := FetchProtocolDeviceID(context.Background())
	if err != nil {
		t.Fatalf("FetchProtocolDeviceID: %v", err)
	}
	if got != "B"+wantID {
		t.Fatalf("id=%q", got)
	}
	if seen.ctype != "application/json;charset=UTF-8" {
		t.Fatalf("content-type=%q", seen.ctype)
	}
	if seen.origin != "https://platform.deepseek.com" {
		t.Fatalf("origin=%q", seen.origin)
	}
	if seen.ua == "" {
		t.Fatal("missing ua")
	}
	if seen.payload["appId"] != smApp || seen.payload["organization"] != smOrg {
		t.Fatalf("payload ids %#v", seen.payload)
	}
	if seen.payload["os"] != "web" {
		t.Fatalf("os=%#v", seen.payload["os"])
	}
	if jsonNumber(seen.payload["encode"]) != 5 || jsonNumber(seen.payload["compress"]) != 2 {
		t.Fatalf("encode/compress %#v", seen.payload)
	}
	data, _ := seen.payload["data"].(string)
	if data == "" {
		t.Fatal("missing data")
	}
	ep, _ := seen.payload["ep"].(string)
	if ep == "" {
		t.Fatal("missing ep")
	}
}

func TestFetchProtocolDeviceIDRejectsNon1100(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":1001,"message":"nope"}`))
	}))
	t.Cleanup(srv.Close)
	old := profileURL
	profileURL = srv.URL + "/deviceprofile/v4"
	t.Cleanup(func() { profileURL = old })
	if _, err := FetchProtocolDeviceID(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
