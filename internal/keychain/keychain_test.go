package keychain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

func fresh(t *testing.T) {
	t.Helper()
	keyring.MockInit()
}

func TestSetAndGet_Roundtrip(t *testing.T) {
	fresh(t)

	before := time.Now().UTC()
	if err := SetToken("sometoken"); err != nil {
		t.Fatal(err)
	}
	entry, err := Get()
	if err != nil {
		t.Fatal(err)
	}
	if entry.Token != "sometoken" {
		t.Errorf("Token = %q, want %q", entry.Token, "sometoken")
	}
	if entry.CreatedAt.Before(before) || entry.CreatedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Errorf("CreatedAt out of expected window: %s", entry.CreatedAt)
	}
}

func TestGet_WhenEmptyReturnsErrNotFound(t *testing.T) {
	fresh(t)

	_, err := Get()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGet_MalformedJSONReturnsError(t *testing.T) {
	fresh(t)
	if err := keyring.Set(service, user, "definitely not json"); err != nil {
		t.Fatal(err)
	}

	_, err := Get()
	if err == nil {
		t.Fatal("expected error for non-JSON entry")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("error = %q, want to mention 'malformed'", err)
	}
}

func TestGet_EmptyTokenInEnvelopeReturnsError(t *testing.T) {
	fresh(t)
	if err := keyring.Set(service, user, `{"token":"","created_at":"2026-01-01T00:00:00Z"}`); err != nil {
		t.Fatal(err)
	}

	_, err := Get()
	if err == nil {
		t.Fatal("expected error for empty-token envelope")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("error = %q, want to mention 'malformed'", err)
	}
}

func TestGetToken_PropagatesGetError(t *testing.T) {
	fresh(t)

	if _, err := GetToken(); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGetToken_HappyPath(t *testing.T) {
	fresh(t)
	if err := SetToken("abc"); err != nil {
		t.Fatal(err)
	}
	got, err := GetToken()
	if err != nil {
		t.Fatal(err)
	}
	if got != "abc" {
		t.Errorf("GetToken = %q, want %q", got, "abc")
	}
}

func TestClear_WhenEmptyReturnsErrNotFound(t *testing.T) {
	fresh(t)

	err := Clear()
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestClear_RemovesStoredEntry(t *testing.T) {
	fresh(t)
	if err := SetToken("something"); err != nil {
		t.Fatal(err)
	}

	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := Get(); !errors.Is(err, ErrNotFound) {
		t.Errorf("after Clear, Get err = %v, want ErrNotFound", err)
	}
}
