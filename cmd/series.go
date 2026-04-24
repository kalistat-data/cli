/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"text/tabwriter"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var seriesCmd = &cobra.Command{
	Use:   "series",
	Short: "Resolve and fetch time series",
}

var seriesListCmd = &cobra.Command{
	Use:   "list <dataset> <pattern>",
	Short: "Resolve a ticker pattern into concrete series",
	Long: `Resolve a ticker pattern into concrete series.

Use '*' as a wildcard in any dimension position, e.g. A.*.TOT. Each matched
series is listed with its observation count and time range — use ` + "`series get`" + `
to fetch the observations for one of them.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, pattern := args[0], args[1]
		if err := validateSegment("dataset", dataset); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		path := "/datasets/" + url.PathEscape(dataset) + "/series"
		q := url.Values{"pattern": []string{pattern}}
		var resp api.SeriesListResponse
		body, err := client.GetJSON(path, q, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printSeriesList(cmd, pattern, &resp)
	},
}

var seriesGetCmd = &cobra.Command{
	Use:   "get <dataset> <series-code>",
	Short: "Fetch observations for a single series",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		dataset, seriesCode := args[0], args[1]
		if err := validateSegment("dataset", dataset); err != nil {
			return err
		}
		if err := validateSegment("series code", seriesCode); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		path := "/datasets/" + url.PathEscape(dataset) + "/series/" + url.PathEscape(seriesCode)
		var resp api.SeriesResponse
		body, err := client.GetJSON(path, nil, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printSeriesGet(cmd, &resp.Data)
	},
}

func printSeriesList(cmd *cobra.Command, pattern string, resp *api.SeriesListResponse) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Pattern: %s\n", sanitizeForTerminal(pattern))
	fmt.Fprintf(out, "Matched: %d series\n", len(resp.Data))
	if w := resp.Meta.Warning; w != nil {
		limit := w.Limit
		if limit == 0 {
			limit = len(resp.Data)
		}
		fmt.Fprintf(out, "Warning: results truncated to first %d matches — narrow the pattern to see more. (%s)\n",
			limit, sanitizeForTerminal(w.Code))
	}
	fmt.Fprintln(out)
	if len(resp.Data) == 0 {
		return nil
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TICKER\tOBSERVATIONS\tRANGE")
	for _, s := range resp.Data {
		from, to := observationRange(s.Values)
		fmt.Fprintf(tw, "%s\t%d\t%s\n", s.Ticker, len(s.Values), rangeString(from, to))
	}
	return tw.Flush()
}

func printSeriesGet(cmd *cobra.Command, s *api.Series) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Ticker: %s\n", s.Ticker)
	if len(s.Dimensions) > 0 {
		fmt.Fprintln(out, "Dimensions:")
		dims := make([]api.SeriesDimension, len(s.Dimensions))
		copy(dims, s.Dimensions)
		sort.Slice(dims, func(i, j int) bool { return dims[i].Position < dims[j].Position })
		dw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		for _, d := range dims {
			fmt.Fprintf(dw, "  %s (%s)\t%s\n", d.Label, d.Key, d.Value)
		}
		if err := dw.Flush(); err != nil {
			return err
		}
	}
	from, to := observationRange(s.Values)
	fmt.Fprintf(out, "Observations: %d", len(s.Values))
	if len(s.Values) > 0 {
		fmt.Fprintf(out, " (%s)", rangeString(from, to))
	}
	fmt.Fprintln(out)
	if len(s.Values) == 0 {
		return nil
	}
	fmt.Fprintln(out)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TIME\tVALUE")
	for _, obs := range s.Values {
		fmt.Fprintf(tw, "%s\t%s\n", obs.Time, formatValue(obs.Value))
	}
	return tw.Flush()
}

func observationRange(obs []api.Observation) (from, to string) {
	if len(obs) == 0 {
		return "", ""
	}
	return obs[0].Time, obs[len(obs)-1].Time
}

func rangeString(from, to string) string {
	if from == "" {
		return "—"
	}
	if from == to {
		return from
	}
	return from + " → " + to
}

func formatValue(v *float64) string {
	if v == nil {
		return "—"
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func init() {
	seriesCmd.AddCommand(seriesListCmd, seriesGetCmd)
	rootCmd.AddCommand(seriesCmd)
}
