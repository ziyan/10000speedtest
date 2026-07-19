package speedtest

import (
	"strings"
	"testing"
)

// TestChartRendererFrameShape draws a single frame (no cursor-up prefix) and
// checks it has the expected shape: a header, chartHeight bar rows, and an axis.
func TestChartRendererFrameShape(t *testing.T) {
	builder := &strings.Builder{}
	chart := &chartRenderer{name: "Download", useColor: false, output: builder}

	// One sample => one frame with no leading cursor-up escape.
	chart.sample(500)
	frame := builder.String()

	if strings.HasPrefix(frame, "\033[8A") || strings.Contains(frame, "A\r") {
		t.Fatal("first frame must not move the cursor up")
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
	// Header + chartHeight bar rows + axis line = chartHeight+2 newline-ended lines.
	if lines := strings.Count(frame, "\n"); lines != chartHeight+2 {
		t.Fatalf("expected %d lines in the frame, got %d", chartHeight+2, lines)
	}
}

// TestChartRendererRedrawMovesCursor verifies later frames rewind the cursor so
// they overwrite the previous frame in place.
func TestChartRendererRedrawMovesCursor(t *testing.T) {
	builder := &strings.Builder{}
	chart := &chartRenderer{name: "Download", useColor: false, output: builder}

	chart.sample(100)
	firstLength := builder.Len()
	chart.sample(200)
	secondFrame := builder.String()[firstLength:]

	if !strings.HasPrefix(secondFrame, "\033[10A") { // chartHeight+2
		t.Fatalf("expected the second frame to move the cursor up %d lines, got %q", chartHeight+2, secondFrame[:8])
	}
}

// TestChartRendererColor verifies color escapes appear only when enabled.
func TestChartRendererColor(t *testing.T) {
	colored := &strings.Builder{}
	plain := &strings.Builder{}
	(&chartRenderer{name: "Upload", useColor: true, output: colored}).sample(300)
	(&chartRenderer{name: "Upload", useColor: false, output: plain}).sample(300)

	if !strings.Contains(colored.String(), "\033[38;2;") {
		t.Fatal("expected a 24-bit color escape when color is enabled")
	}
	if strings.Contains(plain.String(), "\033[38;2;") {
		t.Fatal("did not expect color escapes when color is disabled")
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
