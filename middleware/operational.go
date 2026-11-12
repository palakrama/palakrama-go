// Package middleware holds the HTTP middleware every Palakrama service uses.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type serviceError struct {
	Code    string `json:"code"`
	Service string `json:"service"`
	Message string `json:"message"`
}

type serviceErrorResponse struct {
	Success   bool         `json:"success"`
	Error     serviceError `json:"error"`
	Service   string       `json:"service"`
	RequestID string       `json:"requestId"`
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Operational wraps a service's whole handler. It adds the service name and a
// request ID (X-Service-Name, X-Request-ID; a valid incoming ID is kept) to
// every response, logs requests (every GET and POST, server errors, and
// anything slower than 100 ms), turns a panic into a 500 service error, and in
// maintenance mode answers everything except /health with 503
// SERVICE_MAINTENANCE. maintenanceMode is the MAINTENANCE_MODE setting: 1,
// true, yes, or on (case-insensitive) turn it on.
func Operational(service, maintenanceMode string, next http.Handler) http.Handler {
	maintenance := enabled(maintenanceMode)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if !validRequestID(requestID) {
			requestID = newRequestID()
		}
		w.Header().Set("X-Service-Name", service)
		w.Header().Set("X-Request-ID", requestID)

		if maintenance && r.URL.Path == "/ready" {
			log.Printf("request method=%s path=%s status=503 request_id=%s code=SERVICE_MAINTENANCE", r.Method, r.URL.Path, requestID)
			w.Header().Set("Retry-After", "60")
			writeServiceError(w, http.StatusServiceUnavailable, service, requestID, "SERVICE_MAINTENANCE", "The service is under maintenance.")
			return
		}
		if maintenance && r.URL.Path != "/health" {
			log.Printf("request method=%s path=%s status=503 request_id=%s code=SERVICE_MAINTENANCE", r.Method, r.URL.Path, requestID)
			w.Header().Set("Retry-After", "60")
			writeServiceError(w, http.StatusServiceUnavailable, service, requestID, "SERVICE_MAINTENANCE", "The service is under maintenance. Please try again in a moment.")
			return
		}

		tracked := &statusWriter{ResponseWriter: w}
		defer func() {
			duration := time.Since(startedAt)
			durationMS := duration.Milliseconds()
			if recovered := recover(); recovered != nil {
				log.Printf("request method=%s path=%s status=500 duration_ms=%d request_id=%s panic=%v", r.Method, r.URL.Path, durationMS, requestID, recovered)
				if tracked.status == 0 {
					writeServiceError(w, http.StatusInternalServerError, service, requestID, "SERVICE_ERROR", "Temporary problem. Please try again.")
					return
				}
				panic(recovered)
			}
			status := tracked.status
			if status == 0 {
				status = http.StatusOK
			}
			if r.Method == http.MethodGet || r.Method == http.MethodPost || status >= http.StatusInternalServerError || duration >= 100*time.Millisecond {
				log.Printf("request method=%s path=%s status=%d duration_ms=%d request_id=%s", r.Method, r.URL.Path, status, durationMS, requestID)
			}
		}()
		next.ServeHTTP(tracked, r)
	})
}

func enabled(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func validRequestID(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && char != '-' && char != '_' {
			return false
		}
	}
	return true
}

func newRequestID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err == nil {
		return hex.EncodeToString(value[:])
	}
	return strconv.FormatInt(time.Now().UTC().UnixNano(), 10)
}

func writeServiceError(w http.ResponseWriter, status int, service, requestID, code, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(serviceErrorResponse{
		Success: false,
		Error: serviceError{
			Code: code, Service: service, Message: message,
		},
		Service: service, RequestID: requestID,
	})
}
