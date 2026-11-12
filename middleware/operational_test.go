package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func serve(t *testing.T, maintenance string, next http.Handler, path, requestID string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	recorder := httptest.NewRecorder()
	Operational("site", maintenance, next).ServeHTTP(recorder, request)
	return recorder
}

var ok = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("ok")) })

func TestOperationalAddsIdentity(t *testing.T) {
	recorder := serve(t, "", ok, "/x", "abc-123_X")
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("got %d %q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("X-Service-Name"); got != "site" {
		t.Fatalf("service name %q", got)
	}
	if got := recorder.Header().Get("X-Request-ID"); got != "abc-123_X" {
		t.Fatalf("a valid request id is kept, got %q", got)
	}
}

func TestOperationalReplacesInvalidRequestID(t *testing.T) {
	for _, id := range []string{"bad id", "x/y", string(make([]byte, 65))} {
		got := serve(t, "", ok, "/x", id).Header().Get("X-Request-ID")
		if got == id || len(got) != 32 {
			t.Fatalf("id %q replaced by %q", id, got)
		}
	}
}

func TestOperationalMaintenance(t *testing.T) {
	for _, value := range []string{"1", "true", " YES ", "on"} {
		recorder := serve(t, value, ok, "/internal/x", "")
		if recorder.Code != http.StatusServiceUnavailable || recorder.Header().Get("Retry-After") != "60" {
			t.Fatalf("%q: got %d", value, recorder.Code)
		}
		var body serviceErrorResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || body.Error.Code != "SERVICE_MAINTENANCE" || body.Service != "site" || body.RequestID == "" {
			t.Fatalf("%q: body %s", value, recorder.Body.String())
		}
		if serve(t, value, ok, "/ready", "").Code != http.StatusServiceUnavailable {
			t.Fatalf("%q: /ready answers 503", value)
		}
		if serve(t, value, ok, "/health", "").Code != http.StatusOK {
			t.Fatalf("%q: /health stays up", value)
		}
	}
	if serve(t, "off", ok, "/internal/x", "").Code != http.StatusOK {
		t.Fatal("off is not maintenance")
	}
}

func TestOperationalRecoversPanic(t *testing.T) {
	recorder := serve(t, "", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }), "/x", "")
	var body serviceErrorResponse
	if recorder.Code != http.StatusInternalServerError || json.Unmarshal(recorder.Body.Bytes(), &body) != nil || body.Error.Code != "SERVICE_ERROR" {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestOperationalRepanicsAfterWrite(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("a panic after the response started is passed on")
		}
	}()
	serve(t, "", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		panic("late")
	}), "/x", "")
}
