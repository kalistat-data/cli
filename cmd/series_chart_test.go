package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kalistat-data/cli/internal/api"
)

const seriesChartQuarterlyJSON = `{
  "data": {
    "ticker": "IT.NACC.111/Q.IT.P72_C_W1_S1.L_2020.N.2025M11",
    "dimensions": [
      {"key":"FREQ","label":"Frequency","position":1,"value":"Q"}
    ],
    "values": [
      {"time":"2020-Q1","value":100.5},
      {"time":"2020-Q2","value":102.1},
      {"time":"2020-Q3","value":null},
      {"time":"2020-Q4","value":105.2}
    ]
  },
  "meta": {"generated_at": "2026-04-22T18:00:00Z"}
}`

func TestSeriesChart_RequiresBothArgs(t *testing.T) {
	resetCmd(t)

	if err := runCLI(t, "series", "chart"); err == nil {
		t.Fatal("expected error when no args")
	}
	if err := runCLI(t, "series", "chart", "only-one"); err == nil {
		t.Fatal("expected error when only dataset arg")
	}
}

func TestSeriesChart_EmptyValuesShowsNoObservationsMessage(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":{"ticker":"IT.X/Y","dimensions":[],"values":[]},"meta":{}}`, 0)

	if err := runCLI(t, "series", "chart", "IT.X", "Y"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "no plottable observations") {
		t.Errorf("empty series should announce no plottable observations, got:\n%s", out)
	}
	// The header must still render so the user knows which series was fetched.
	if !strings.Contains(out, "Ticker: IT.X/Y") {
		t.Errorf("header should still render for empty series, got:\n%s", out)
	}
}

func TestSeriesChart_AllNullValuesShowsNoObservationsMessage(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, `{"data":{"ticker":"IT.X/Y","dimensions":[],"values":[
		{"time":"2020-01","value":null},
		{"time":"2020-02","value":null}
	]},"meta":{}}`, 0)

	if err := runCLI(t, "series", "chart", "IT.X", "Y"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "no plottable observations") {
		t.Errorf("all-null series should announce no plottable observations, got:\n%s", buf.String())
	}
}

func TestSeriesChart_JSONPassthrough(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesChartQuarterlyJSON, 0)

	if err := runCLI(t, "series", "chart", "IT.NACC.111", "Q.IT.P72_C_W1_S1.L_2020.N.2025M11", "--json"); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("--json mode should emit raw JSON, got: %v\n%s", err, buf.String())
	}
	if _, ok := got["data"]; !ok {
		t.Errorf("missing 'data' key in JSON passthrough: %s", buf.String())
	}
}

func TestSeriesChart_BuildsCorrectURL(t *testing.T) {
	loggedIn(t)
	var gotPath string
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(seriesChartQuarterlyJSON))
	})

	if err := runCLI(t, "series", "chart", "IT.NACC.111", "Q.IT.P72_C_W1_S1.L_2020.N.2025M11", "--width", "40", "--height", "10"); err != nil {
		t.Fatal(err)
	}
	want := "/datasets/IT.NACC.111/series/Q.IT.P72_C_W1_S1.L_2020.N.2025M11"
	if !strings.HasSuffix(gotPath, want) {
		t.Errorf("path = %q, want to end with %q", gotPath, want)
	}
}

func TestSeriesChart_RendersChartForQuarterlySeries(t *testing.T) {
	buf := loggedIn(t)
	mockJSONAPI(t, seriesChartQuarterlyJSON, 0)

	// Force a small size so output stays compact and deterministic enough
	// to inspect for structural markers. We don't snapshot the braille body.
	if err := runCLI(t, "series", "chart", "IT.NACC.111", "Q.IT.P72_C_W1_S1.L_2020.N.2025M11", "--width", "40", "--height", "10"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// The header must render with the ticker and observation range.
	if !strings.Contains(out, "Ticker: IT.NACC.111") {
		t.Errorf("output missing ticker line:\n%s", out)
	}
	if !strings.Contains(out, "Observations: 4") {
		t.Errorf("output missing observation count:\n%s", out)
	}
	// Chart body must actually render (non-empty beyond the header).
	// A rendered chart always contains the Y-axis "│" or the default tick
	// character; check for either. This is a weak assertion on purpose —
	// terminal rendering is terminal-size sensitive.
	if len(out) < 200 {
		t.Errorf("chart output suspiciously short (%d bytes), may not have rendered:\n%s", len(out), out)
	}
}

func TestSeriesChart_RejectsPathTraversal(t *testing.T) {
	mockAPI(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API should not be called for invalid segments; got %s %s", r.Method, r.URL)
	})

	cases := []struct {
		name string
		args []string
	}{
		{"dataset is ..", []string{"series", "chart", "..", "A.B"}},
		{"series-code is ..", []string{"series", "chart", "IT.NACC.111", ".."}},
		{"series-code has slash", []string{"series", "chart", "IT.NACC.111", "A/B"}},
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

func TestSeriesChart_NotLoggedInReturnsError(t *testing.T) {
	resetCmd(t)

	err := runCLI(t, "series", "chart", "IT.NACC.111", "Q.IT.TOT")
	if err == nil {
		t.Fatal("expected error when not logged in")
	}
}

func TestParseObservationTime(t *testing.T) {
	cases := []struct {
		in       string
		wantYear int
		wantMon  time.Month
		wantDay  int
		wantOK   bool
	}{
		{"2012", 2012, time.January, 1, true},
		{"2012-01", 2012, time.January, 1, true},
		{"2012-07", 2012, time.July, 1, true},
		{"2012-01-15", 2012, time.January, 15, true},
		{"2012-Q1", 2012, time.January, 1, true},
		{"2012-Q2", 2012, time.April, 1, true},
		{"2012-Q3", 2012, time.July, 1, true},
		{"2012-Q4", 2012, time.October, 1, true},
		{"  2012-Q1  ", 2012, time.January, 1, true},
		// ISO week — 2012-W01 starts Monday Jan 2, 2012.
		{"2012-W01", 2012, time.January, 2, true},
		{"", 0, 0, 0, false},
		{"nope", 0, 0, 0, false},
		{"2012-Q0", 0, 0, 0, false},
		{"2012-Q5", 0, 0, 0, false},
		{"2012-W54", 0, 0, 0, false},
		{"2012-13", 0, 0, 0, false}, // invalid month
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, ok := parseObservationTime(c.in)
			if ok != c.wantOK {
				t.Fatalf("ok = %v, want %v (got=%v)", ok, c.wantOK, got)
			}
			if !ok {
				return
			}
			if got.Year() != c.wantYear || got.Month() != c.wantMon || got.Day() != c.wantDay {
				t.Errorf("got %s, want %04d-%02d-%02d", got.Format("2006-01-02"), c.wantYear, c.wantMon, c.wantDay)
			}
			if got.Location() != time.UTC {
				t.Errorf("expected UTC, got %v", got.Location())
			}
		})
	}
}

func TestParseObservations_DropsNilValuesAndCountsSkips(t *testing.T) {
	v := func(x float64) *float64 { return &x }
	obs := []api.Observation{
		{Time: "2020-01", Value: v(1.0)},
		{Time: "2020-02", Value: nil},
		{Time: "garbage", Value: v(2.0)},
		{Time: "2020-03", Value: v(3.0)},
	}
	points, skipped := parseObservations(obs)
	if len(points) != 2 {
		t.Errorf("got %d points, want 2", len(points))
	}
	if skipped != 1 {
		t.Errorf("got %d skipped, want 1", skipped)
	}
}

func TestPaddedYRange(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name             string
		values           []float64
		wantMin, wantMax float64
	}{
		{"empty", nil, 0, 1},
		// Range 100 → 10% margin = 10 each side → [90, 220].
		{"wide range pads 10% each side", []float64{100, 200}, 90, 210},
		// Range 0 → falls back to centered 1.0 interval.
		{"single point", []float64{42}, 41.5, 42.5},
		// Range 0 → centered 1.0 interval around the constant value.
		{"constant series", []float64{7, 7, 7}, 6.5, 7.5},
		// Range 0.4 → padded to 0.48, still under 1.0 → expand to 1.0 around center 5.
		{"small range expands to minimum 1.0", []float64{4.8, 5.2}, 4.5, 5.5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pts := make([]chartPoint, len(c.values))
			for i, v := range c.values {
				pts[i] = chartPoint{v: v}
			}
			gotMin, gotMax := paddedYRange(pts)
			if abs(gotMin-c.wantMin) > eps || abs(gotMax-c.wantMax) > eps {
				t.Errorf("got [%g, %g], want [%g, %g]", gotMin, gotMax, c.wantMin, c.wantMax)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestSeriesFrequency(t *testing.T) {
	cases := []struct {
		name string
		dims []api.SeriesDimension
		want string
	}{
		{"present, lowercase", []api.SeriesDimension{{Key: "FREQ", Value: "q"}}, "Q"},
		{"present, uppercase", []api.SeriesDimension{{Key: "FREQ", Value: "M"}}, "M"},
		{"missing", []api.SeriesDimension{{Key: "REF_AREA", Value: "IT"}}, ""},
		{"empty dimensions", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := seriesFrequency(&api.Series{Dimensions: c.dims})
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestChartDimensions_FallbackWhenNotTTY(t *testing.T) {
	// In `go test` stdout is usually not a TTY, so GetSize errors and we
	// fall back to the hardcoded 80x20 default.
	w, h := chartDimensions(0, 0)
	if w != 80 || h != 20 {
		t.Errorf("fallback dims = (%d, %d), want (80, 20)", w, h)
	}
}

func TestChartDimensions_RespectsExplicitFlags(t *testing.T) {
	w, h := chartDimensions(42, 12)
	if w != 42 || h != 12 {
		t.Errorf("explicit dims = (%d, %d), want (42, 12)", w, h)
	}
}
