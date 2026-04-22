/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication with the Kalistat API",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in with an API token",
	Long: `Log in with an API token.

When stdin is a terminal, you are prompted and input is hidden.
When stdin is a pipe, the first line is read as the token — useful for scripts:

    echo "$KALISTAT_TOKEN" | kalistat auth login`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		token, err := readToken(cmd)
		if err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("no token provided")
		}
		if err := keychain.SetToken(token); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Logged in.")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Verify the stored token against the API",
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		entry, err := keychain.Get()
		if errors.Is(err, keychain.ErrNotFound) {
			fmt.Fprintln(out, "Not logged in.")
			return nil
		}
		if err != nil {
			return err
		}

		client, err := apiClient()
		if err != nil {
			return err
		}
		if _, err := client.GetJSON("/", nil); err != nil {
			fmt.Fprintln(out, describeStatusError(err))
			return errSilent
		}

		fmt.Fprintln(out, "Logged in.")
		if !entry.CreatedAt.IsZero() {
			fmt.Fprintf(out, "Token age: %s.\n", humanizeAge(time.Since(entry.CreatedAt)))
		}
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove the stored API token",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keychain.Clear(); err != nil {
			if errors.Is(err, keychain.ErrNotFound) {
				fmt.Fprintln(cmd.OutOrStdout(), "Not logged in.")
				return nil
			}
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Logged out.")
		return nil
	},
}

// describeStatusError picks a user-facing message for an `auth status` failure,
// distinguishing authentication problems from transport/server problems so the
// user doesn't chase the wrong remedy (e.g. regenerating a token that is fine).
func describeStatusError(err error) string {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return "Logged in, but the API is unreachable."
	}
	switch {
	case apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden:
		return "Logged in, but the token is not valid."
	case apiErr.StatusCode >= 500:
		return fmt.Sprintf("Logged in, but the API returned a server error (%d).", apiErr.StatusCode)
	default:
		return fmt.Sprintf("Logged in, but the API returned an unexpected status (%d).", apiErr.StatusCode)
	}
}

// readToken reads a token from the command's stdin. If stdin is a terminal
// the input is hidden (no echo) and a prompt is printed to stderr; otherwise
// a single line is read. This keeps tokens out of argv, /proc, and shell
// history while still supporting piped, non-interactive usage.
func readToken(cmd *cobra.Command) (string, error) {
	in := cmd.InOrStdin()
	if f, ok := in.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		fmt.Fprint(cmd.ErrOrStderr(), "Enter API token: ")
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(cmd.ErrOrStderr())
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	s := bufio.NewScanner(in)
	if !s.Scan() {
		if err := s.Err(); err != nil {
			return "", err
		}
		return "", nil
	}
	return strings.TrimSpace(s.Text()), nil
}

func init() {
	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}

func humanizeAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute", "minutes")
	case d < 24*time.Hour:
		return plural(int(d.Hours()), "hour", "hours")
	default:
		return plural(int(d.Hours()/24), "day", "days")
	}
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
