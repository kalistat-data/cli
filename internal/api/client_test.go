package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *APIError
		want string
	}{
		{"message wins", &APIError{StatusCode: 401, Message: "Bad token"}, "Bad token"},
		{"fallback to status text", &APIError{StatusCode: 503}, "Service Unavailable (503)"},
		{"fallback to HTTP code for unknown status", &APIError{StatusCode: 799}, "HTTP 799"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.err.Error(); got != c.want {
				t.Errorf("Error() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestNewWithToken_URLValidation(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{"https public", "https://app.kalistat.com/api/v1", false},
		{"http on 127.0.0.1", "http://127.0.0.1:8080", false},
		{"http on localhost", "http://localhost:1234", false},
		{"http on IPv6 loopback", "http://[::1]:8000", false},
		{"http on public host rejected", "http://attacker.com/v1", true},
		{"file scheme rejected", "file:///etc/passwd", true},
		{"ftp scheme rejected", "ftp://example.com/", true},
		{"ws scheme rejected", "ws://example.com/", true},
		{"garbage rejected", "not-a-url", true},
		{"missing scheme rejected", "://missing-scheme", true},
		{"empty host rejected", "https://", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewWithToken("t", c.baseURL)
			if (err != nil) != c.wantErr {
				t.Errorf("err = %v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestNewWithToken_EmptyBaseURLUsesDefault(t *testing.T) {
	c, err := NewWithToken("t", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.BaseURL, "https://") {
		t.Errorf("default BaseURL should use https, got %q", c.BaseURL)
	}
}

func TestGetJSON_ForwardsQueryParams(t *testing.T) {
	var gotPath, gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, _ := NewWithToken("tok", server.URL)
	q := url.Values{}
	q.Set("q", "employment")
	q.Set("page", "2")
	q.Set("pattern", "A.*.TOT") // wildcard must be percent-encoded

	if _, err := c.GetJSON("/search", q, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotPath, "/search") {
		t.Errorf("path = %q, want to end with /search", gotPath)
	}
	for _, want := range []string{"q=employment", "page=2", "pattern=A.%2A.TOT"} {
		if !strings.Contains(gotQuery, want) {
			t.Errorf("query %q missing %q", gotQuery, want)
		}
	}
}

func TestGetJSON_NilQueryProducesNoQueryString(t *testing.T) {
	var gotQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, _ := NewWithToken("tok", server.URL)
	if _, err := c.GetJSON("/sources", nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "" {
		t.Errorf("nil query should produce no RawQuery, got %q", gotQuery)
	}
}

func TestGetJSON_SendsBearerHeader(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c, err := NewWithToken("tok", server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetJSON("/", nil, nil); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer tok")
	}
}

func TestGetJSON_DecodesIntoOut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"kalistat","count":3}`))
	}))
	defer server.Close()

	c, _ := NewWithToken("t", server.URL)
	var got struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	body, err := c.GetJSON("/", nil, &got)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "kalistat" || got.Count != 3 {
		t.Errorf("decoded = %+v", got)
	}
	if !strings.Contains(string(body), `"count":3`) {
		t.Errorf("raw body should be returned alongside: %q", body)
	}
}

func TestGetJSON_DecodeErrorWhenBodyIsNotJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`this is definitely not json`))
	}))
	defer server.Close()

	c, _ := NewWithToken("tok", server.URL)
	var target struct {
		Data string `json:"data"`
	}
	body, err := c.GetJSON("/", nil, &target)
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("error = %q, want to mention 'decode response'", err)
	}
	if !strings.Contains(string(body), "not json") {
		t.Errorf("body should still be returned so callers can inspect it: %q", body)
	}
}

func TestGetJSON_ParsesAPIErrorEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"Bad token"}}`))
	}))
	defer server.Close()

	c, _ := NewWithToken("tok", server.URL)
	_, err := c.GetJSON("/", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v (%T), want *APIError", err, err)
	}
	if apiErr.Code != "unauthorized" || apiErr.Message != "Bad token" {
		t.Errorf("APIError = %+v", apiErr)
	}
}

func TestGetJSON_APIErrorWhenBodyIsNotJSONEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`<html>500</html>`))
	}))
	defer server.Close()

	c, _ := NewWithToken("tok", server.URL)
	_, err := c.GetJSON("/", nil, nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v (%T), want *APIError", err, err)
	}
	if apiErr.Message != "" {
		t.Errorf("Message = %q, want empty (no envelope to parse)", apiErr.Message)
	}
	if !strings.Contains(apiErr.Error(), "Internal Server Error") {
		t.Errorf("Error() should fall back to status text: %q", apiErr.Error())
	}
}

// TestGetJSON_TransportError exercises the non-HTTP-response path: the server
// is started and then closed so the TCP dial itself fails.
func TestGetJSON_TransportError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close() // close immediately — subsequent dials will fail

	c, _ := NewWithToken("tok", server.URL)
	_, err := c.GetJSON("/", nil, nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("transport failures must not be reported as APIError, got %+v", apiErr)
	}
}

// TestGetJSON_BuildURLError exercises url.JoinPath's rare error branch. A path
// containing an invalid host-like component can't be cleanly appended.
func TestGetJSON_InvalidPathSegment(t *testing.T) {
	c, _ := NewWithToken("tok", "https://app.kalistat.com/api/v1")
	// %-escape hex that isn't valid hex forces the underlying URL parse to fail
	// somewhere — either in JoinPath or in http.NewRequest.
	_, err := c.GetJSON("/%zz", nil, nil)
	if err == nil {
		t.Fatal("expected URL error for bad percent-escape")
	}
}

// TestGetJSON_LimitsResponseBody verifies the 10 MiB LimitReader trips — the
// CLI should never read an unbounded amount of data into memory even if a
// (redirected or misbehaving) server sends more.
func TestGetJSON_LimitsResponseBody(t *testing.T) {
	const oversize = (10 << 20) + 1024
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Write a JSON string of zeros padded to oversize bytes so the Unmarshal
		// step is skipped; we're only asserting the read is bounded.
		_, _ = w.Write([]byte{'"'})
		_, _ = w.Write(make([]byte, oversize-2))
		_, _ = w.Write([]byte{'"'})
	}))
	defer server.Close()

	c, _ := NewWithToken("tok", server.URL)
	body, _ := c.GetJSON("/", nil, nil)
	if len(body) > 10<<20 {
		t.Errorf("read %d bytes, want <= 10 MiB (LimitReader should cap)", len(body))
	}
}
