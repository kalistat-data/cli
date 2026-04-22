/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	searchSource      string
	searchCategoryKey string
	searchPage        int
	searchPageSize    int
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search datasets (weighted full-text)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := strings.TrimSpace(args[0])
		if query == "" {
			return fmt.Errorf("search query cannot be empty")
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		var resp api.SearchResponse
		body, err := client.GetJSON("/search", searchQuery(query), &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printSearch(cmd, &resp)
	},
}

func searchQuery(q string) url.Values {
	v := url.Values{}
	v.Set("q", q)
	if searchSource != "" {
		v.Set("source", searchSource)
	}
	if searchCategoryKey != "" {
		v.Set("category_key", searchCategoryKey)
	}
	if searchPage > 0 {
		v.Set("page", strconv.Itoa(searchPage))
	}
	if searchPageSize > 0 {
		v.Set("page_size", strconv.Itoa(searchPageSize))
	}
	return v
}

func printSearch(cmd *cobra.Command, resp *api.SearchResponse) error {
	out := cmd.OutOrStdout()
	if len(resp.Data) == 0 {
		fmt.Fprintln(out, "No matches.")
		return nil
	}
	if p := resp.Meta.Pagination; p != nil {
		start := (p.Page-1)*p.PageSize + 1
		end := start + len(resp.Data) - 1
		fmt.Fprintf(out, "Showing %d-%d of %d match%s.\n\n", start, end, p.Total, pluralSuffix(p.Total, "", "es"))
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CODE\tNAME\tSOURCE\tCATEGORY\tCATEGORY KEY")
	for _, h := range resp.Data {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			h.Code, truncate(h.Name, 60), h.Source, truncate(h.Category.Name, 40), h.Category.Key)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if p := resp.Meta.Pagination; p != nil && p.HasMore {
		fmt.Fprintf(out, "\nMore results — use --page %d to fetch the next page.\n", p.Page+1)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return strings.TrimSpace(s[:n-1]) + "…"
}

func pluralSuffix(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func init() {
	searchCmd.Flags().StringVar(&searchSource, "source", "", "Filter by source: istat or eurostat")
	searchCmd.Flags().StringVar(&searchCategoryKey, "category-key", "", "Restrict to a category subtree")
	searchCmd.Flags().IntVar(&searchPage, "page", 0, "Page number (default 1)")
	searchCmd.Flags().IntVar(&searchPageSize, "page-size", 0, "Page size (default 50, max 200)")
	rootCmd.AddCommand(searchCmd)
}
