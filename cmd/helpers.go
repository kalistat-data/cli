/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"os"
	"regexp"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/kalistat-data/cli/internal/keychain"
)

// validPathSegment matches identifiers used as URL path segments (dataset
// codes, series codes, dimension keys). It deliberately forbids `.`/`..`
// and slashes so a user-supplied value can't walk the URL path after
// url.URL.JoinPath runs path.Clean on the constructed URL.
var validPathSegment = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func validateSegment(name, value string) error {
	if !validPathSegment.MatchString(value) {
		return fmt.Errorf("%s %q contains invalid characters (allowed: letters, digits, '.', '-', '_'; must not be empty or start with a symbol)", name, value)
	}
	return nil
}

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
