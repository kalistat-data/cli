/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "List available data sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.New()
		if err != nil {
			return err
		}
		var resp api.SourcesResponse
		body, err := client.GetJSON("/sources", &resp)
		if err != nil {
			return err
		}

		jsonMode, _ := cmd.Flags().GetBool("json")
		if jsonMode {
			return writeRaw(cmd, body)
		}
		return printSources(cmd, resp.Data)
	},
}

func printSources(cmd *cobra.Command, sources []api.Source) error {
	out := cmd.OutOrStdout()
	if len(sources) == 0 {
		fmt.Fprintln(out, "No sources available.")
		return nil
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNAME\tCOUNTRY\tKEY")
	for _, s := range sources {
		country := "—"
		if s.Country != nil && *s.Country != "" {
			country = *s.Country
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", s.ID, s.Name, country, s.RootKey)
	}
	return tw.Flush()
}

func init() {
	rootCmd.AddCommand(sourcesCmd)
}
