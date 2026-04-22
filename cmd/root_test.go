package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/api"
)

func TestPrintError_APIError_JSONModeWritesBodyToStdout(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	apiErr := &api.APIError{
		StatusCode: 401,
		Message:    "Missing or invalid Bearer token",
		Body:       []byte(`{"error":{"code":"unauthorized"}}`),
	}

	printError(stdout, stderr, apiErr, true)

	if !strings.Contains(stdout.String(), `"unauthorized"`) {
		t.Errorf("stdout should contain raw body: %q", stdout.String())
	}
	if !strings.HasSuffix(stdout.String(), "\n") {
		t.Errorf("stdout should end with newline: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty in JSON mode: %q", stderr.String())
	}
}

func TestPrintError_APIError_JSONMode_PreservesExistingNewline(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	body := []byte("{\"error\":{}}\n")
	apiErr := &api.APIError{StatusCode: 500, Body: body}

	printError(stdout, stderr, apiErr, true)

	if stdout.String() != string(body) {
		t.Errorf("stdout should equal body exactly (no doubled newline), got %q", stdout.String())
	}
}

func TestPrintError_PlainErrorGoesToStderr(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	printError(stdout, stderr, errors.New("boom"), false)

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Error: boom") {
		t.Errorf("stderr should contain formatted error: %q", stderr.String())
	}
}

// TestPrintError_IntegrationWithJSONFlag pins the end-to-end contract that
// `kalistat <cmd> --json` against a failing endpoint produces the raw API
// error body on stdout (not the formatted message on stderr). The unit
// tests above exercise printError in isolation with a hardcoded bool; this
// test confirms that cobra actually wires --json into the jsonOutput var
// that Execute() reads.
func TestPrintError_IntegrationWithJSONFlag(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t, `{"error":{"code":"unauthorized","message":"bad"}}`, http.StatusUnauthorized)

	err := runCLI(t, "sources", "--json")
	if !jsonOutput {
		t.Fatal("--json flag did not set jsonOutput; cobra wiring is broken")
	}
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}

	// Simulate what Execute does with the same writers Execute would use.
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	printError(stdout, stderr, err, jsonOutput)

	if !bytes.Contains(stdout.Bytes(), []byte(`"unauthorized"`)) {
		t.Errorf("raw JSON body should be on stdout: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr should be empty in JSON mode: %q", stderr.String())
	}
}

func TestPrintError_APIErrorWithoutJSONModeGoesToStderr(t *testing.T) {
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	apiErr := &api.APIError{
		StatusCode: 401,
		Message:    "Missing or invalid Bearer token",
		Body:       []byte(`{"error":{"code":"unauthorized"}}`),
	}

	printError(stdout, stderr, apiErr, false)

	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Missing or invalid Bearer token") {
		t.Errorf("stderr should contain parsed message: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "unauthorized") {
		t.Errorf("stderr must not leak raw body when JSON mode is off: %q", stderr.String())
	}
}
