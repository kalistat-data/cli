/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"io"
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

var datasetAncestorsCmd = &cobra.Command{
	Use:   "ancestors <code>",
	Short: "Print the breadcrumb trail from root to the dataset's parent category",
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
		var resp api.AncestorsResponse
		body, err := client.GetJSON("/datasets/"+url.PathEscape(code)+"/ancestors", nil, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		g := unicodeGlyphs
		if treeASCII {
			g = asciiGlyphs
		}
		return printAncestors(cmd.OutOrStdout(), resp.Data, g)
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

	if err := printDimensionTable(out, "Dimensions:", d.Dimensions); err != nil {
		return err
	}
	if err := printDimensionTable(out, "Time dimensions:", d.TimeDimensions); err != nil {
		return err
	}
	return nil
}

// printDimensionTable renders a titled, position-sorted table of dimensions.
// No-op when dims is empty, so callers don't need to guard the call.
//
// If any dimension carries a FixedValue the table grows a FIXED VALUE column
// so users can tell at a glance which dimensions are pinned to a single code
// (and therefore need no wildcard in `series list` patterns). When no
// dimension is pinned the 3-column layout is preserved byte-for-byte.
func printDimensionTable(out io.Writer, title string, dims []api.Dimension) error {
	if len(dims) == 0 {
		return nil
	}
	sorted := append([]api.Dimension(nil), dims...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Position < sorted[j].Position })

	hasFixed := false
	for _, d := range sorted {
		if d.FixedValue != nil {
			hasFixed = true
			break
		}
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, title)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if hasFixed {
		fmt.Fprintln(tw, "  POS\tKEY\tLABEL\tFIXED VALUE")
		for _, d := range sorted {
			fv := ""
			if d.FixedValue != nil {
				fv = fmt.Sprintf("%s (%s)", d.FixedValue.Code, d.FixedValue.Name)
			}
			fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\n", d.Position, d.Key, d.Label, fv)
		}
	} else {
		fmt.Fprintln(tw, "  POS\tKEY\tLABEL")
		for _, d := range sorted {
			fmt.Fprintf(tw, "  %d\t%s\t%s\n", d.Position, d.Key, d.Label)
		}
	}
	return tw.Flush()
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
	// --ascii shares the same package-level var as `category tree` / `category
	// ancestors`. Cobra only parses one subcommand per invocation so there's
	// no risk of cross-contamination.
	datasetAncestorsCmd.Flags().BoolVar(&treeASCII, "ascii", false, "Use ASCII connectors instead of Unicode box-drawing")

	datasetCmd.AddCommand(datasetGetCmd, datasetAncestorsCmd, datasetValuesCmd)
	rootCmd.AddCommand(datasetCmd)
}
