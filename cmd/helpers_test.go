package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/zalando/go-keyring"
)

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
