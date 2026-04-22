/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	treeDepth        int
	treeSource       string
	treeWithDatasets bool
	treeASCII        bool

	catDatasetsRecursive bool
	catDatasetsPage      int
	catDatasetsPageSize  int
)

// resetCategoryFlags is called from tests (via resetCmd) to prevent
// one test's flag values from leaking into the next.
func resetCategoryFlags() {
	treeDepth = 0
	treeSource = ""
	treeWithDatasets = false
	treeASCII = false
	catDatasetsRecursive = false
	catDatasetsPage = 0
	catDatasetsPageSize = 0
}

var categoryCmd = &cobra.Command{
	Use:   "category",
	Short: "Navigate the category tree",
}

var categoryTreeCmd = &cobra.Command{
	Use:   "tree [<key>]",
	Short: "Render the category tree",
	Long: `Render the category tree as a visual tree.

With no key, shows all roots across sources (or just one source if --source is
given). With a key, shows the subtree rooted at that category down to --depth
levels. Use --with-datasets to embed dataset stubs under their categories.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		depth := treeDepth
		if depth < 1 {
			depth = 2
		}
		if depth > 5 {
			return fmt.Errorf("--depth must be between 1 and 5 (API limit)")
		}

		client, err := apiClient()
		if err != nil {
			return err
		}

		var (
			roots   []api.Category
			rawBody []byte
		)

		if len(args) == 0 {
			listBody, listResp, err := fetchRoots(client, treeSource)
			if err != nil {
				return err
			}
			roots = listResp.Data
			rawBody = listBody

			if depth > 1 {
				for i, root := range roots {
					_, subResp, err := fetchSubtree(client, root.Key, depth, treeWithDatasets)
					if err != nil {
						return err
					}
					roots[i] = subResp.Data
				}
			}
		} else {
			key := args[0]
			if err := validateSegment("category", key); err != nil {
				return err
			}
			body, sub, err := fetchSubtree(client, key, depth, treeWithDatasets)
			if err != nil {
				return err
			}
			roots = []api.Category{sub.Data}
			rawBody = body
		}

		if jsonOutput {
			// Only a single-request call can faithfully pass through raw.
			// Multi-call (no-key + depth>1) doesn't have a meaningful single body.
			if len(args) == 1 || depth <= 1 {
				return writeRaw(cmd, rawBody)
			}
			return fmt.Errorf("--json with --depth > 1 requires a <key>; run with a key or reduce --depth")
		}

		g := unicodeGlyphs
		if treeASCII {
			g = asciiGlyphs
		}
		renderTree(cmd.OutOrStdout(), roots, g, treeWithDatasets)
		return nil
	},
}

var categoryGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Show a category node and its direct children",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if err := validateSegment("category", key); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		var resp api.CategoryResponse
		body, err := client.GetJSON("/categories/"+url.PathEscape(key), nil, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printCategoryDetail(cmd, &resp.Data)
	},
}

var categoryAncestorsCmd = &cobra.Command{
	Use:   "ancestors <key>",
	Short: "Print the breadcrumb trail from root to the given category",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if err := validateSegment("category", key); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		var resp api.AncestorsResponse
		body, err := client.GetJSON("/categories/"+url.PathEscape(key)+"/ancestors", nil, &resp)
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

var categoryDatasetsCmd = &cobra.Command{
	Use:   "datasets <key>",
	Short: "List datasets attached to a category",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if err := validateSegment("category", key); err != nil {
			return err
		}
		client, err := apiClient()
		if err != nil {
			return err
		}
		q := url.Values{}
		if catDatasetsRecursive {
			q.Set("recursive", "true")
		}
		if catDatasetsPage > 0 {
			q.Set("page", strconv.Itoa(catDatasetsPage))
		}
		if catDatasetsPageSize > 0 {
			q.Set("page_size", strconv.Itoa(catDatasetsPageSize))
		}
		if len(q) == 0 {
			q = nil
		}
		var resp api.CategoryDatasetsResponse
		body, err := client.GetJSON("/categories/"+url.PathEscape(key)+"/datasets", q, &resp)
		if err != nil {
			return err
		}
		if jsonOutput {
			return writeRaw(cmd, body)
		}
		return printCategoryDatasets(cmd, key, &resp)
	},
}

func fetchRoots(client *api.Client, source string) ([]byte, *api.CategoriesListResponse, error) {
	var q url.Values
	if source != "" {
		q = url.Values{"source": []string{source}}
	}
	var resp api.CategoriesListResponse
	body, err := client.GetJSON("/categories", q, &resp)
	return body, &resp, err
}

func fetchSubtree(client *api.Client, key string, depth int, withDatasets bool) ([]byte, *api.CategoryResponse, error) {
	q := url.Values{"depth": []string{strconv.Itoa(depth)}}
	if withDatasets {
		q.Set("include", "datasets")
	}
	var resp api.CategoryResponse
	body, err := client.GetJSON("/categories/"+url.PathEscape(key)+"/subtree", q, &resp)
	return body, &resp, err
}

func printCategoryDetail(cmd *cobra.Command, c *api.Category) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s — %s\n", c.Key, c.Name)
	fmt.Fprintf(out, "Source:       %s\n", c.Source)
	fmt.Fprintf(out, "Has children: %s\n", yesNo(c.HasChildren))
	fmt.Fprintf(out, "Has datasets: %s\n", yesNo(c.HasDatasets))
	if len(c.Children) > 0 {
		fmt.Fprintln(out)
		fmt.Fprintf(out, "Children (%d):\n", len(c.Children))
		tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "  KEY\tNAME\tHAS-CHILDREN\tHAS-DATASETS")
		for _, child := range c.Children {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", child.Key, child.Name, yesNo(child.HasChildren), yesNo(child.HasDatasets))
		}
		return tw.Flush()
	}
	return nil
}

// printAncestors renders the breadcrumb chain using tree-style connectors.
// Glyphs are passed in so the function has no hidden dependency on package
// state — matching the renderTree(out, roots, g, withDatasets) shape.
func printAncestors(out io.Writer, ancestors []api.Ancestor, g treeGlyphs) error {
	if len(ancestors) == 0 {
		fmt.Fprintln(out, "No ancestors.")
		return nil
	}
	// Breadcrumbs form a single chain (every node is the only child of its
	// parent), so every connector is the last-sibling glyph.
	for i, a := range ancestors {
		marker := ""
		if a.Depth == 0 {
			marker = "> "
		}
		if i == 0 {
			fmt.Fprintf(out, "%s%s  %s\n", marker, a.Key, a.Name)
			continue
		}
		prefix := strings.Repeat(g.space, i-1) + g.lastBranch
		fmt.Fprintf(out, "%s%s%s  %s\n", prefix, marker, a.Key, a.Name)
	}
	return nil
}

func printCategoryDatasets(cmd *cobra.Command, key string, resp *api.CategoryDatasetsResponse) error {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Category: %s\n", key)
	if len(resp.Data) == 0 {
		fmt.Fprintln(out, "Datasets: 0")
		return nil
	}
	if resp.Meta.Pagination == nil {
		fmt.Fprintf(out, "Datasets: %d\n\n", len(resp.Data))
	}
	printPaginationHeader(out, resp.Meta.Pagination, len(resp.Data), "dataset", "datasets")
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "CODE\tNAME\tSOURCE\tCATEGORY")
	for _, ds := range resp.Data {
		cat := ds.CategoryKey
		if cat == "" {
			cat = "—"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", ds.Code, truncate(ds.Name, 60), ds.Source, cat)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	printPaginationFooter(out, resp.Meta.Pagination)
	return nil
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func init() {
	categoryTreeCmd.Flags().IntVar(&treeDepth, "depth", 0, "Depth (1-5). Default 2.")
	categoryTreeCmd.Flags().StringVar(&treeSource, "source", "", "Filter roots by source: istat or eurostat (ignored when <key> is given)")
	categoryTreeCmd.Flags().BoolVar(&treeWithDatasets, "with-datasets", false, "Embed dataset stubs under categories that hold them")
	// --ascii is only meaningful for the two commands that render a tree;
	// registering it on the parent would pollute help for `get` / `datasets`.
	// Sharing the same package-level var across both is safe because cobra
	// only parses one subcommand per invocation.
	categoryTreeCmd.Flags().BoolVar(&treeASCII, "ascii", false, "Use ASCII connectors instead of Unicode box-drawing")
	categoryAncestorsCmd.Flags().BoolVar(&treeASCII, "ascii", false, "Use ASCII connectors instead of Unicode box-drawing")

	categoryDatasetsCmd.Flags().BoolVar(&catDatasetsRecursive, "recursive", false, "Include datasets from descendant categories")
	categoryDatasetsCmd.Flags().IntVar(&catDatasetsPage, "page", 0, "Page number (default 1)")
	categoryDatasetsCmd.Flags().IntVar(&catDatasetsPageSize, "page-size", 0, "Page size (default 50, max 200)")

	categoryCmd.AddCommand(categoryTreeCmd, categoryGetCmd, categoryAncestorsCmd, categoryDatasetsCmd)
	rootCmd.AddCommand(categoryCmd)
}
