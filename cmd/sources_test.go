package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/kalistat-data/cli/internal/keychain"
)

const sourcesResponseJSON = `{
  "data": [
    {"id":"istat","name":"ISTAT","country":"Italy","root_key":"IT","links":{}},
    {"id":"eurostat","name":"Eurostat","country":null,"root_key":"EU","links":{}}
  ],
  "meta": {"generated_at":"2026-04-22T17:04:43.262974Z"}
}`

func setJSONFlag(t *testing.T, v bool) {
	t.Helper()
	if err := rootCmd.PersistentFlags().Set("json", strFor(v)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rootCmd.PersistentFlags().Set("json", "false") })
}

func strFor(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestSources_PrettyByDefault(t *testing.T) {
	buf := resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sourcesResponseJSON))
	}))
	defer server.Close()
	t.Setenv("KALISTAT_API_URL", server.URL)

	if err := runCLI(t, "sources"); err != nil {
		t.Fatalf("sources: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "istat", "ISTAT", "Italy", "eurostat", "—"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "generated_at") {
		t.Errorf("pretty output should not include raw JSON fields: %s", out)
	}
}

func TestSources_JSONFlagReturnsRaw(t *testing.T) {
	buf := resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}
	setJSONFlag(t, true)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sourcesResponseJSON))
	}))
	defer server.Close()
	t.Setenv("KALISTAT_API_URL", server.URL)

	if err := runCLI(t, "sources"); err != nil {
		t.Fatalf("sources --json: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("JSON output missing 'data' field: %s", buf.String())
	}
}

func TestSources_UnauthorizedReturnsAPIError(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("bad"); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"Missing or invalid Bearer token"}}`))
	}))
	defer server.Close()
	t.Setenv("KALISTAT_API_URL", server.URL)

	err := runCLI(t, "sources")
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *api.APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if apiErr.Message != "Missing or invalid Bearer token" {
		t.Errorf("Message = %q", apiErr.Message)
	}
	if apiErr.Error() != "Missing or invalid Bearer token" {
		t.Errorf("Error() = %q, want only the message (no raw body leak)", apiErr.Error())
	}
	if !bytes.Contains(apiErr.Body, []byte(`"unauthorized"`)) {
		t.Errorf("Body not preserved for --json mode: %s", apiErr.Body)
	}
}
