package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPostSendsJSONAndKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || r.URL.Path != "/internal/x" || r.Header.Get("X-Site-Service-Key") != "secret" ||
			r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Admin-Session") != "token" || string(body) != `{"a":1}` {
			t.Errorf("request %s %s %v %q", r.Method, r.URL.Path, r.Header, body)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	c := New(" "+server.URL+"/ ", "X-Site-Service-Key", " secret ", time.Second)
	var result struct{ OK bool }
	if err := c.Post(context.Background(), "/internal/x", map[string]int{"a": 1}, &result, http.Header{"X-Admin-Session": {"token"}}); err != nil || !result.OK {
		t.Fatalf("got %v %v", result, err)
	}
}

func TestGetSendsNoBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.ContentLength > 0 || r.Header.Get("Content-Type") != "" {
			t.Errorf("get %s %d %q", r.Method, r.ContentLength, r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	if err := New(server.URL, "X-Key", "k", time.Second).Get(context.Background(), "/x", nil); err != nil {
		t.Fatal(err)
	}
}

func TestStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"slow down","retry_after":30}`))
	}))
	defer server.Close()
	err := New(server.URL, "X-Key", "k", time.Second).Post(context.Background(), "/x", map[string]int{}, nil)
	var status *StatusError
	if !errors.As(err, &status) || status.Status != http.StatusTooManyRequests || status.Header.Get("Retry-After") != "30" || !strings.Contains(string(status.Body), "slow down") {
		t.Fatalf("got %v", err)
	}
	if err.Error() != "POST /x: HTTP 429" {
		t.Fatalf("message %q", err.Error())
	}
}

func TestTimeoutAndBadAnswer(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { time.Sleep(200 * time.Millisecond) }))
	defer slow.Close()
	err := New(slow.URL, "X-Key", "k", 20*time.Millisecond).Get(context.Background(), "/x", nil)
	var status *StatusError
	if err == nil || errors.As(err, &status) || !strings.HasPrefix(err.Error(), "GET /x: ") {
		t.Fatalf("timeout: %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("not json")) }))
	defer bad.Close()
	var result map[string]any
	if err := New(bad.URL, "X-Key", "k", time.Second).Get(context.Background(), "/x", &result); err == nil || !strings.Contains(err.Error(), "decode") {
		t.Fatalf("bad answer: %v", err)
	}
}
