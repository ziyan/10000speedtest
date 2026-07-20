package speedtest

import (
	"strings"
	"testing"
	"time"
)

func sampleResult() Result {
	return Result{Name: "Download", Bytes: 100 * 1024 * 1024, Elapsed: 10 * time.Second, MegabitsPerSecond: 83.89}
}

// TestPlainRendererOnlyFinalLine verifies the non-terminal renderer draws no
// live progress and emits a clean final line (no carriage return or escapes).
func TestPlainRendererOnlyFinalLine(t *testing.T) {
	builder := &strings.Builder{}
	plain := &plainRenderer{name: "Download", output: builder}

	plain.sample(500)
	if builder.Len() != 0 {
		t.Fatalf("plain renderer must not draw live progress, got %q", builder.String())
	}

	plain.finish(sampleResult())
	out := builder.String()
	if strings.ContainsRune(out, '\r') || strings.Contains(out, "\033[") {
		t.Fatalf("plain final line must contain no carriage return or ANSI escape, got %q", out)
	}
	if !strings.Contains(out, "Download:") || !strings.HasSuffix(out, "\n") {
		t.Fatalf("expected a clean final summary line, got %q", out)
	}
}

// TestLineRendererOverwrites verifies the terminal single-line renderer returns
// to column 0 and clears the line so each update overwrites the last.
func TestLineRendererOverwrites(t *testing.T) {
	builder := &strings.Builder{}
	line := &lineRenderer{name: "Download", output: builder}

	line.sample(300)
	line.finish(sampleResult())
	out := builder.String()

	if !strings.HasPrefix(out, "\r\033[K") {
		t.Fatalf("line renderer should start by clearing the line, got %q", out)
	}
	if !strings.Contains(out, "Download:") {
		t.Fatal("expected the final summary line")
	}
}

// TestSilentRendererDrawsNothing verifies the JSON renderer produces no output
// and does not panic.
func TestSilentRendererDrawsNothing(t *testing.T) {
	var renderer silentRenderer
	renderer.sample(500)
	renderer.finish(sampleResult())
}

// primeWarmUp feeds the chart the leading warmup samples it discards, so a test
// can then exercise the real charting path.
func primeWarmUp(chart *chartRenderer) {
	for index := 0; index < warmUpSamples; index++ {
		chart.sample(1)
	}
}

// TestChartRendererFrameShape draws a single real frame (no cursor-up prefix)
// and checks it has the expected shape: header, chartHeight bar rows, and axis.
func TestChartRendererFrameShape(t *testing.T) {
	builder := &strings.Builder{}
	chart := &chartRenderer{name: "Download", useColor: false, output: builder, width: 40}

	primeWarmUp(chart)
	builder.Reset()

	// First post-warmup sample => one frame with no leading cursor-up escape.
	chart.sample(500)
	frame := builder.String()

	if strings.HasPrefix(frame, "\033[") {
		t.Fatal("first drawn frame must not move the cursor up")
	}
	if !strings.Contains(frame, "Download") {
		t.Fatal("expected the stage name in the chart header")
	}
	if !strings.Contains(frame, "peak") {
		t.Fatal("expected a peak label in the chart header")
	}
	if !strings.ContainsRune(frame, '█') {
		t.Fatal("expected at least one full bar block in the chart")
	}
	if !strings.ContainsRune(frame, '└') {
		t.Fatal("expected a bottom axis corner in the chart")
	}
	if lines := strings.Count(frame, "\n"); lines != chartHeight+2 {
		t.Fatalf("expected %d lines in the frame, got %d", chartHeight+2, lines)
	}
}

// TestChartRendererRedrawMovesCursor verifies later frames rewind the cursor so
// they overwrite the previous frame in place.
func TestChartRendererRedrawMovesCursor(t *testing.T) {
	builder := &strings.Builder{}
	chart := &chartRenderer{name: "Download", useColor: false, output: builder, width: 40}

	primeWarmUp(chart)
	builder.Reset()

	chart.sample(100)
	firstLength := builder.Len()
	chart.sample(200)
	secondFrame := builder.String()[firstLength:]

	if !strings.HasPrefix(secondFrame, "\033[10A") { // chartHeight+2
		t.Fatalf("expected the second frame to move the cursor up %d lines, got %q", chartHeight+2, secondFrame[:8])
	}
}

// TestChartRendererDiscardsWarmUp verifies the first second of samples is
// dropped from both the bars and the scale.
func TestChartRendererDiscardsWarmUp(t *testing.T) {
	builder := &strings.Builder{}
	chart := &chartRenderer{name: "Download", useColor: false, output: builder, width: 40}

	// A large warmup spike that must not become a bar or set the scale.
	for index := 0; index < warmUpSamples; index++ {
		chart.sample(9999)
	}
	if len(chart.samples) != 0 {
		t.Fatalf("expected warmup samples to be discarded, got %d retained", len(chart.samples))
	}
	if chart.drawn {
		t.Fatal("chart should not draw a frame during warmup")
	}
	if !strings.Contains(builder.String(), "warming up") {
		t.Fatal("expected a warming-up placeholder during warmup")
	}

	chart.sample(100)
	if len(chart.samples) != 1 {
		t.Fatalf("expected 1 charted sample after warmup, got %d", len(chart.samples))
	}
	if !chart.drawn {
		t.Fatal("chart should draw after the first post-warmup sample")
	}
}

// TestChartRendererColor verifies color escapes appear only when enabled.
func TestChartRendererColor(t *testing.T) {
	colored := &strings.Builder{}
	plain := &strings.Builder{}
	coloredChart := &chartRenderer{name: "Upload", useColor: true, output: colored, width: 40}
	plainChart := &chartRenderer{name: "Upload", useColor: false, output: plain, width: 40}

	primeWarmUp(coloredChart)
	primeWarmUp(plainChart)
	colored.Reset()
	plain.Reset()
	coloredChart.sample(300)
	plainChart.sample(300)

	if !strings.Contains(colored.String(), "\033[38;2;") {
		t.Fatal("expected a 24-bit color escape when color is enabled")
	}
	if strings.Contains(plain.String(), "\033[38;2;") {
		t.Fatal("did not expect color escapes when color is disabled")
	}
}

// TestTruncateColumns checks the header truncation used to prevent wrapping.
func TestTruncateColumns(t *testing.T) {
	if got := truncateColumns("hello", 10); got != "hello" {
		t.Fatalf("short text should be unchanged, got %q", got)
	}
	if got := truncateColumns("hello world", 5); got != "hello" {
		t.Fatalf("expected truncation to 5 columns, got %q", got)
	}
}

// TestBarsForColumns checks the terminal-width-to-bar-count arithmetic: the
// gutter and right margin are subtracted, narrow terminals hit the floor, and
// an unknown width falls back to the default.
func TestBarsForColumns(t *testing.T) {
	cases := []struct {
		columns int
		want    int
	}{
		{columns: 120, want: 120 - chartGutter - 1},
		{columns: 80, want: 80 - chartGutter - 1},
		{columns: 12, want: minChartWidth},                         // 12-9=3, floored to 10
		{columns: 0, want: defaultTerminalWidth - chartGutter - 1}, // unknown -> default
		{columns: -5, want: defaultTerminalWidth - chartGutter - 1},
	}
	for _, testCase := range cases {
		if got := barsForColumns(testCase.columns); got != testCase.want {
			t.Errorf("barsForColumns(%d) = %d, want %d", testCase.columns, got, testCase.want)
		}
	}
}

// TestSpeedColorRamp checks the low→high color ramp goes red-ish to green-ish.
func TestSpeedColorRamp(t *testing.T) {
	lowRed, lowGreen, _ := speedColor(0)
	_, highGreen, _ := speedColor(1)
	if lowGreen >= lowRed {
		t.Fatalf("expected low ratio to be red-dominant, got r=%d g=%d", lowRed, lowGreen)
	}
	if highGreen < lowGreen {
		t.Fatalf("expected high ratio to be greener than low, got low g=%d high g=%d", lowGreen, highGreen)
	}
}
