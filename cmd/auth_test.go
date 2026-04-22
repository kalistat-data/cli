package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/zalando/go-keyring"
)

// resetCmd gives each test a clean keychain, fresh writers, and empty stdin.
// All mutations of rootCmd are rolled back via t.Cleanup so tests don't leak
// state to each other regardless of order or future parallelism.
func resetCmd(t *testing.T) *bytes.Buffer {
	t.Helper()
	keyring.MockInit()
	jsonOutput = false

	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetIn(strings.NewReader(""))
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
		rootCmd.SetIn(nil)
		rootCmd.SetArgs(nil)
		jsonOutput = false
	})
	return buf
}

func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func runCLIWithStdin(t *testing.T, stdin string, args ...string) error {
	t.Helper()
	rootCmd.SetIn(strings.NewReader(stdin))
	return runCLI(t, args...)
}

func TestAuthLogin_StoresTokenWithTimestamp(t *testing.T) {
	buf := resetCmd(t)

	if err := runCLIWithStdin(t, "secret\n", "auth", "login"); err != nil {
		t.Fatalf("auth login: %v", err)
	}

	entry, err := keychain.Get()
	if err != nil {
		t.Fatalf("keychain.Get: %v", err)
	}
	if entry.Token != "secret" {
		t.Errorf("Token = %q, want %q", entry.Token, "secret")
	}
	if age := time.Since(entry.CreatedAt); age < 0 || age > time.Minute {
		t.Errorf("CreatedAt not recent: age=%s", age)
	}
	if !strings.Contains(buf.String(), "Logged in") {
		t.Errorf("output = %q, want to contain %q", buf.String(), "Logged in")
	}
}

func TestAuthLogin_RejectsPositionalToken(t *testing.T) {
	resetCmd(t)

	if err := runCLIWithStdin(t, "anything\n", "auth", "login", "positional-token"); err == nil {
		t.Fatal("expected error: positional token must not be accepted")
	}
}

func TestAuthLogin_EmptyStdinFails(t *testing.T) {
	resetCmd(t)

	err := runCLIWithStdin(t, "", "auth", "login")
	if err == nil {
		t.Fatal("expected error when stdin is empty")
	}
}

func TestAuthLogin_TrimsWhitespace(t *testing.T) {
	resetCmd(t)

	if err := runCLIWithStdin(t, "  padded-token  \n", "auth", "login"); err != nil {
		t.Fatal(err)
	}
	entry, err := keychain.Get()
	if err != nil {
		t.Fatal(err)
	}
	if entry.Token != "padded-token" {
		t.Errorf("Token = %q, want %q", entry.Token, "padded-token")
	}
}

func TestAuthLogout_RemovesToken(t *testing.T) {
	buf := loggedIn(t)

	if err := runCLI(t, "auth", "logout"); err != nil {
		t.Fatalf("auth logout: %v", err)
	}

	if _, err := keychain.Get(); !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("after logout, keychain.Get err = %v, want ErrNotFound", err)
	}
	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("output = %q, want to contain %q", buf.String(), "Logged out")
	}
}

func TestAuthLogout_NotLoggedIn(t *testing.T) {
	buf := resetCmd(t)

	if err := runCLI(t, "auth", "logout"); err != nil {
		t.Fatalf("auth logout: %v", err)
	}
	if !strings.Contains(buf.String(), "Not logged in") {
		t.Errorf("output = %q, want to contain %q", buf.String(), "Not logged in")
	}
}

func TestAuthStatus_NotLoggedIn(t *testing.T) {
	buf := resetCmd(t)

	if err := runCLI(t, "auth", "status"); err != nil {
		t.Fatalf("auth status: %v", err)
	}
	if !strings.Contains(buf.String(), "Not logged in") {
		t.Errorf("output = %q, want to contain %q", buf.String(), "Not logged in")
	}
}

func TestAuthStatus_ValidToken(t *testing.T) {
	buf := loggedIn(t)
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer secret"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	})

	if err := runCLI(t, "auth", "status"); err != nil {
		t.Fatalf("auth status: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Logged in") {
		t.Errorf("output missing 'Logged in': %q", out)
	}
	if !strings.Contains(out, "Token age") {
		t.Errorf("output missing 'Token age': %q", out)
	}
}

func TestAuthStatus_InvalidTokenIsSilent(t *testing.T) {
	buf := loggedIn(t)
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})

	err := runCLI(t, "auth", "status")
	if !errors.Is(err, errSilent) {
		t.Fatalf("err = %v, want errSilent", err)
	}
	if !strings.Contains(buf.String(), "not valid") {
		t.Errorf("output = %q, want to contain %q", buf.String(), "not valid")
	}
}

func TestAuthStatus_APIUnreachable(t *testing.T) {
	loggedIn(t)
	// Point at a closed server: mock it and immediately close.
	server := mockAPI(t, func(http.ResponseWriter, *http.Request) {})
	server.Close()

	err := runCLI(t, "auth", "status")
	// The API call fails → status prints its line and returns errSilent.
	if !errors.Is(err, errSilent) {
		t.Errorf("err = %v, want errSilent for network failure", err)
	}
}

func TestAuthStatus_CorruptedEntry(t *testing.T) {
	resetCmd(t)
	if err := keyring.Set("kalistat", "api-token", "plain-old-token"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := runCLI(t, "auth", "status"); err == nil {
		t.Fatal("expected error for corrupted (non-JSON) keychain entry — backward compat should be gone")
	}
}

// TestAuthWorkflow exercises the full login/status/logout lifecycle to catch
// regressions where the pieces work individually but not together.
func TestAuthWorkflow(t *testing.T) {
	buf := resetCmd(t)
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	})

	if err := runCLIWithStdin(t, "secret\n", "auth", "login"); err != nil {
		t.Fatalf("login: %v", err)
	}
	if !strings.Contains(buf.String(), "Logged in") {
		t.Errorf("login output: %q", buf.String())
	}

	buf.Reset()
	if err := runCLI(t, "auth", "status"); err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(buf.String(), "Logged in") {
		t.Errorf("status output: %q", buf.String())
	}

	buf.Reset()
	if err := runCLI(t, "auth", "logout"); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if !strings.Contains(buf.String(), "Logged out") {
		t.Errorf("logout output: %q", buf.String())
	}

	buf.Reset()
	if err := runCLI(t, "auth", "status"); err != nil {
		t.Fatalf("post-logout status: %v", err)
	}
	if !strings.Contains(buf.String(), "Not logged in") {
		t.Errorf("post-logout status output: %q", buf.String())
	}
}

func TestHumanizeAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{59 * time.Second, "just now"},
		{time.Minute, "1 minute"},
		{5 * time.Minute, "5 minutes"},
		{time.Hour, "1 hour"},
		{2 * time.Hour, "2 hours"},
		{24 * time.Hour, "1 day"},
		{48 * time.Hour, "2 days"},
	}
	for _, c := range cases {
		if got := humanizeAge(c.d); got != c.want {
			t.Errorf("humanizeAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
