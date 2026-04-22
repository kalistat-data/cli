package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/api"
)

const sourcesResponseJSON = `{
  "data": [
    {"id":"istat","name":"ISTAT","country":"Italy","root_key":"IT","links":{}},
    {"id":"eurostat","name":"Eurostat","country":null,"root_key":"EU","links":{}}
  ],
  "meta": {"generated_at":"2026-04-22T17:04:43.262974Z"}
}`

func TestSources_PrettyByDefault(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, sourcesResponseJSON, 0)

	if err := runCLI(t, "sources"); err != nil {
		t.Fatalf("sources: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"ID", "NAME", "istat", "ISTAT", "Italy", "eurostat", "—", "IT", "EU"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "generated_at") {
		t.Errorf("pretty output should not include raw JSON fields: %s", out)
	}
}

// TestSources_PrettyColumnsAligned verifies tabwriter output actually aligns
// columns, not just that the right substrings are present.
func TestSources_PrettyColumnsAligned(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, sourcesResponseJSON, 0)

	if err := runCLI(t, "sources"); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("want header + rows, got %d line(s): %s", len(lines), buf.String())
	}
	// Every row should have the value in the NAME column start at the same offset.
	nameCol := regexp.MustCompile(`^\S+\s+`).FindStringIndex(lines[0])
	if nameCol == nil {
		t.Fatalf("can't find NAME column in %q", lines[0])
	}
	offset := nameCol[1]
	for _, row := range lines[1:] {
		cell := rune(row[offset-1])
		if cell != ' ' {
			t.Errorf("column break not aligned at offset %d in row %q", offset, row)
		}
	}
}

func TestSources_EmptyList(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{}}`, 0)

	if err := runCLI(t, "sources"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No sources available") {
		t.Errorf("want empty-state message, got %q", buf.String())
	}
}

func TestSources_JSONFlagReturnsRaw(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, sourcesResponseJSON, 0)

	if err := runCLI(t, "sources", "--json"); err != nil {
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
	loggedIn(t)
	mockJSONAPI(t,
		`{"error":{"code":"unauthorized","message":"Missing or invalid Bearer token"}}`,
		http.StatusUnauthorized,
	)

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
