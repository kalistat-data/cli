package cmd

import (
	"bytes"
	"errors"
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
