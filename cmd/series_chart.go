/*
Copyright © 2026 Kalistat
*/
package cmd

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/linechart"
	"github.com/NimbleMarkets/ntcharts/linechart/timeserieslinechart"
	"github.com/kalistat-data/cli/internal/api"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	chartWidth  int
	chartHeight int
)

// resetChartFlags restores the package-level flag variables owned by this
// file. Registered in resetCmd per the test contract in auth_test.go.
func resetChartFlags() {
	chartWidth = 0
	chartHeight = 0
}

var seriesChartCmd = &cobra.Command{
	Use:   "chart <dataset> <series-code>",
	Short: "Render a time series as a line chart",
	Long: `Render a time series as a line chart in the terminal.

Fetches the same data as ` + "`series get`" + ` and draws it with Unicode braille
characters. The X axis auto-labels based on the series frequency (annual,
quarterly, monthly, daily). Observations with null values are skipped.

Use --width and --height to override the chart size; by default the chart
fills the terminal up to 120 columns by 24 rows, falling back to 80x20 when
output is piped.`,
	Args: cobra.ExactArgs(2),
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
		w, h := chartDimensions(chartWidth, chartHeight)
		return printSeriesChart(cmd, &resp.Data, w, h)
	},
}

// chartPoint is a parsed observation ready for plotting.
type chartPoint struct {
	t time.Time
	v float64
}

func printSeriesChart(cmd *cobra.Command, s *api.Series, width, height int) error {
	out := cmd.OutOrStdout()
	if err := printSeriesHeader(out, s); err != nil {
		return err
	}

	points, skipped := parseObservations(s.Values)
	if skipped > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "skipped %d unparseable timestamps\n", skipped)
	}
	if len(points) == 0 {
		fmt.Fprintln(out, "\nno plottable observations")
		return nil
	}

	fmt.Fprintln(out)

	freq := seriesFrequency(s)
	tslc := timeserieslinechart.New(width, height,
		timeserieslinechart.WithXLabelFormatter(xLabelFormatter(freq, points)),
	)
	yMin, yMax := paddedYRange(points)
	tslc.SetYRange(yMin, yMax)
	tslc.SetViewYRange(yMin, yMax)
	for _, p := range points {
		tslc.Push(timeserieslinechart.TimePoint{Time: p.t, Value: p.v})
	}
	tslc.DrawBraille()
	fmt.Fprintln(out, tslc.View())
	return nil
}

// paddedYRange returns a Y-axis range that hugs the data: 10% margin on each
// side of [min, max], expanded to a minimum total interval of 1.0 so a single
// point or a constant series still produces a readable chart.
func paddedYRange(points []chartPoint) (float64, float64) {
	if len(points) == 0 {
		return 0, 1
	}
	lo, hi := points[0].v, points[0].v
	for _, p := range points[1:] {
		if p.v < lo {
			lo = p.v
		}
		if p.v > hi {
			hi = p.v
		}
	}
	margin := 0.1 * (hi - lo)
	yMin, yMax := lo-margin, hi+margin
	if yMax-yMin < 1.0 {
		center := (lo + hi) / 2
		yMin, yMax = center-0.5, center+0.5
	}
	return yMin, yMax
}

// parseObservations turns the API's string/pointer-float observations into
// points the chart can consume. Nil-valued observations are dropped silently;
// observations with unparseable time strings are counted in `skipped` so the
// caller can surface a warning.
func parseObservations(obs []api.Observation) (points []chartPoint, skipped int) {
	points = make([]chartPoint, 0, len(obs))
	for _, o := range obs {
		if o.Value == nil {
			continue
		}
		t, ok := parseObservationTime(o.Time)
		if !ok {
			skipped++
			continue
		}
		points = append(points, chartPoint{t: t, v: *o.Value})
	}
	return points, skipped
}

// parseObservationTime accepts the time formats the Kalistat API emits:
// annual "2012", quarterly "2012-Q1", monthly "2012-01", daily "2012-01-02",
// and ISO-week "2012-W01". Anchors partial dates to the start of their period
// in UTC so the chart's X ordering is stable.
func parseObservationTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if len(s) == 7 && s[4] == '-' && s[5] == 'Q' {
		year, err := strconv.Atoi(s[:4])
		if err != nil {
			return time.Time{}, false
		}
		q, err := strconv.Atoi(s[6:])
		if err != nil || q < 1 || q > 4 {
			return time.Time{}, false
		}
		return time.Date(year, time.Month((q-1)*3+1), 1, 0, 0, 0, 0, time.UTC), true
	}
	if len(s) == 8 && s[4] == '-' && s[5] == 'W' {
		year, err := strconv.Atoi(s[:4])
		if err != nil {
			return time.Time{}, false
		}
		week, err := strconv.Atoi(s[6:])
		if err != nil || week < 1 || week > 53 {
			return time.Time{}, false
		}
		return isoWeekStart(year, week), true
	}
	for _, layout := range []string{"2006-01-02", "2006-01", "2006"} {
		if t, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// isoWeekStart returns the Monday of ISO week `week` in `year`, stepping back
// from Jan 4 (which always falls in ISO week 1).
func isoWeekStart(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	mondayOfWeek1 := jan4.AddDate(0, 0, -(weekday - 1))
	return mondayOfWeek1.AddDate(0, 0, (week-1)*7)
}

// seriesFrequency returns the value of the FREQ dimension ("A"/"Q"/"M"/"D"/"W"),
// or "" if no such dimension exists on the series.
func seriesFrequency(s *api.Series) string {
	for _, d := range s.Dimensions {
		if d.Key == "FREQ" {
			return strings.ToUpper(d.Value)
		}
	}
	return ""
}

// xLabelFormatter picks an X-axis label formatter matching the series'
// periodicity. FREQ is authoritative when set; otherwise infer from the gap
// between the first two points. For quarterly/weekly data we reconstruct the
// label from the reconstructed time because Go's time layouts can't express
// "Q" or "W".
func xLabelFormatter(freq string, points []chartPoint) linechart.LabelFormatter {
	switch freq {
	case "A", "Y":
		return simpleTimeFormatter("2006")
	case "Q":
		return quarterFormatter
	case "M":
		return simpleTimeFormatter("2006-01")
	case "W":
		return weekFormatter
	case "D":
		return simpleTimeFormatter("2006-01-02")
	}
	if len(points) >= 2 {
		gap := points[1].t.Sub(points[0].t)
		switch {
		case gap >= 360*24*time.Hour:
			return simpleTimeFormatter("2006")
		case gap >= 80*24*time.Hour:
			return quarterFormatter
		case gap >= 25*24*time.Hour:
			return simpleTimeFormatter("2006-01")
		}
	}
	return simpleTimeFormatter("2006-01-02")
}

func simpleTimeFormatter(layout string) linechart.LabelFormatter {
	return func(_ int, v float64) string {
		return time.Unix(int64(v), 0).UTC().Format(layout)
	}
}

func quarterFormatter(_ int, v float64) string {
	t := time.Unix(int64(v), 0).UTC()
	q := (int(t.Month())-1)/3 + 1
	return fmt.Sprintf("%d-Q%d", t.Year(), q)
}

func weekFormatter(_ int, v float64) string {
	t := time.Unix(int64(v), 0).UTC()
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// chartDimensions resolves (width, height) for the chart. A zero input means
// "auto": fill the terminal up to 120x24, falling back to 80x20 when stdout
// isn't a TTY (piped output, CI).
func chartDimensions(width, height int) (int, int) {
	w, h := width, height
	if w <= 0 || h <= 0 {
		if cols, rows, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			if w <= 0 {
				w = min(cols-2, 120)
			}
			if h <= 0 {
				// Leave room for the header and the shell prompt.
				h = min(rows-10, 24)
			}
		}
	}
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 20
	}
	return w, h
}

func init() {
	seriesChartCmd.Flags().IntVar(&chartWidth, "width", 0, "Chart width in columns (default: auto)")
	seriesChartCmd.Flags().IntVar(&chartHeight, "height", 0, "Chart height in rows (default: auto)")
	seriesCmd.AddCommand(seriesChartCmd)
}
