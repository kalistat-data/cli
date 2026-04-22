package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kalistat-data/cli/internal/keychain"
)

// loggedIn resets the harness and seeds the keychain with a test token.
// Returns the output buffer so tests don't need to chain resetCmd themselves.
func loggedIn(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}
	return buf
}

// mockAPI starts an httptest server serving `handler`, points the CLI at it
// via KALISTAT_API_URL, and registers cleanup. Tests never need to remember
// `defer server.Close()` or to unset the env var.
func mockAPI(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	t.Setenv("KALISTAT_API_URL", server.URL)
	return server
}

// mockJSONAPI is shorthand for servers that return a single fixed JSON body.
// Pass status=0 for a plain 200 OK.
func mockJSONAPI(t *testing.T, body string, status int) *httptest.Server {
	t.Helper()
	return mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if status != 0 {
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte(body))
	})
}
