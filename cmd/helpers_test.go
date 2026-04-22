package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/zalando/go-keyring"
)

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"exactly-10", 10, "exactly-10"},
		{"longer-than-10", 10, "longer-th…"},
		// The TrimSpace is intentional: if the cut lands on a space, don't
		// leave a dangling one before the ellipsis.
		{"cut at space here", 14, "cut at space…"},
		{"", 5, ""},
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}

func TestSanitizeForTerminal(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain text unchanged", "hello world", "hello world"},
		{"keeps tab and newline", "a\tb\nc", "a\tb\nc"},
		{"strips ANSI CSI", "\x1b[2Jcleared", "[2Jcleared"},
		{"strips OSC", "\x1b]0;title\x07visible", "]0;titlevisible"},
		{"strips raw control chars", "before\x01\x02\x03after", "beforeafter"},
		{"strips DEL", "keep\x7fme", "keepme"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeForTerminal(c.in); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestAPIClient_NoToken(t *testing.T) {
	resetCmd(t)

	_, err := apiClient()
	if err == nil {
		t.Fatal("expected error when no token is stored")
	}
	if !strings.Contains(err.Error(), "auth login") {
		t.Errorf("error should point user to `auth login`, got %q", err)
	}
	if !errors.Is(err, keychain.ErrNotFound) {
		t.Errorf("err should wrap ErrNotFound for callers to detect; got %v", err)
	}
}

func TestAPIClient_CorruptedKeychainEntry(t *testing.T) {
	resetCmd(t)
	if err := keyring.Set("kalistat", "api-token", "not-json"); err != nil {
		t.Fatal(err)
	}

	_, err := apiClient()
	if err == nil {
		t.Fatal("expected error for corrupted keychain entry")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("error should mention malformed entry, got %q", err)
	}
}

func TestAPIClient_InvalidBaseURLRejected(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KALISTAT_API_URL", "http://attacker.com/v1")

	_, err := apiClient()
	if err == nil {
		t.Fatal("expected error: plain http on non-loopback host must be rejected")
	}
}

func TestAPIClient_SuccessPassesTokenAndBaseURL(t *testing.T) {
	resetCmd(t)
	if err := keychain.SetToken("secret"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KALISTAT_API_URL", "https://custom.example.com/api/v1")

	c, err := apiClient()
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != "secret" {
		t.Errorf("Token = %q", c.Token)
	}
	if !strings.HasPrefix(c.BaseURL, "https://custom.example.com") {
		t.Errorf("BaseURL = %q, want custom base", c.BaseURL)
	}
}
