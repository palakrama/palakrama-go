// Package client is the JSON client a Palakrama service uses to call another
// one: the target's base URL, its key header, and a timeout. What a call
// means (mapping a status to a domain error, skipping a call when the target
// is not configured) stays with the calling service's adapter.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	// maxAnswer is the most of a 2xx answer that is decoded.
	maxAnswer = 1 << 20
	// maxErrorBody is the most of a non-2xx answer kept in StatusError.Body.
	maxErrorBody = 64 << 10
)

// Client calls one service. It is safe for concurrent use.
type Client struct {
	baseURL   string
	keyHeader string
	key       string
	http      *http.Client
}

// New returns a client for the service at baseURL (spaces and trailing
// slashes trimmed) that sends key in keyHeader with every request and gives
// up after timeout.
func New(baseURL, keyHeader, key string, timeout time.Duration) *Client {
	return &Client{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		keyHeader: keyHeader,
		key:       strings.TrimSpace(key),
		http:      &http.Client{Timeout: timeout},
	}
}

// StatusError is a non-2xx answer. Body holds up to 64 KiB of it, for
// callers that read the target's error body.
type StatusError struct {
	Method string
	Path   string
	Status int
	Header http.Header
	Body   []byte
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Path, e.Status)
}

// Post sends body as JSON (nil sends none) and decodes a 2xx answer into
// result (nil skips decoding). header adds request headers, such as an admin
// session. A non-2xx answer is a *StatusError.
func (c *Client) Post(ctx context.Context, path string, body, result any, header ...http.Header) error {
	return c.Do(ctx, http.MethodPost, path, body, result, header...)
}

// Get decodes a 2xx answer into result (nil skips decoding); see Post.
func (c *Client) Get(ctx context.Context, path string, result any, header ...http.Header) error {
	return c.Do(ctx, http.MethodGet, path, nil, result, header...)
}

// Do sends one request; see Post.
func (c *Client) Do(ctx context.Context, method, path string, body, result any, header ...http.Header) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("%s %s: encode: %w", method, path, err)
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set(c.keyHeader, c.key)
	for _, extra := range header {
		for name, values := range extra {
			for _, value := range values {
				request.Header.Add(name, value)
			}
		}
	}
	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	// Read what is left so the connection can be reused.
	defer func() { _, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxAnswer)) }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
		return &StatusError{Method: method, Path: path, Status: response.StatusCode, Header: response.Header, Body: data}
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxAnswer)).Decode(result); err != nil {
		return fmt.Errorf("%s %s: decode: %w", method, path, err)
	}
	return nil
}
