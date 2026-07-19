package speedtest

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// reportInterval is how often the live throughput is sampled and drawn.
const reportInterval = 250 * time.Millisecond

// chartWarmUp is how much of the start of a stage the chart ignores, so the
// initial connection-ramp spike does not dominate the vertical scale. The
// summary average still covers the whole run.
const chartWarmUp = time.Second

// warmUpSamples is the number of leading samples the chart discards.
const warmUpSamples = int(chartWarmUp / reportInterval)

const (
	chartHeight = 8  // number of bar rows
	chartWidth  = 50 // maximum number of bars kept on screen
)

// barBlocks maps an eighth level (0..8) to a partial-height block glyph.
var barBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// renderer draws live throughput samples for a single stage.
type renderer interface {
	// sample records an instantaneous throughput reading in Mbps and redraws.
	sample(megabitsPerSecond float64)
}

// newRenderer returns a colored bar chart when chart output is requested and
// stdout is a terminal, and a single-line reporter otherwise.
func newRenderer(name string, chart bool) renderer {
	if chart && isTerminal(os.Stdout) {
		return &chartRenderer{name: name, useColor: os.Getenv("NO_COLOR") == "", output: os.Stdout}
	}
	return &lineRenderer{name: name}
}

// lineRenderer prints one rewritten line with the current throughput.
type lineRenderer struct {
	name string
}

func (self *lineRenderer) sample(megabitsPerSecond float64) {
	fmt.Printf("\r%-10s %8.2f Mbps   ", self.name+":", megabitsPerSecond)
}

// chartRenderer draws a scrolling colored bar chart in place, one bar per
// sample, auto-scaling the vertical axis to the peak seen so far. The first
// chartWarmUp of samples are discarded so the connection-ramp spike does not
// compress the rest of the chart.
type chartRenderer struct {
	name     string
	useColor bool
	output   io.Writer
	received int
	samples  []float64
	drawn    bool
}

func (self *chartRenderer) sample(megabitsPerSecond float64) {
	self.received++
	if self.received <= warmUpSamples {
		// Show a placeholder on the line the chart header will later overwrite.
		_, _ = fmt.Fprintf(self.output, "\r\033[K%-10s warming up…", self.name)
		return
	}
	self.samples = append(self.samples, megabitsPerSecond)
	if len(self.samples) > chartWidth {
		self.samples = self.samples[len(self.samples)-chartWidth:]
	}
	self.render()
}

func (self *chartRenderer) render() {
	peak := 0.0
	for _, value := range self.samples {
		if value > peak {
			peak = value
		}
	}
	if peak <= 0 {
		peak = 1
	}
	current := self.samples[len(self.samples)-1]

	var builder strings.Builder
	// On every frame after the first, move the cursor back up over the block we
	// drew last time so this frame overwrites it in place.
	if self.drawn {
		fmt.Fprintf(&builder, "\033[%dA", chartHeight+2)
	}
	self.drawn = true

	// Header: stage name, current reading, and the running peak.
	fmt.Fprintf(&builder, "\r\033[K%-10s %8.2f Mbps   peak %.2f Mbps\n", self.name, current, peak)

	// Bar rows, from the top row down to the bottom.
	for row := chartHeight - 1; row >= 0; row-- {
		axisLabel := peak * float64(row+1) / float64(chartHeight)
		fmt.Fprintf(&builder, "\r\033[K%6.0f │", axisLabel)
		for _, value := range self.samples {
			builder.WriteString(self.cell(value, peak, row))
		}
		builder.WriteString("\n")
	}

	// Bottom axis line.
	fmt.Fprintf(&builder, "\r\033[K%6.0f └%s\n", 0.0, strings.Repeat("─", len(self.samples)))

	_, _ = fmt.Fprint(self.output, builder.String())
}

// cell renders one bar's glyph for the given row, colored by how tall the bar
// is relative to the current peak.
func (self *chartRenderer) cell(value, peak float64, row int) string {
	level := value / peak * float64(chartHeight) // bar height in cells
	fill := level - float64(row)                 // how full this cell is, in cells
	switch {
	case fill <= 0:
		return " "
	case fill >= 1:
		return self.colorize('█', value/peak)
	default:
		eighth := int(fill * 8)
		if eighth < 1 {
			eighth = 1
		}
		return self.colorize(barBlocks[eighth], value/peak)
	}
}

// colorize wraps a glyph in a 24-bit color that runs red (low) → yellow → green
// (high) with the throughput ratio, unless color is disabled.
func (self *chartRenderer) colorize(glyph rune, ratio float64) string {
	if !self.useColor {
		return string(glyph)
	}
	red, green, blue := speedColor(ratio)
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%c\033[0m", red, green, blue, glyph)
}

// speedColor maps a ratio in [0,1] to an RGB triple on a red→yellow→green ramp.
func speedColor(ratio float64) (int, int, int) {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0.5 {
		fraction := ratio / 0.5
		return interpolate(220, 235, fraction), interpolate(60, 200, fraction), interpolate(50, 50, fraction)
	}
	fraction := (ratio - 0.5) / 0.5
	return interpolate(235, 70, fraction), interpolate(200, 200, fraction), interpolate(50, 90, fraction)
}

// interpolate linearly blends between two integers by fraction in [0,1].
func interpolate(from, to int, fraction float64) int {
	return from + int(float64(to-from)*fraction)
}

// isTerminal reports whether the file is a character device (a terminal),
// meaning ANSI cursor control is safe to use.
func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
