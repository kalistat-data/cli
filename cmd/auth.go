/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/kalistat-data/cli/internal/keychain"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication with the Kalistat API",
}

var authLoginCmd = &cobra.Command{
	Use:   "login <token>",
	Short: "Log in with an API token",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := keychain.SetToken(args[0]); err != nil {
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

		client, err := api.New()
		if err != nil {
			return err
		}
		if _, err := client.GetJSON("/", nil); err != nil {
			fmt.Fprintln(out, "Logged in, but token is not valid.")
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

func init() {
	authCmd.AddCommand(authLoginCmd, authStatusCmd, authLogoutCmd)
	rootCmd.AddCommand(authCmd)
}

func humanizeAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours", int(d.Hours()))
	default:
		return fmt.Sprintf("%d days", int(d.Hours()/24))
	}
}
