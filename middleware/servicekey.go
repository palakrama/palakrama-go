package middleware

import (
	"context"
	"crypto/subtle"
	"net/http"
)

// Reject answers a request whose service key is missing or wrong. configured
// is false when the service has no key set at all (every accepted key is
// empty), so a service can answer that as its own setup error.
type Reject func(w http.ResponseWriter, configured bool)

// RejectText answers 401 with the plain text "unauthorized", also when no key
// is configured.
func RejectText(w http.ResponseWriter, _ bool) {
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

// RejectTextUnconfigured answers 500 with notConfigured when the service has
// no key set, else 401 "unauthorized" as plain text.
func RejectTextUnconfigured(notConfigured string) Reject {
	return func(w http.ResponseWriter, configured bool) {
		if !configured {
			http.Error(w, notConfigured, http.StatusInternalServerError)
			return
		}
		RejectText(w, true)
	}
}

// RejectJSON answers 401 with body as JSON, also when no key is configured.
func RejectJSON(body string) Reject {
	return func(w http.ResponseWriter, _ bool) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(body))
	}
}

// MatchKey returns the 1-based position of the key in keys that provided
// equals, or 0. Every comparison is constant-time and runs for every key; an
// empty key (not configured) never matches, nor does an empty provided value.
func MatchKey(provided string, keys ...string) int {
	match := 0
	for index, key := range keys {
		if key == "" || provided == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(key)) == 1 && match == 0 {
			match = index + 1
		}
	}
	return match
}

type keyContext struct{}

// RequireKey lets a request through when its header holds one of keys (see
// MatchKey) and records which one (KeyOf); any other request goes to reject.
// A service passes several keys when several callers may use the route, such
// as Palakrama's key and the admin app's key.
func RequireKey(header string, reject Reject, next http.Handler, keys ...string) http.Handler {
	configured := false
	for _, key := range keys {
		if key != "" {
			configured = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		match := 0
		if configured {
			match = MatchKey(r.Header.Get(header), keys...)
		}
		if match == 0 {
			reject(w, configured)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), keyContext{}, match)))
	})
}

// KeyOf returns the 1-based position (in RequireKey's keys) of the key the
// request used, or 0 outside RequireKey.
func KeyOf(ctx context.Context) int {
	match, _ := ctx.Value(keyContext{}).(int)
	return match
}
