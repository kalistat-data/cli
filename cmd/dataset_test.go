package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/kalistat-data/cli/internal/api"
)

const datasetGetResponseJSON = `{
  "data": {
    "code": "IT.LAMA.132",
    "links": {"self": "/api/v1/datasets/IT.LAMA.132"},
    "name": "-- MAIN INDICATORS -- by gender, age",
    "source": "istat",
    "dimensions": [
      {"label":"Frequency","position":1,"key":"FREQ"},
      {"label":"Age","position":6,"key":"AGE"}
    ],
    "dataflow_id": "150_875_DF_DCCV_OCCUPATIMENS1_2",
    "series_count": 1020,
    "time_dimensions": [{"label":"Time","position":10,"key":"TIME_PERIOD"}],
    "category_key": "IT.5.2.1"
  },
  "meta": {"generated_at": "2026-04-22T18:00:00Z"}
}`

const datasetGetFixedValueJSON = `{
  "data": {
    "code": "IT.LAMA.132",
    "name": "Main indicators",
    "source": "istat",
    "dataflow_id": "df",
    "dimensions": [
      {"label":"Frequency","position":1,"key":"FREQ"},
      {"label":"Reference area","position":2,"key":"REF_AREA",
        "fixed_value":{"code":"IT","name":"Italy"}}
    ],
    "time_dimensions": [{"label":"Time","position":10,"key":"TIME_PERIOD"}]
  },
  "meta": {}
}`

const datasetAncestorsJSON = `{
  "data":[
    {"key":"IT","name":"Italy","depth":-3},
    {"key":"IT.5","name":"Labor market","depth":-2},
    {"key":"IT.5.2.1","name":"Employed - monthly data","depth":-1},
    {"key":"IT.LAMA.132","name":"Main indicators","depth":0}
  ],
  "meta":{}
}`

const datasetValuesResponseJSON = `{
  "data": [
    {"code":"Y15-24","name":"15-24 years","level":0},
    {"code":"Y25-34","name":"25-34 years","level":0}
  ],
  "meta": {"generated_at": "2026-04-22T18:00:00Z"}
}`

const datasetValuesHierarchicalJSON = `{
  "data": [
    {"code":"EU","name":"Europe","level":0},
    {"code":"IT","name":"Italy","level":1},
    {"code":"IT1","name":"North-West","level":2}
  ],
  "meta": {}
}`

func TestDatasetGet_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var gotPath string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(datasetGetResponseJSON))
	})

	if err := runCLI(t, "dataset", "get", "IT.LAMA.132"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotPath, "/datasets/IT.LAMA.132") {
		t.Errorf("path = %q, want to end with /datasets/IT.LAMA.132", gotPath)
	}
}

func TestDatasetGet_PrettyOutput(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetGetResponseJSON, 0)

	if err := runCLI(t, "dataset", "get", "IT.LAMA.132"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"IT.LAMA.132",
		"MAIN INDICATORS",
		"Source:       istat",
		"Dataflow:     150_875_DF_DCCV_OCCUPATIMENS1_2",
		"Category:     IT.5.2.1",
		"Series count: 1020",
		"Dimensions:", "POS", "KEY", "LABEL",
		"FREQ", "Frequency",
		"AGE", "Age",
		"Time dimensions:", "TIME_PERIOD",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestDatasetGet_DimensionsSortedByPosition(t *testing.T) {
	buf := loggedIn(t)
	// Deliberately shuffled order in the fixture to prove the CLI sorts.
	mockJSONAPI(t, `{
		"data": {
			"code":"X","name":"n","source":"istat","dataflow_id":"d",
			"dimensions":[
				{"label":"Third","position":3,"key":"C"},
				{"label":"First","position":1,"key":"A"},
				{"label":"Second","position":2,"key":"B"}
			],
			"time_dimensions":[]
		},
		"meta":{}
	}`, 0)

	if err := runCLI(t, "dataset", "get", "X"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	iA := strings.Index(out, "First")
	iB := strings.Index(out, "Second")
	iC := strings.Index(out, "Third")
	if !(iA < iB && iB < iC) {
		t.Errorf("dimensions not sorted by position: iA=%d iB=%d iC=%d\n%s", iA, iB, iC, out)
	}
}

func TestDatasetGet_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetGetResponseJSON, 0)

	if err := runCLI(t, "dataset", "get", "IT.LAMA.132", "--json"); err != nil {
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

func TestDatasetGet_APIErrorSurfacesCleanly(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t,
		`{"error":{"code":"dataset_not_found","message":"Dataset 'X' not found"}}`,
		http.StatusNotFound,
	)

	err := runCLI(t, "dataset", "get", "X")
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}

func TestDatasetGet_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	err := runCLI(t, "dataset", "get", "IT.LAMA.132")
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
	if !strings.Contains(err.Error(), "no API token") {
		t.Errorf("error = %q", err)
	}
}

func TestDatasetGet_FixedValueAddsColumn(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetGetFixedValueJSON, 0)

	if err := runCLI(t, "dataset", "get", "IT.LAMA.132"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "FIXED VALUE") {
		t.Errorf("expected FIXED VALUE column when any dimension is pinned:\n%s", out)
	}
	if !strings.Contains(out, "IT (Italy)") {
		t.Errorf("expected fixed-value cell 'IT (Italy)':\n%s", out)
	}
	// Pinned column should render only on the dimensions table — time
	// dimensions never carry fixed_value, so the time table must keep its
	// 3-column layout and not grow a trailing empty column. We check this
	// by confirming FIXED VALUE appears exactly once (dimensions header).
	if n := strings.Count(out, "FIXED VALUE"); n != 1 {
		t.Errorf("FIXED VALUE header should appear once, got %d:\n%s", n, out)
	}
}

func TestDatasetGet_NoFixedValuePreservesThreeColumnLayout(t *testing.T) {
	buf := loggedIn(t)
	// Regression: the existing happy-path fixture has no fixed_value, so
	// the table must stay exactly 3 columns.
	mockJSONAPI(t, datasetGetResponseJSON, 0)

	if err := runCLI(t, "dataset", "get", "IT.LAMA.132"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "FIXED VALUE") {
		t.Errorf("no dimension is pinned; FIXED VALUE column must not appear:\n%s", buf.String())
	}
}

func TestDatasetAncestors_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var path string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(datasetAncestorsJSON))
	})

	if err := runCLI(t, "dataset", "ancestors", "IT.LAMA.132"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, "/datasets/IT.LAMA.132/ancestors") {
		t.Errorf("path = %q", path)
	}
}

func TestDatasetAncestors_RendersTreeConnectors(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetAncestorsJSON, 0)

	if err := runCLI(t, "dataset", "ancestors", "IT.LAMA.132"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Target (depth 0) is the dataset itself and must be flagged with '>'.
	if !strings.Contains(out, "> IT.LAMA.132") {
		t.Errorf("target line should be marked with '>':\n%s", out)
	}
	// The breadcrumb is a single chain — every non-root line uses the
	// last-sibling connector.
	if !strings.Contains(out, "└── IT.5  ") {
		t.Errorf("expected tree connector at IT.5:\n%s", out)
	}
}

func TestDatasetAncestors_ASCIIFlag(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetAncestorsJSON, 0)

	if err := runCLI(t, "dataset", "ancestors", "IT.LAMA.132", "--ascii"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.ContainsAny(out, "└├│") {
		t.Errorf("ASCII mode must not emit box-drawing glyphs:\n%s", out)
	}
	if !strings.Contains(out, "`-- IT.5  ") {
		t.Errorf("ASCII mode should use `-- connector:\n%s", out)
	}
}

func TestDatasetAncestors_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetAncestorsJSON, 0)

	if err := runCLI(t, "dataset", "ancestors", "IT.LAMA.132", "--json"); err != nil {
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

func TestDatasetAncestors_EmptyList(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{}}`, 0)

	if err := runCLI(t, "dataset", "ancestors", "X"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No ancestors") {
		t.Errorf("empty list should say 'No ancestors.', got: %q", buf.String())
	}
}

func TestDatasetAncestors_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	if err := runCLI(t, "dataset", "ancestors", "IT.LAMA.132"); err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestDatasetValues_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var gotPath string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(datasetValuesResponseJSON))
	})

	if err := runCLI(t, "dataset", "values", "IT.LAMA.132", "AGE"); err != nil {
		t.Fatal(err)
	}
	want := "/datasets/IT.LAMA.132/dimensions/AGE/values"
	if !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want to end with %q", gotPath, want)
	}
}

func TestDatasetValues_FlatListOmitsLevelColumn(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetValuesResponseJSON, 0)

	if err := runCLI(t, "dataset", "values", "IT.LAMA.132", "AGE"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Dimension: AGE", "Values: 2", "CODE", "NAME", "Y15-24", "15-24 years"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	if strings.Contains(out, "LEVEL") {
		t.Errorf("LEVEL column should be hidden for flat codelists, got:\n%s", out)
	}
}

func TestDatasetValues_HierarchicalShowsLevelColumn(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetValuesHierarchicalJSON, 0)

	if err := runCLI(t, "dataset", "values", "EU.XYZ", "REF_AREA"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "LEVEL") {
		t.Errorf("LEVEL column should appear when any value has level > 0:\n%s", out)
	}
	if !strings.Contains(out, "IT1") {
		t.Errorf("output missing 'IT1':\n%s", out)
	}
}

func TestDatasetValues_EmptyList(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{}}`, 0)

	if err := runCLI(t, "dataset", "values", "X", "Y"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Values: 0") {
		t.Errorf("output should show zero values, got %q", buf.String())
	}
}

func TestDatasetValues_APIErrorSurfacesCleanly(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t,
		`{"error":{"code":"dimension_not_found","message":"Dimension 'X' not found"}}`,
		http.StatusNotFound,
	)

	err := runCLI(t, "dataset", "values", "IT.LAMA.132", "NONESUCH")
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}

func TestDatasetValues_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, datasetValuesResponseJSON, 0)

	if err := runCLI(t, "dataset", "values", "IT.LAMA.132", "AGE", "--json"); err != nil {
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

func TestDatasetValues_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	err := runCLI(t, "dataset", "values", "IT.LAMA.132", "AGE")
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}

// TestDataset_RejectsPathTraversal covers the security path. validateSegment
// is shared with series but we assert it's actually wired in on both
// subcommands and on both path arguments.
func TestDataset_RejectsPathTraversal(t *testing.T) {
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for invalid segments; got %s %s", r.Method, r.URL)
	})

	cases := []struct {
		name string
		args []string
	}{
		{"get: dataset is ..", []string{"dataset", "get", ".."}},
		{"get: dataset has slash", []string{"dataset", "get", "a/b"}},
		{"get: dataset empty", []string{"dataset", "get", ""}},
		{"ancestors: dataset is ..", []string{"dataset", "ancestors", ".."}},
		{"ancestors: dataset has slash", []string{"dataset", "ancestors", "a/b"}},
		{"values: dataset is ..", []string{"dataset", "values", "..", "AGE"}},
		{"values: dim-key is ..", []string{"dataset", "values", "IT.LAMA.132", ".."}},
		{"values: dim-key has slash", []string{"dataset", "values", "IT.LAMA.132", "A/B"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loggedIn(t)
			err := runCLI(t, c.args...)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), "invalid characters") {
				t.Errorf("error = %q, want to mention invalid characters", err)
			}
		})
	}
}
