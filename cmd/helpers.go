/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"os"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/kalistat-data/cli/internal/keychain"
)

// apiClient resolves the stored token and builds an authenticated client.
// Commands use this instead of an api.New()-style factory, keeping the
// keychain dependency in the cmd layer where auth concerns already live.
func apiClient() (*api.Client, error) {
	token, err := keychain.GetToken()
	if err != nil {
		return nil, fmt.Errorf("no API token found — run `kalistat auth login` first: %w", err)
	}
	return api.NewWithToken(token, os.Getenv("KALISTAT_API_URL"))
}
