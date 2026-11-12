package httpx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	WriteJSON(recorder, http.StatusCreated, map[string]any{"success": true})
	if recorder.Code != http.StatusCreated || recorder.Header().Get("Content-Type") != "application/json" || recorder.Body.String() != "{\"success\":true}\n" {
		t.Fatalf("got %d %q %q", recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.String())
	}
}

func TestDecodeJSON(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}
	decode := func(text string) (body, error) {
		var value body
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(text))
		return value, DecodeJSON(request, &value)
	}
	if value, err := decode(`{"name":"Arga"}`); err != nil || value.Name != "Arga" {
		t.Fatalf("valid: %v %v", value, err)
	}
	if _, err := decode(" {\"name\":\"Arga\"} \n"); err != nil {
		t.Fatalf("whitespace around the value is fine: %v", err)
	}
	if _, err := decode(`{"name":"Arga","extra":1}`); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := decode(`{"name":"a"}{"name":"b"}`); !errors.Is(err, ErrMultipleValues) {
		t.Fatalf("two values: %v", err)
	}
	if _, err := decode(`{"name":"a"} x`); err == nil {
		t.Fatal("trailing garbage accepted")
	}
	for _, text := range []string{"", "{", "[1]"} {
		if _, err := decode(text); err == nil {
			t.Fatalf("%q accepted", text)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/", nil)
	request.Body = nil
	if err := DecodeJSON(request, &body{}); err == nil {
		t.Fatal("nil body accepted")
	}
}
