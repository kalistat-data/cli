/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/spf13/cobra"
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

// writeRaw copies the server's JSON body verbatim to the command's stdout,
// adding a trailing newline if the body doesn't already have one. Used by
// every command to honour the --json flag.
func writeRaw(cmd *cobra.Command, body []byte) error {
	out := cmd.OutOrStdout()
	if _, err := out.Write(body); err != nil {
		return err
	}
	if !bytes.HasSuffix(body, []byte("\n")) {
		_, err := fmt.Fprintln(out)
		return err
	}
	return nil
}

// plural formats a count with the right noun form.
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralForm)
}

// sanitizeForTerminal drops ASCII/Unicode control characters from s (except
// tab and newline) so user-supplied strings echoed to stdout can't inject
// ANSI escape sequences, reposition the cursor, or trigger OSC handlers.
func sanitizeForTerminal(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\t' || r == '\n' {
			b.WriteRune(r)
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// truncate caps a string at n bytes and appends an ellipsis if it was cut.
// Used for compact tabular display of long names.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}

// printPaginationHeader writes "Showing X-Y of N items." when pagination
// metadata is present and the current page has items. `dataLen` is the
// number of items rendered on this page.
func printPaginationHeader(out io.Writer, p *api.Pagination, dataLen int, singular, pluralForm string) {
	if p == nil || dataLen == 0 {
		return
	}
	start := (p.Page-1)*p.PageSize + 1
	end := start + dataLen - 1
	fmt.Fprintf(out, "Showing %d-%d of %s.\n\n", start, end, plural(p.Total, singular, pluralForm))
}

// printPaginationFooter writes the "More results — use --page N" hint when
// there are additional pages, and is a no-op otherwise.
func printPaginationFooter(out io.Writer, p *api.Pagination) {
	if p == nil || !p.HasMore {
		return
	}
	fmt.Fprintf(out, "\nMore results — use --page %d to fetch the next page.\n", p.Page+1)
}
