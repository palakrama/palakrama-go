// Package httpx holds the JSON request and response helpers every Palakrama
// service's handlers use. Error bodies are not here: each service keeps its
// own error shape, which its clients read.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// WriteJSON answers status with value as JSON (Content-Type application/json).
// An encoding error is ignored: the status line is already sent.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// ErrMultipleValues is returned by DecodeJSON for a body with more than one JSON value.
var ErrMultipleValues = errors.New("request body must contain one JSON value")

// DecodeJSON reads exactly one JSON value from the request body into target,
// refusing fields target does not have and anything after the value. It does
// not limit the size; the service's handler (http.MaxBytesHandler) does.
func DecodeJSON(r *http.Request, target any) error {
	if r.Body == nil {
		return io.EOF
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var next json.RawMessage
	if err := decoder.Decode(&next); !errors.Is(err, io.EOF) {
		if err == nil {
			return ErrMultipleValues
		}
		return err
	}
	return nil
}
