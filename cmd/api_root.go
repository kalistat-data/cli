/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"encoding/json"
	"os"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var rootEndpointCmd = &cobra.Command{
	Use:   "root",
	Short: "Show API root information (GET /)",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		var resp any
		if err := client.GetJSON("/", &resp); err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(resp)
	},
}

func init() {
	rootCmd.AddCommand(rootEndpointCmd)
}

