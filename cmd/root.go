/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "kalistat",
	Short:         "Kalistat CLI — explore and query Kalistat data sources",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// jsonOutput is bound to the --json persistent flag. Commands read this
// variable directly rather than looking the flag up by name, which keeps
// registration and access consistent across the whole tree.
var jsonOutput bool

// baseURL is bound to the --base-url persistent flag. Empty means
// "fall back to $KALISTAT_API_URL, then to the default".
var baseURL string

// errSilent signals "exit non-zero, but do not print anything else" — the
// command has already produced user-facing output.
var errSilent = errors.New("silent")

func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}
	if !errors.Is(err, errSilent) {
		printError(os.Stdout, os.Stderr, err, jsonOutput)
	}
	os.Exit(1)
}

// printError is exposed via its writers for testability.
func printError(stdout, stderr io.Writer, err error, jsonMode bool) {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && jsonMode {
		_, _ = stdout.Write(apiErr.Body)
		if !bytes.HasSuffix(apiErr.Body, []byte("\n")) {
			fmt.Fprintln(stdout)
		}
		return
	}
	fmt.Fprintf(stderr, "Error: %s\n", err)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output raw JSON response")
	rootCmd.PersistentFlags().StringVar(&baseURL, "base-url", "",
		"API base URL (overrides $KALISTAT_API_URL; default: "+api.DefaultBaseURL+")")
}
