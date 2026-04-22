/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

const cliVersion = "v1"

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show CLI and API information",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := apiClient()
		if err != nil {
			return err
		}
		var resp api.RootResponse
		body, err := client.GetJSON("/", nil, &resp)
		if err != nil {
			return err
		}

		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printInfo(cmd, resp.Data)
	},
}

func printInfo(cmd *cobra.Command, r api.Root) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Kalistat CLI %s\n", cliVersion)
	if r.Version != "" {
		fmt.Fprintf(out, "  Using API %s\n", r.Version)
	}
	if len(r.Sources) > 0 {
		fmt.Fprintf(out, "  Sources:    %s\n", strings.Join(r.Sources, ", "))
	}
	if r.RateLimit.RequestsPerMinute > 0 {
		fmt.Fprintf(out, "  Rate limit: %d requests/minute\n", r.RateLimit.RequestsPerMinute)
	}
	return nil
}

// writeRaw copies the server's JSON body verbatim to the command's stdout,
// adding a trailing newline if the body doesn't already have one.
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

func init() {
	rootCmd.AddCommand(infoCmd)
}
