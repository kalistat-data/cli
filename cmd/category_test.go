package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kalistat-data/cli/internal/api"
)

const rootsJSON = `{
  "data": [
    {"source":"eurostat","key":"EU","name":"EUROSTAT","has_children":true,"has_datasets":false,
      "children":[{"source":"eurostat","key":"EU.5","name":"Labor market","has_children":true,"has_datasets":false}]},
    {"source":"istat","key":"IT","name":"Italy","has_children":true,"has_datasets":false,
      "children":[{"source":"istat","key":"IT.5","name":"Labor market","has_children":true,"has_datasets":false}]}
  ],
  "meta":{"generated_at":"2026-04-22T20:00:00Z"}
}`

const subtreeIT5JSON = `{
  "data": {
    "source":"istat","key":"IT.5","name":"Labor market","has_children":true,"has_datasets":false,
    "children":[
      {"source":"istat","key":"IT.5.1","name":"Activity","has_children":true,"has_datasets":false,
        "children":[{"source":"istat","key":"IT.5.1.1","name":"Monthly","has_children":false,"has_datasets":true}]},
      {"source":"istat","key":"IT.5.2","name":"Employment","has_children":true,"has_datasets":false}
    ]
  },
  "meta":{"generated_at":"2026-04-22T20:00:00Z"}
}`

const subtreeWithDatasetsJSON = `{
  "data": {
    "source":"istat","key":"IT.5.2.1","name":"Employed","has_children":false,"has_datasets":true,
    "datasets":[
      {"code":"IT.LAMA.131","name":"By professional status","source":"istat","category_key":"IT.5.2.1"},
      {"code":"IT.LAMA.132","name":"Main indicators","source":"istat","category_key":"IT.5.2.1"}
    ]
  },
  "meta":{}
}`

const categoryGetJSON = `{
  "data":{
    "source":"istat","key":"IT.5","name":"Labor market","has_children":true,"has_datasets":false,
    "children":[
      {"source":"istat","key":"IT.5.1","name":"Activity","has_children":true,"has_datasets":false},
      {"source":"istat","key":"IT.5.2","name":"Employment","has_children":true,"has_datasets":false}
    ]
  },
  "meta":{}
}`

const ancestorsJSON = `{
  "data":[
    {"key":"IT","name":"Italy","depth":-3},
    {"key":"IT.5","name":"Labor market","depth":-2},
    {"key":"IT.5.2","name":"Employment","depth":-1},
    {"key":"IT.5.2.1","name":"Employed - monthly data","depth":0}
  ],
  "meta":{}
}`

const categoryDatasetsJSON = `{
  "data":[
    {"code":"IT.LAMA.131","name":"By professional status","source":"istat","category_key":"IT.5.2.1"},
    {"code":"IT.LAMA.132","name":"Main indicators","source":"istat","category_key":"IT.5.2.1"}
  ],
  "meta":{"pagination":{"total":2,"page":1,"page_size":50,"has_more":false},"generated_at":"2026-04-22T20:00:00Z"}
}`

// ---------- tree ----------

func TestCategoryTree_NoKeyUsesCategoriesEndpointDepth1(t *testing.T) {
	buf := loggedIn(t)
	var path, query string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rootsJSON))
	})

	if err := runCLI(t, "category", "tree", "--depth", "1"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/categories") {
		t.Errorf("path = %q, want /categories", path)
	}
	if query != "" {
		t.Errorf("depth=1 with no source should produce no query string; got %q", query)
	}
	out := buf.String()
	// Both roots and their depth-1 children must be rendered. Each root has
	// exactly one child in the fixture, so the connector is the last-sibling
	// glyph (└──) for both.
	for _, want := range []string{"EU  EUROSTAT", "└── EU.5", "IT  Italy", "└── IT.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestCategoryTree_NoKeyWithSourceFilter(t *testing.T) {
	loggedIn(t)
	var query string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(rootsJSON))
	})

	if err := runCLI(t, "category", "tree", "--depth", "1", "--source", "istat"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "source=istat") {
		t.Errorf("query = %q, want source=istat", query)
	}
}

func TestCategoryTree_WithKeyCallsSubtree(t *testing.T) {
	loggedIn(t)
	var path, query string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(subtreeIT5JSON))
	})

	if err := runCLI(t, "category", "tree", "IT.5", "--depth", "3"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/categories/IT.5/subtree") {
		t.Errorf("path = %q", path)
	}
	if !strings.Contains(query, "depth=3") {
		t.Errorf("query %q missing depth=3", query)
	}
}

func TestCategoryTree_WithDatasetsAddsIncludeFlag(t *testing.T) {
	buf := loggedIn(t)
	var query string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(subtreeWithDatasetsJSON))
	})

	if err := runCLI(t, "category", "tree", "IT.5.2.1", "--with-datasets"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "include=datasets") {
		t.Errorf("query %q missing include=datasets", query)
	}
	out := buf.String()
	for _, want := range []string{"IT.5.2.1", "[dataset] IT.LAMA.131", "[dataset] IT.LAMA.132"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestCategoryTree_ASCIIFlagSwitchesGlyphs(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, subtreeIT5JSON, 0)

	if err := runCLI(t, "category", "tree", "IT.5", "--ascii"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "├│└") {
		t.Errorf("ASCII mode should not emit box-drawing glyphs:\n%s", out)
	}
	if !strings.Contains(out, "|-- ") && !strings.Contains(out, "`-- ") {
		t.Errorf("ASCII mode should use |-- and `-- connectors:\n%s", out)
	}
}

func TestCategoryTree_EllipsisHintWhenChildrenTruncated(t *testing.T) {
	buf := loggedIn(t)
	// Category claims has_children but embeds none — depth cap hit.
	mockJSONAPI(t, `{"data":{"source":"istat","key":"IT.5","name":"Labor","has_children":true,"has_datasets":false,"children":[]},"meta":{}}`, 0)

	if err := runCLI(t, "category", "tree", "IT.5", "--depth", "1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "…") {
		t.Errorf("output should hint at truncated subtree with an ellipsis:\n%s", buf.String())
	}
}

func TestCategoryTree_NoKeyDepth2FetchesSubtreePerRoot(t *testing.T) {
	loggedIn(t)
	var calls atomic.Int32
	var paths []string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/categories"):
			_, _ = w.Write([]byte(rootsJSON))
		case strings.Contains(r.URL.Path, "/subtree"):
			_, _ = w.Write([]byte(subtreeIT5JSON))
		default:
			http.NotFound(w, r)
		}
	})

	if err := runCLI(t, "category", "tree", "--depth", "2"); err != nil {
		t.Fatal(err)
	}
	// 1 call to /categories + 1 subtree call per root (EU + IT = 2).
	if got := calls.Load(); got != 3 {
		t.Errorf("call count = %d, want 3 (1 roots + 2 subtrees). paths=%v", got, paths)
	}
}

func TestCategoryTree_DepthValidation(t *testing.T) {
	loggedIn(t)
	// Server shouldn't be hit for out-of-range depths.
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for invalid depth; got %s", r.URL)
	})

	if err := runCLI(t, "category", "tree", "IT.5", "--depth", "6"); err == nil {
		t.Fatal("expected error for depth > 5")
	}
}

func TestCategoryTree_JSONPassthroughSingleCall(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, subtreeIT5JSON, 0)

	if err := runCLI(t, "category", "tree", "IT.5", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
}

func TestCategoryTree_JSONWithMultipleCallsRejected(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t, rootsJSON, 0)

	// no key + depth > 1 produces N requests — no single body to pass through.
	err := runCLI(t, "category", "tree", "--depth", "2", "--json")
	if err == nil {
		t.Fatal("expected error when --json combined with multi-call tree")
	}
}

// ---------- get ----------

func TestCategoryGet_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var path string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(categoryGetJSON))
	})

	if err := runCLI(t, "category", "get", "IT.5"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/categories/IT.5") {
		t.Errorf("path = %q", path)
	}
}

func TestCategoryGet_PrettyOutput(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, categoryGetJSON, 0)

	if err := runCLI(t, "category", "get", "IT.5"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"IT.5 — Labor market",
		"Source:       istat",
		"Has children: yes",
		"Has datasets: no",
		"Children (2):",
		"IT.5.1", "Activity",
		"IT.5.2", "Employment",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

// ---------- ancestors ----------

func TestCategoryAncestors_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var path string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ancestorsJSON))
	})

	if err := runCLI(t, "category", "ancestors", "IT.5.2.1"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/categories/IT.5.2.1/ancestors") {
		t.Errorf("path = %q", path)
	}
}

func TestCategoryAncestors_RendersTreeConnectors(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, ancestorsJSON, 0)

	if err := runCLI(t, "category", "ancestors", "IT.5.2.1"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Target must be flagged with ">".
	if !strings.Contains(out, "> IT.5.2.1") {
		t.Errorf("output should mark target with '>':\n%s", out)
	}
	// Non-target entries should not have the arrow marker.
	if strings.Contains(out, "> IT.5.2  ") || strings.Contains(out, "> IT  ") {
		t.Errorf("non-target entry incorrectly marked as target:\n%s", out)
	}
	// Every non-root line should use the last-sibling connector (ancestors
	// form a single chain with no siblings).
	if !strings.Contains(out, "└── IT.5  ") {
		t.Errorf("missing tree connector at IT.5:\n%s", out)
	}
	// Each step deeper should shift right by exactly len(g.space)=4 columns.
	var prev int
	for i, k := range []string{"IT  ", "IT.5  ", "IT.5.2  ", "IT.5.2.1  "} {
		idx := strings.Index(out, k)
		if idx < 0 {
			t.Fatalf("line for %q not found:\n%s", k, out)
		}
		lineStart := strings.LastIndex(out[:idx], "\n") + 1
		col := idx - lineStart
		if i > 0 && col <= prev {
			t.Errorf("indent of %q (col %d) should exceed previous (%d)", k, col, prev)
		}
		prev = col
	}
}

func TestCategoryAncestors_ASCIIFlag(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, ancestorsJSON, 0)

	if err := runCLI(t, "category", "ancestors", "IT.5.2.1", "--ascii"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "└├│") {
		t.Errorf("ASCII mode should not emit box-drawing glyphs:\n%s", out)
	}
	if !strings.Contains(out, "`-- IT.5  ") {
		t.Errorf("ASCII mode should use `-- connector:\n%s", out)
	}
}

// ---------- datasets ----------

func TestCategoryDatasets_BuildsCorrectURLAndQuery(t *testing.T) {
	loggedIn(t)
	var path, query string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(categoryDatasetsJSON))
	})

	if err := runCLI(t, "category", "datasets", "IT.5", "--recursive", "--page", "2", "--page-size", "25"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/categories/IT.5/datasets") {
		t.Errorf("path = %q", path)
	}
	for _, want := range []string{"recursive=true", "page=2", "page_size=25"} {
		if !strings.Contains(query, want) {
			t.Errorf("query %q missing %q", query, want)
		}
	}
}

func TestCategoryDatasets_PrettyOutputWithPagination(t *testing.T) {
	buf := loggedIn(t)
	// Synthesize a "has_more" case.
	body := `{"data":[
		{"code":"IT.LAMA.001","name":"First","source":"istat","category_key":"IT.5.1.1"},
		{"code":"IT.LAMA.002","name":"Second","source":"istat","category_key":"IT.5.2.1"}
	],"meta":{"pagination":{"total":200,"page":1,"page_size":2,"has_more":true}}}`
	mockJSONAPI(t, body, 0)

	if err := runCLI(t, "category", "datasets", "IT.5", "--recursive"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Category: IT.5",
		"Showing 1-2 of 200 datasets",
		"CATEGORY",
		"IT.LAMA.001", "First", "IT.5.1.1",
		"IT.LAMA.002", "Second", "IT.5.2.1",
		"More results", "--page 2",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestCategoryDatasets_CategoryColumnRendersDashForMissing(t *testing.T) {
	buf := loggedIn(t)
	// No category_key on the stub — shouldn't render the zero-value "".
	mockJSONAPI(t, `{"data":[
		{"code":"IT.LAMA.001","name":"N","source":"istat"}
	],"meta":{"pagination":{"total":1,"page":1,"page_size":50,"has_more":false}}}`, 0)

	if err := runCLI(t, "category", "datasets", "IT.5"); err != nil {
		t.Fatal(err)
	}
	// Should show an em-dash, not an empty trailing cell.
	if !strings.Contains(buf.String(), "—") {
		t.Errorf("missing category_key should render as '—', got:\n%s", buf.String())
	}
}

func TestCategoryDatasets_EmptyList(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{"pagination":{"total":0,"page":1,"page_size":50,"has_more":false}}}`, 0)

	if err := runCLI(t, "category", "datasets", "IT.5"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Datasets: 0") {
		t.Errorf("output should show zero datasets, got:\n%s", buf.String())
	}
}

// ---------- --json passthrough for non-tree subcommands ----------

func TestCategoryGet_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, categoryGetJSON, 0)

	if err := runCLI(t, "category", "get", "IT.5", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("missing 'data' key")
	}
}

func TestCategoryAncestors_EmptyList(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{}}`, 0)

	if err := runCLI(t, "category", "ancestors", "IT"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No ancestors") {
		t.Errorf("empty ancestor list should say 'No ancestors.', got: %q", buf.String())
	}
}

func TestCategoryAncestors_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, ancestorsJSON, 0)

	if err := runCLI(t, "category", "ancestors", "IT.5.2.1", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("missing 'data' key")
	}
}

func TestCategoryDatasets_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, categoryDatasetsJSON, 0)

	if err := runCLI(t, "category", "datasets", "IT.5.2.1", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("missing 'data' key")
	}
}

// ---------- error paths / validation ----------

func TestCategory_APIErrorSurfacesCleanly(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"get 404", []string{"category", "get", "BAD"}},
		{"ancestors 404", []string{"category", "ancestors", "BAD"}},
		{"datasets 404", []string{"category", "datasets", "BAD"}},
		{"tree 404", []string{"category", "tree", "BAD"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loggedIn(t)
			mockJSONAPI(t, `{"error":{"code":"category_not_found","message":"Category 'BAD' not found"}}`, http.StatusNotFound)
			err := runCLI(t, c.args...)
			var apiErr *api.APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("err = %T %v, want *APIError", err, err)
			}
			if apiErr.StatusCode != http.StatusNotFound {
				t.Errorf("status = %d", apiErr.StatusCode)
			}
		})
	}
}

func TestCategory_NotLoggedIn(t *testing.T) {
	for _, args := range [][]string{
		{"category", "tree"},
		{"category", "tree", "IT.5"},
		{"category", "get", "IT.5"},
		{"category", "ancestors", "IT.5"},
		{"category", "datasets", "IT.5"},
	} {
		t.Run(fmt.Sprintf("%v", args), func(t *testing.T) {
			resetCmd(t)
			err := runCLI(t, args...)
			if err == nil {
				t.Fatal("expected error when not logged in")
			}
		})
	}
}

func TestCategory_RejectsPathTraversal(t *testing.T) {
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for invalid segments; got %s", r.URL)
	})

	cases := []struct {
		name string
		args []string
	}{
		{"tree ..", []string{"category", "tree", ".."}},
		{"tree slash", []string{"category", "tree", "a/b"}},
		{"get ..", []string{"category", "get", ".."}},
		{"ancestors ..", []string{"category", "ancestors", ".."}},
		{"datasets ..", []string{"category", "datasets", ".."}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loggedIn(t)
			err := runCLI(t, c.args...)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "invalid characters") {
				t.Errorf("error = %q", err)
			}
		})
	}
}
