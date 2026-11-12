package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func call(handler http.Handler, header, value string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "/internal/x", nil)
	if value != "" {
		request.Header.Set(header, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestMatchKey(t *testing.T) {
	cases := []struct {
		provided string
		keys     []string
		want     int
	}{
		{"a", []string{"a", "b"}, 1},
		{"b", []string{"a", "b"}, 2},
		{"c", []string{"a", "b"}, 0},
		{"", []string{"", "b"}, 0},
		{"", []string{""}, 0},
		{"a", []string{"", "a"}, 2},
		{"ab", []string{"a"}, 0},
	}
	for _, c := range cases {
		if got := MatchKey(c.provided, c.keys...); got != c.want {
			t.Errorf("MatchKey(%q, %q) = %d, want %d", c.provided, c.keys, got, c.want)
		}
	}
}

func TestRequireKeyRecordsTheKeyUsed(t *testing.T) {
	var seen int
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { seen = KeyOf(r.Context()) })
	handler := RequireKey("X-Key", RejectText, next, "client", "admin")
	if recorder := call(handler, "X-Key", "admin"); recorder.Code != http.StatusOK || seen != 2 {
		t.Fatalf("admin key: %d, key %d", recorder.Code, seen)
	}
	if recorder := call(handler, "X-Key", "client"); recorder.Code != http.StatusOK || seen != 1 {
		t.Fatalf("client key: %d, key %d", recorder.Code, seen)
	}
	for _, value := range []string{"", "wrong"} {
		recorder := call(handler, "X-Key", value)
		if recorder.Code != http.StatusUnauthorized || strings.TrimSpace(recorder.Body.String()) != "unauthorized" {
			t.Fatalf("%q: %d %q", value, recorder.Code, recorder.Body.String())
		}
	}
}

func TestRejects(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("must not pass") })

	unconfigured := RejectTextUnconfigured("site service key is not configured")
	if recorder := call(RequireKey("X-Key", unconfigured, next, ""), "X-Key", ""); recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "not configured") {
		t.Fatalf("unconfigured: %d %q", recorder.Code, recorder.Body.String())
	}
	if recorder := call(RequireKey("X-Key", unconfigured, next, "k"), "X-Key", "bad"); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("configured, wrong key: %d", recorder.Code)
	}
	// A service without a key set refuses everything, even an empty header value.
	if recorder := call(RequireKey("X-Key", RejectText, next, ""), "X-Key", ""); recorder.Code != http.StatusUnauthorized {
		t.Fatalf("no key, plain reject: %d", recorder.Code)
	}

	recorder := call(RequireKey("X-Key", RejectJSON(`{"error":"unauthorized"}`), next, "k"), "X-Key", "bad")
	if recorder.Code != http.StatusUnauthorized || recorder.Header().Get("Content-Type") != "application/json" || recorder.Body.String() != `{"error":"unauthorized"}` {
		t.Fatalf("json: %d %q %q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}
