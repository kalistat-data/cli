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

const seriesListResponseJSON = `{
  "data": [
    {
      "ticker": "IT.LAMA.132/M.IT.EMP.Y.9.Y15-24.9.9.2026M4G1",
      "dimensions": [
        {"key":"FREQ","label":"Frequency","position":1,"value":"M"},
        {"key":"AGE","label":"Age","position":6,"value":"Y15-24"}
      ],
      "values": [
        {"time":"2004-01","value":1730.913},
        {"time":"2004-02","value":1712.509},
        {"time":"2026-02","value":1002.014}
      ]
    },
    {
      "ticker": "IT.LAMA.132/M.IT.EMP.Y.9.Y25-34.9.9.2026M4G1",
      "dimensions": [],
      "values": [
        {"time":"2004-01","value":100.0},
        {"time":"2026-02","value":200.0}
      ]
    }
  ],
  "meta": {"count": 2, "generated_at": "2026-04-22T18:00:00Z"}
}`

const seriesGetResponseJSON = `{
  "data": {
    "ticker": "IT.LAMA.132/M.IT.EMP.Y.9.Y15-24.9.9.CURRENT",
    "dimensions": [
      {"key":"FREQ","label":"Frequency","position":1,"value":"M"},
      {"key":"REF_AREA","label":"Territory","position":2,"value":"IT"},
      {"key":"AGE","label":"Age","position":6,"value":"Y15-24"}
    ],
    "values": [
      {"time":"2004-01","value":1730.913},
      {"time":"2004-02","value":null},
      {"time":"2004-03","value":1730.837}
    ]
  },
  "meta": {"generated_at": "2026-04-22T18:00:00Z"}
}`

func TestSeriesList_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var gotPath, gotQuery string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(seriesListResponseJSON))
	})

	if err := runCLI(t, "series", "list", "IT.LAMA.132", "M.IT.EMP.Y.9.*.9.9.CURRENT"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotPath, "/datasets/IT.LAMA.132/series") {
		t.Errorf("path = %q", gotPath)
	}
	if !strings.Contains(gotQuery, "pattern=M.IT.EMP.Y.9.%2A.9.9.CURRENT") {
		t.Errorf("query should percent-encode the wildcard, got %q", gotQuery)
	}
}

func TestSeriesList_PrettyOutput(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesListResponseJSON, 0)

	if err := runCLI(t, "series", "list", "IT.LAMA.132", "M.IT.EMP.Y.9.*.9.9.CURRENT"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"Pattern: M.IT.EMP.Y.9.*.9.9.CURRENT", "Matched: 2 series", "TICKER", "OBSERVATIONS", "RANGE", "2004-01 → 2026-02", "Y15-24", "Y25-34"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestSeriesList_NoMatches(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":[],"meta":{"count":0}}`, 0)

	if err := runCLI(t, "series", "list", "IT.LAMA.132", "X.Y.Z"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Matched: 0 series") {
		t.Errorf("output should show zero matches, got %q", buf.String())
	}
}

func TestSeriesList_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesListResponseJSON, 0)

	if err := runCLI(t, "series", "list", "IT.LAMA.132", "*", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, buf.String())
	}
}

func TestSeriesGet_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var gotPath string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(seriesGetResponseJSON))
	})

	if err := runCLI(t, "series", "get", "IT.LAMA.132", "M.IT.EMP.Y.9.Y15-24.9.9.CURRENT"); err != nil {
		t.Fatal(err)
	}
	want := "/datasets/IT.LAMA.132/series/M.IT.EMP.Y.9.Y15-24.9.9.CURRENT"
	if !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want to end with %q", gotPath, want)
	}
}

func TestSeriesGet_PrettyOutput(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesGetResponseJSON, 0)

	if err := runCLI(t, "series", "get", "IT.LAMA.132", "M.IT.EMP.Y.9.Y15-24.9.9.CURRENT"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"Ticker: IT.LAMA.132",
		"Dimensions:",
		"Frequency (FREQ)",
		"Territory (REF_AREA)",
		"Age (AGE)",
		"Y15-24",
		"Observations: 3",
		"2004-01 → 2004-03",
		"TIME", "VALUE",
		"1730.913",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
}

func TestSeriesGet_NullValueRendersAsDash(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesGetResponseJSON, 0)

	if err := runCLI(t, "series", "get", "X", "Y"); err != nil {
		t.Fatal(err)
	}
	// The 2004-02 observation has value: null — must render as "—" to make
	// missing observations visually obvious instead of misreading as zero.
	if !strings.Contains(buf.String(), "2004-02  —") {
		t.Errorf("null observation should render as '—', got:\n%s", buf.String())
	}
}

func TestSeriesGet_RequiresBothArgs(t *testing.T) {
	resetCmd(t)

	if err := runCLI(t, "series", "get"); err == nil {
		t.Fatal("expected error when no args")
	}
	if err := runCLI(t, "series", "get", "only-one"); err == nil {
		t.Fatal("expected error when only dataset arg")
	}
}

func TestSeriesGet_ZeroObservations(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":{"ticker":"IT.X/Y","dimensions":[],"values":[]},"meta":{}}`, 0)

	if err := runCLI(t, "series", "get", "IT.X", "Y"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Observations: 0") {
		t.Errorf("want 'Observations: 0', got:\n%s", out)
	}
	// Empty series must not print the observations table header.
	if strings.Contains(out, "TIME") || strings.Contains(out, "VALUE") {
		t.Errorf("empty series should not print observation table:\n%s", out)
	}
	// No range should appear in parentheses after the count.
	if strings.Contains(out, "Observations: 0 (") {
		t.Errorf("empty series should not advertise a time range:\n%s", out)
	}
}

func TestSeriesGet_SingleObservationRangeIsSinglePoint(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":{"ticker":"IT.X/Y","dimensions":[],"values":[{"time":"2024-01","value":42}]},"meta":{}}`, 0)

	if err := runCLI(t, "series", "get", "IT.X", "Y"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Single observation should render range as "(T)", never "T → T".
	if strings.Contains(out, "2024-01 → 2024-01") {
		t.Errorf("single-observation range must not use the arrow form:\n%s", out)
	}
	if !strings.Contains(out, "Observations: 1 (2024-01)") {
		t.Errorf("single-observation range should appear as '(2024-01)':\n%s", out)
	}
	// The observations table itself should still render — 1 row is a row.
	if !strings.Contains(out, "TIME") || !strings.Contains(out, "42") {
		t.Errorf("single-observation should still print the table:\n%s", out)
	}
}

func TestSeriesList_RequiresBothArgs(t *testing.T) {
	resetCmd(t)

	if err := runCLI(t, "series", "list", "only-dataset"); err == nil {
		t.Fatal("expected error when pattern arg is missing")
	}
}

func TestSeriesGet_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesGetResponseJSON, 0)

	if err := runCLI(t, "series", "get", "IT.LAMA.132", "M.IT.EMP.Y.9.Y15-24.9.9.CURRENT", "--json"); err != nil {
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

func TestSeriesList_APIErrorSurfacesCleanly(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t,
		`{"error":{"code":"dataset_not_found","message":"dataset not found"}}`,
		http.StatusNotFound,
	)

	err := runCLI(t, "series", "list", "UNKNOWN", "A.*.B")
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}

func TestSeriesGet_APIErrorSurfacesCleanly(t *testing.T) {
	loggedIn(t)
	mockJSONAPI(t,
		`{"error":{"code":"not_found","message":"series not found"}}`,
		http.StatusNotFound,
	)

	err := runCLI(t, "series", "get", "IT.LAMA.132", "BOGUS.CODE")
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %T %v, want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
}

func TestSeriesList_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	err := runCLI(t, "series", "list", "IT.LAMA.132", "A.*.B")
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
	if !strings.Contains(err.Error(), "no API token") {
		t.Errorf("error = %q, want to mention missing token", err)
	}
}

func TestSeriesGet_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	err := runCLI(t, "series", "get", "IT.LAMA.132", "A.IT.TOT")
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}

// TestSeries_RejectsPathTraversal covers the security fix: url.PathEscape
// does not encode `.` or `..`, and url.URL.JoinPath then runs path.Clean,
// which would silently redirect requests to unintended endpoints. Both
// subcommands must reject such inputs before constructing the URL.
func TestSeries_RejectsPathTraversal(t *testing.T) {
	// Fail the test if any request reaches the server — validation must
	// happen client-side before the HTTP call.
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for invalid segments; got %s %s", r.Method, r.URL)
	})

	cases := []struct {
		name string
		args []string
	}{
		{"list: dataset is ..", []string{"series", "list", "..", "A.B"}},
		{"list: dataset is .", []string{"series", "list", ".", "A.B"}},
		{"list: dataset has slash", []string{"series", "list", "a/b", "A.B"}},
		{"get: dataset is ..", []string{"series", "get", "..", "A.B"}},
		{"get: serie-code is ..", []string{"series", "get", "IT.LAMA.132", ".."}},
		{"get: serie-code has slash", []string{"series", "get", "IT.LAMA.132", "A/B"}},
		{"get: dataset empty", []string{"series", "get", "", "A.B"}},
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
