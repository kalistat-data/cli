/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"io"

	"github.com/kalistat-data/cli/internal/api"
)

type treeGlyphs struct {
	branch     string // connector for a non-last sibling, e.g. "├── "
	lastBranch string // connector for the last sibling,    e.g. "└── "
	vertical   string // prefix segment under a non-last,   e.g. "│   "
	space      string // prefix segment under the last,     e.g. "    "
}

var (
	unicodeGlyphs = treeGlyphs{branch: "├── ", lastBranch: "└── ", vertical: "│   ", space: "    "}
	asciiGlyphs   = treeGlyphs{branch: "|-- ", lastBranch: "`-- ", vertical: "|   ", space: "    "}
)

// renderTree prints one or more category roots and their descendants. Each
// root is printed at column 0 (no connector); children get box-drawing
// connectors whose prefix accumulates as we descend.
func renderTree(out io.Writer, roots []api.Category, g treeGlyphs, withDatasets bool) {
	for _, root := range roots {
		fmt.Fprintln(out, categoryLabel(root))
		renderChildren(out, &root, "", g, withDatasets)
	}
}

func renderChildren(out io.Writer, parent *api.Category, prefix string, g treeGlyphs, withDatasets bool) {
	items := childItems(parent, withDatasets)
	for i, item := range items {
		isLast := i == len(items)-1
		conn := g.branch
		childPfx := g.vertical
		if isLast {
			conn = g.lastBranch
			childPfx = g.space
		}
		fmt.Fprintln(out, prefix+conn+item.label)
		if item.category != nil {
			renderChildren(out, item.category, prefix+childPfx, g, withDatasets)
		}
	}
}

// treeItem is a renderable row under a category — either a child category
// (that may recurse) or a dataset stub (a terminal leaf when --with-datasets).
type treeItem struct {
	label    string
	category *api.Category // non-nil if this item can have descendants
}

func childItems(parent *api.Category, withDatasets bool) []treeItem {
	items := make([]treeItem, 0, len(parent.Children)+len(parent.Datasets))
	for i := range parent.Children {
		child := &parent.Children[i]
		items = append(items, treeItem{label: categoryLabel(*child), category: child})
	}
	if withDatasets {
		for _, ds := range parent.Datasets {
			items = append(items, treeItem{label: datasetStubLabel(ds)})
		}
	}
	return items
}

// categoryLabel formats a category node for display. A truncated subtree is
// hinted with an ellipsis when the node advertises children the server did
// not embed (e.g. depth cap was reached).
func categoryLabel(c api.Category) string {
	hint := ""
	if c.HasChildren && len(c.Children) == 0 {
		hint = " …"
	}
	return fmt.Sprintf("%s  %s%s", c.Key, c.Name, hint)
}

func datasetStubLabel(ds api.DatasetStub) string {
	return fmt.Sprintf("[dataset] %s  %s", ds.Code, ds.Name)
}
