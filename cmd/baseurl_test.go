package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/keychain"
)

// TestBaseURL_FlagTakesPrecedenceOverEnv starts two mock servers, points the
// env var at the "wrong" one and the flag at the "right" one, and verifies
// the flag wins.
func TestBaseURL_FlagTakesPrecedenceOverEnv(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}

	var hitEnv, hitFlag bool
	envServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitEnv = true
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(envServer.Close)
	flagServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitFlag = true
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(flagServer.Close)

	t.Setenv("KALISTAT_API_URL", envServer.URL)

	if err := runCLI(t, "sources", "--base-url", flagServer.URL); err != nil {
		t.Fatalf("sources: %v", err)
	}
	if hitEnv {
		t.Error("env var server was hit; flag should have taken precedence")
	}
	if !hitFlag {
		t.Error("flag server was not hit")
	}
}

func TestBaseURL_EnvUsedWhenFlagAbsent(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}

	var hit bool
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		hit = true
		_, _ = w.Write([]byte(`{"data":[]}`))
	})

	if err := runCLI(t, "sources"); err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Error("env var server should have been hit")
	}
}

func TestBaseURL_FlagAppearsInHelp(t *testing.T) {
	buf := resetCmd(t)

	if err := runCLI(t, "--help"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "--base-url") {
		t.Errorf("help should document --base-url: %q", out)
	}
	if !strings.Contains(out, "KALISTAT_API_URL") {
		t.Errorf("help should mention the env var for discoverability: %q", out)
	}
}

func TestBaseURL_InvalidFlagRejected(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}

	// http://public — not loopback, must be rejected by NewWithToken.
	err := runCLI(t, "sources", "--base-url", "http://attacker.com/v1")
	if err == nil {
		t.Fatal("expected error for plain-http non-loopback base URL passed via flag")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Errorf("error should mention the scheme requirement: %q", err)
	}
}
