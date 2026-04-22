package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/zalando/go-keyring"
)

func resetCmd(t *testing.T) *bytes.Buffer {
	t.Helper()
	keyring.MockInit()
	buf := &bytes.Buffer{}
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	return buf
}

func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	rootCmd.SetArgs(args)
	return rootCmd.Execute()
}

func TestAuthLogin_StoresTokenWithTimestamp(t *testing.T) {
	buf := resetCmd(t)

	if err := runCLI(t, "auth", "login", "secret"); err != nil {
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

func TestAuthLogin_RequiresTokenArg(t *testing.T) {
	resetCmd(t)

	if err := runCLI(t, "auth", "login"); err == nil {
		t.Fatal("expected error when token arg is missing")
	}
}

func TestAuthLogout_RemovesToken(t *testing.T) {
	buf := resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatalf("seed: %v", err)
	}

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
	buf := resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer secret"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{}}`))
	}))
	defer server.Close()
	t.Setenv("KALISTAT_API_URL", server.URL)

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

func TestAuthStatus_InvalidToken(t *testing.T) {
	buf := resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()
	t.Setenv("KALISTAT_API_URL", server.URL)

	if err := runCLI(t, "auth", "status"); err == nil {
		t.Fatal("expected error for invalid token")
	}
	if !strings.Contains(buf.String(), "not valid") {
		t.Errorf("output = %q, want to contain %q", buf.String(), "not valid")
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

func TestHumanizeAge(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5 minutes"},
		{2 * time.Hour, "2 hours"},
		{48 * time.Hour, "2 days"},
	}
	for _, c := range cases {
		if got := humanizeAge(c.d); got != c.want {
			t.Errorf("humanizeAge(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
