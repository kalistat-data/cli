package keychain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	service = "kalistat"
	user    = "api-token"
)

// ErrNotFound is returned when no token is stored.
var ErrNotFound = keyring.ErrNotFound

type Entry struct {
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

func SetToken(token string) error {
	entry := Entry{Token: token, CreatedAt: time.Now().UTC()}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return keyring.Set(service, user, string(data))
}

func Get() (Entry, error) {
	raw, err := keyring.Get(service, user)
	if err != nil {
		return Entry{}, err
	}
	var entry Entry
	if err := json.Unmarshal([]byte(raw), &entry); err != nil {
		return Entry{}, fmt.Errorf("stored credentials are malformed; run `kalistat auth logout` and log in again: %w", err)
	}
	if entry.Token == "" {
		return Entry{}, fmt.Errorf("stored credentials are malformed; run `kalistat auth logout` and log in again")
	}
	return entry, nil
}

func GetToken() (string, error) {
	entry, err := Get()
	if err != nil {
		return "", err
	}
	return entry.Token, nil
}

func Clear() error {
	err := keyring.Delete(service, user)
	if errors.Is(err, keyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
