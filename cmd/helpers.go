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

// resolveBaseURL picks the effective API base URL. Precedence: --base-url
// flag > KALISTAT_API_URL env var > api.DefaultBaseURL (selected by
// NewWithToken when the argument is empty).
func resolveBaseURL() string {
	if baseURL != "" {
		return baseURL
	}
	return os.Getenv("KALISTAT_API_URL")
}

// apiClient resolves the stored token and builds an authenticated client.
// Commands use this instead of an api.New()-style factory, keeping the
// keychain dependency in the cmd layer where auth concerns already live.
func apiClient() (*api.Client, error) {
	token, err := keychain.GetToken()
	if err != nil {
		return nil, fmt.Errorf("no API token found — run `kalistat auth login` first: %w", err)
	}
	return api.NewWithToken(token, resolveBaseURL())
}
