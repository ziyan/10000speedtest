package speedtest

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
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
	chartHeight          = 8  // number of bar rows
	chartGutter          = 8  // display columns used by the "%6.0f │" axis label
	minChartWidth        = 10 // fewest bars to draw when the terminal is very narrow
	defaultTerminalWidth = 80 // assumed width when the terminal size is unknown
)

// barBlocks maps an eighth level (0..8) to a partial-height block glyph.
var barBlocks = []rune{' ', '▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// renderer shows live progress for a stage and prints its final summary.
type renderer interface {
	// sample records an instantaneous throughput reading in Mbps and redraws.
	sample(megabitsPerSecond float64)
	// finish prints the final one-line summary for the stage.
	finish(result Result)
}

// newRenderer chooses how a stage reports progress:
//   - JSON output: nothing (the caller prints JSON at the end);
//   - not a terminal (piped/redirected): no live output, just a final line;
//   - a terminal with charting: a live colored bar chart;
//   - a terminal without charting: a single rewritten progress line.
func newRenderer(name string, config Config) renderer {
	switch {
	case config.JSONOutput:
		return silentRenderer{}
	case !stdoutIsTerminal():
		return &plainRenderer{name: name, output: os.Stdout}
	case config.Chart:
		return &chartRenderer{
			name:     name,
			useColor: os.Getenv("NO_COLOR") == "",
			output:   os.Stdout,
			width:    chartBars(os.Stdout),
		}
	default:
		return &lineRenderer{name: name, output: os.Stdout}
	}
}

// stdoutIsTerminal reports whether stdout is an interactive terminal, where
// ANSI cursor control and line rewriting are safe.
func stdoutIsTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// summaryLine formats the final one-line result shared by every renderer.
func summaryLine(result Result) string {
	return fmt.Sprintf("%-10s %8.2f Mbps   (%s in %.1fs)",
		result.Name+":", result.MegabitsPerSecond, humanBytes(result.Bytes), result.Elapsed.Seconds())
}

// lineRenderer rewrites a single line with the current throughput; used on a
// terminal when charting is disabled.
type lineRenderer struct {
	name   string
	output io.Writer
}

func (self *lineRenderer) sample(megabitsPerSecond float64) {
	_, _ = fmt.Fprintf(self.output, "\r\033[K%-10s %8.2f Mbps", self.name+":", megabitsPerSecond)
}

func (self *lineRenderer) finish(result Result) {
	_, _ = fmt.Fprintf(self.output, "\r\033[K%s\n", summaryLine(result))
}

// plainRenderer prints nothing while running and only the final line; used when
// stdout is not a terminal, so piped output is not littered with rewrites.
type plainRenderer struct {
	name   string
	output io.Writer
}

func (self *plainRenderer) sample(float64) {}

func (self *plainRenderer) finish(result Result) {
	_, _ = fmt.Fprintf(self.output, "%s\n", summaryLine(result))
}

// silentRenderer prints nothing at all; used for JSON output.
type silentRenderer struct{}

func (self silentRenderer) sample(float64) {}

func (self silentRenderer) finish(Result) {}

// chartRenderer draws a scrolling colored bar chart in place, one bar per
// sample, auto-scaling the vertical axis to the peak seen so far. The first
// chartWarmUp of samples are discarded so the connection-ramp spike does not
// compress the rest of the chart.
type chartRenderer struct {
	name     string
	useColor bool
	output   io.Writer
	width    int // maximum number of bars kept on screen, sized to the terminal
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
	if len(self.samples) > self.width {
		self.samples = self.samples[len(self.samples)-self.width:]
	}
	self.render()
}

// finish clears the line the cursor is on (below the chart, or the warming-up
// placeholder for a stage shorter than the warmup) and prints the summary.
func (self *chartRenderer) finish(result Result) {
	_, _ = fmt.Fprintf(self.output, "\r\033[K%s\n", summaryLine(result))
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

	// Header: stage name, current reading, and the running peak. Truncate it to
	// the row width so it cannot wrap on a narrow terminal and desync the redraw.
	header := fmt.Sprintf("%-10s %8.2f Mbps   peak %.2f Mbps", self.name, current, peak)
	fmt.Fprintf(&builder, "\r\033[K%s\n", truncateColumns(header, chartGutter+self.width))

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

// chartBars returns how many bar columns fit in the terminal attached to file,
// leaving room for the axis gutter and a one-column right margin so a full row
// cannot wrap and corrupt the in-place redraw.
func chartBars(file *os.File) int {
	columns, _, err := term.GetSize(int(file.Fd()))
	if err != nil {
		columns = 0
	}
	return barsForColumns(columns)
}

// truncateColumns shortens text to at most maxColumns display columns (treating
// each rune as one column, which holds for the ASCII header).
func truncateColumns(text string, maxColumns int) string {
	runes := []rune(text)
	if len(runes) <= maxColumns {
		return text
	}
	return string(runes[:maxColumns])
}

// barsForColumns turns a terminal column count into a bar count, applying the
// gutter, a right margin, and a floor for very narrow terminals. A non-positive
// column count (unknown size) falls back to an assumed width.
func barsForColumns(columns int) int {
	if columns <= 0 {
		columns = defaultTerminalWidth
	}
	bars := columns - chartGutter - 1
	if bars < minChartWidth {
		bars = minChartWidth
	}
	return bars
}
