/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"net/url"
	"sort"
	"text/tabwriter"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var datasetCmd = &cobra.Command{
	Use:   "dataset",
	Short: "Inspect dataset metadata and dimension values",
}

var datasetGetCmd = &cobra.Command{
	Use:   "get <code>",
	Short: "Show metadata for a dataset",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		code := args[0]
		if err := validateSegment("dataset", code); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		var resp api.DatasetResponse
		body, err := client.GetJSON("/datasets/"+url.PathEscape(code), nil, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printDataset(cmd, &resp.Data)
	},
}

var datasetValuesCmd = &cobra.Command{
	Use:   "values <code> <dim-key>",
	Short: "List allowed values for a dimension",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		code, dimKey := args[0], args[1]
		if err := validateSegment("dataset", code); err != nil {
			return err
		}
		if err := validateSegment("dimension key", dimKey); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		path := "/datasets/" + url.PathEscape(code) + "/dimensions/" + url.PathEscape(dimKey) + "/values"
		var resp api.DimensionValuesResponse
		body, err := client.GetJSON(path, nil, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printDimensionValues(cmd, dimKey, resp.Data)
	},
}

func printDataset(cmd *cobra.Command, d *api.Dataset) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s — %s\n", d.Code, d.Name)
	fmt.Fprintf(out, "Source:       %s\n", d.Source)
	if d.DataflowID != "" {
		fmt.Fprintf(out, "Dataflow:     %s\n", d.DataflowID)
	}
	if d.CategoryKey != "" {
		fmt.Fprintf(out, "Category:     %s\n", d.CategoryKey)
	}
	if d.SeriesCount > 0 {
		fmt.Fprintf(out, "Series count: %d\n", d.SeriesCount)
	}

	if len(d.Dimensions) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Dimensions:")
		dims := append([]api.Dimension(nil), d.Dimensions...)
		sort.Slice(dims, func(i, j int) bool { return dims[i].Position < dims[j].Position })
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  POS\tKEY\tLABEL")
		for _, dim := range dims {
			fmt.Fprintf(tw, "  %d\t%s\t%s\n", dim.Position, dim.Key, dim.Label)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}

	if len(d.TimeDimensions) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Time dimensions:")
		tdims := append([]api.TimeDimension(nil), d.TimeDimensions...)
		sort.Slice(tdims, func(i, j int) bool { return tdims[i].Position < tdims[j].Position })
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  POS\tKEY\tLABEL")
		for _, td := range tdims {
			fmt.Fprintf(tw, "  %d\t%s\t%s\n", td.Position, td.Key, td.Label)
		}
		if err := tw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func printDimensionValues(cmd *cobra.Command, dimKey string, values []api.DimensionValue) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Dimension: %s\n", dimKey)
	fmt.Fprintf(out, "Values: %d\n\n", len(values))
	if len(values) == 0 {
		return nil
	}

	// Only show the LEVEL column when the dimension actually has a tree.
	hasHierarchy := false
	for _, v := range values {
		if v.Level > 0 {
			hasHierarchy = true
			break
		}
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if hasHierarchy {
		fmt.Fprintln(tw, "CODE\tNAME\tLEVEL")
		for _, v := range values {
			fmt.Fprintf(tw, "%s\t%s\t%d\n", v.Code, v.Name, v.Level)
		}
	} else {
		fmt.Fprintln(tw, "CODE\tNAME")
		for _, v := range values {
			fmt.Fprintf(tw, "%s\t%s\n", v.Code, v.Name)
		}
	}
	return tw.Flush()
}

func init() {
	datasetCmd.AddCommand(datasetGetCmd, datasetValuesCmd)
	rootCmd.AddCommand(datasetCmd)
}
