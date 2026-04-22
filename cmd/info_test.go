package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/keychain"
)

const rootResponseJSON = `{
  "data": {
    "name": "Kalistat",
    "version": "v1",
    "sources": ["istat", "eurostat"],
    "rate_limit": {"requests_per_minute": 1000},
    "links": {}
  },
  "meta": {"generated_at": "2026-04-22T17:04:43Z"}
}`

func TestInfo_PrettyOutput(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, rootResponseJSON, 0)

	if err := runCLI(t, "info"); err != nil {
		t.Fatalf("info: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Kalistat CLI", "Using API v1", "Sources:", "istat", "eurostat", "Rate limit", "1000"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestInfo_EmptyFieldsOmitOptionalLines(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":{},"meta":{}}`, 0)

	if err := runCLI(t, "info"); err != nil {
		t.Fatalf("info: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Kalistat CLI") {
		t.Errorf("output missing 'Kalistat CLI': %q", out)
	}
	for _, unwanted := range []string{"Using API", "Sources:", "Rate limit"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output should not contain %q when the field is empty:\n%s", unwanted, out)
		}
	}
}

func TestInfo_JSONFlagReturnsRaw(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, rootResponseJSON, 0)

	if err := runCLI(t, "info", "--json"); err != nil {
		t.Fatalf("info --json: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("missing 'data': %s", buf.String())
	}
}

func TestInfo_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	err := runCLI(t, "info")
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
	if !strings.Contains(err.Error(), "no API token") {
		t.Errorf("error = %q, want to mention missing token", err)
	}
}

// failingWriter always errors on Write; used to test writeRaw's error propagation.
type failingWriter struct{}

var errWriter = fmt.Errorf("writer boom")

func (failingWriter) Write(p []byte) (int, error) { return 0, errWriter }

func TestWriteRaw_AppendsNewlineOnlyWhenMissing(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"no trailing newline", `{"a":1}`, "{\"a\":1}\n"},
		{"already has newline", "{\"a\":1}\n", "{\"a\":1}\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			buf := loggedIn(t)
			mockJSONAPI(t, c.body, 0)

			if err := runCLI(t, "info", "--json"); err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != c.want {
				t.Errorf("output = %q, want %q", got, c.want)
			}
		})
	}
}

func TestWriteRaw_PropagatesWriterErrors(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}
	rootCmd.SetOut(failingWriter{})
	mockJSONAPI(t, rootResponseJSON, 0)

	err := runCLI(t, "info", "--json")
	if !errors.Is(err, errWriter) {
		t.Fatalf("err = %v, want to wrap errWriter", err)
	}
}
