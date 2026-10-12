package main

import (
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// The two figures, drawn from the same rows as the two tables.
//
// A chart is generated here rather than drawn by hand for the same reason the
// tables are: a hand-drawn figure of a measurement is a second description of
// it, and the two descriptions drift apart - the first time the benchmark is
// re-run somewhere else, the picture is of a machine that no longer exists.
// Every number in these files is computed by the same method that puts the
// number in the CSV beside it, and a test reads both back and checks they
// agree.
//
// They are SVG because SVG is text. It is written here with no drawing library
// and no dependency, it renders at whatever width a page gives it, and it can
// be read in a diff - which is more than can be said for a PNG committed
// beside the numbers it draws.

// The palette is the two categorical slots 1 and 2 of the reference data-viz
// palette, in both appearances, with a surface, inks and hairlines from the
// same reference. Two series is the whole of it: the client written in CCWIG
// and the one written in Go. The pair was run through the palette validator -
// lightness band, chroma floor, CVD separation, normal-vision floor and
// contrast - and passes every check in both appearances, worst adjacent CVD
// ΔE 24.7 light and 26.8 dark against a floor of 8.
//
// The light and dark values are both declared rather than one being a
// computed flip of the other: a hue stepped for a white surface is not
// readable on a black one, and the dark column is the same two hues stepped
// for the dark surface. Dark is selected with a media query, and the card
// carries its own surface because the ring around a marker has to be the
// color it is drawn on - a ring in the light surface color on a dark page is
// a halo.
const chartStyle = `<style>
  .chart { --surface:#fcfcfb; --ink:#0b0b0b; --ink2:#52514e; --muted:#898781;
           --grid:#e1e0d9; --axis:#c3c2b7; --s1:#2a78d6; --s2:#eb6834; }
  @media (prefers-color-scheme: dark) {
    .chart { --surface:#1a1a19; --ink:#ffffff; --ink2:#c3c2b7;
             --grid:#2c2c2a; --axis:#383835; --s1:#3987e5; --s2:#d95926; }
  }
  .chart text { font-family: system-ui, -apple-system, "Segoe UI", sans-serif; }
  .t-title { font-size:15px; font-weight:600; fill:var(--ink); }
  .t-sub   { font-size:12.5px; fill:var(--muted); }
  .t-tick  { font-size:11.5px; fill:var(--muted); font-variant-numeric:tabular-nums; }
  /* Value labels wear a halo of the surface they are drawn on. A label that
     sits above its point is still in the road of the line arriving at it from
     a steeper neighbour, or of a gridline, and half a label struck through is
     a wrong number. The halo is the surface colour rather than a white, so it
     is invisible on the page and still invisible on the dark one. */
  .t-value { font-size:12px; fill:var(--ink2); font-variant-numeric:tabular-nums;
             paint-order:stroke fill; stroke:var(--surface); stroke-width:3px;
             stroke-linejoin:round; }
  .t-key   { font-size:12px; fill:var(--ink2); }
  .t-foot  { font-size:11px; fill:var(--muted); }
  .grid { stroke:var(--grid); stroke-width:1; }
  .axis { stroke:var(--axis); stroke-width:1; }
  .s1 { fill:var(--s1); }
  .s2 { fill:var(--s2); }
  .line1 { stroke:var(--s1); stroke-width:2; stroke-linecap:round; stroke-linejoin:round; fill:none; }
  .dot1 { fill:var(--s1); stroke:var(--surface); stroke-width:2; }
</style>
`

const chartWidth = 760

// chartFooter is the machine line, repeated in each figure. A figure lifted
// out of the report and pasted somewhere else should still say what it was
// measured on, or it is a picture of a number rather than a measurement.
func chartFooter(info machineInfo) string { return info.describe() }

// svgHead opens a figure: the surface it is drawn on, the title, and the
// machine it was measured on.
//
// The subtitle is a list of lines because one of the figures has something to
// say about its own axis, and says it only when the measurement made it worth
// saying. A note that is always there about a case that has not happened is
// noise on every other run.
func svgHead(title string, subtitles []string, aria string, height float64, info machineInfo) *strings.Builder {
	var b strings.Builder

	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"%d\" height=\"%.0f\" viewBox=\"0 0 %d %.0f\" class=\"chart\" role=\"img\" aria-label=\"%s\">\n",
		chartWidth, height, chartWidth, height, html.EscapeString(aria))
	b.WriteString(chartStyle)
	b.WriteString("<g class=\"chart\">\n")
	fmt.Fprintf(&b, "<rect x=\"0.5\" y=\"0.5\" width=\"%d\" height=\"%.1f\" rx=\"8\" fill=\"var(--surface)\" stroke=\"var(--grid)\"/>\n",
		chartWidth-1, height-1)

	fmt.Fprintf(&b, "<text class=\"t-title\" x=\"28\" y=\"30\">%s</text>\n", html.EscapeString(title))

	for i, line := range subtitles {
		fmt.Fprintf(&b, "<text class=\"t-sub\" x=\"28\" y=\"%.0f\">%s</text>\n",
			52+float64(i)*20, html.EscapeString(line))
	}

	return &b
}

func svgTail(b *strings.Builder, height float64, info machineInfo) string {
	fmt.Fprintf(b, "<text class=\"t-foot\" x=\"28\" y=\"%.0f\">%s</text>\n",
		height-14, html.EscapeString(chartFooter(info)))
	b.WriteString("</g>\n</svg>\n")
	return b.String()
}

// chartKey draws a legend entry: a colored mark, then the words. The words are
// never the series color - identity comes from the swatch beside them.
func chartKey(b *strings.Builder, x, y float64, class, label string) {
	fmt.Fprintf(b, "<rect class=\"%s\" x=\"%s\" y=\"%s\" width=\"12\" height=\"12\" rx=\"3\"/>\n",
		class, f(x), f(y))
	fmt.Fprintf(b, "<text class=\"t-key\" x=\"%s\" y=\"%s\">%s</text>\n",
		f(x+18), f(y+10.5), html.EscapeString(label))
}

// barPath is a horizontal bar whose data-end is rounded and whose baseline end
// is square: the rounded corners are on the end that carries the value, and a
// baseline stays a baseline. The radius gives way to a short bar rather than
// overshooting it.
func barPath(x, y, w, h float64) string {
	r := math.Min(4, math.Min(w/2, h/2))

	return fmt.Sprintf("M %s %s H %s A %s %s 0 0 1 %s %s V %s A %s %s 0 0 1 %s %s Z",
		f(x), f(y), f(x+w-r), f(r), f(r), f(x+w), f(y+r),
		f(y+h-r), f(r), f(r), f(x+w-r), f(y+h))
}

// niceAxis rounds an axis top and picks a tick step for it, so that the longest
// bar fills most of the plot without the top tick being a number nobody would
// choose. It reads the largest value rather than being fixed, because this
// benchmark runs on machines three times slower than the one it was written
// on and a fixed axis would quietly clip their bars.
func niceAxis(maxValue float64) (top, step float64) {
	if maxValue <= 0 {
		return 10, 5
	}

	magnitude := math.Pow(10, math.Floor(math.Log10(maxValue)))
	best := math.Inf(1)

	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		for _, scale := range []float64{magnitude / 10, magnitude, magnitude * 10} {
			s := m * scale
			if s <= 0 {
				continue
			}

			candidate := math.Ceil(maxValue/s) * s

			// Three to six intervals: fewer and the reader is interpolating
			// between distant ticks, more and the axis becomes a ruler.
			if n := candidate / s; n < 3 || n > 6 {
				continue
			}

			if candidate < best {
				best, step = candidate, s
			}
		}
	}

	if math.IsInf(best, 1) {
		return math.Ceil(maxValue), math.Ceil(maxValue)
	}
	return best, step
}

// f formats a coordinate. Two decimals is finer than any screen this is drawn
// on, and a fixed width keeps the file diffable.
func f(v float64) string { return strconv.FormatFloat(v, 'f', 1, 64) }

// micros writes a duration a person can read: microseconds while they are
// small enough to be microseconds, and milliseconds once they are not.
func micros(v float64) string {
	if v < 1000 {
		return fmt.Sprintf("%.0f µs", v)
	}
	return fmt.Sprintf("%.1f ms", v/1000)
}

// percent is how a share is written when the whole point of showing it is that
// it gets small: one decimal below a tenth of a percent would round the
// smallest rows of the sweep to nothing.
func percent(v float64) string {
	if v < 1 {
		return fmt.Sprintf("%.2f%%", v)
	}
	return fmt.Sprintf("%.1f%%", v)
}

// axisPercent writes a tick on the percent axis. It is shorter than percent()
// because an axis is read as a scale and not as a figure: "-5%" is a tick, and
// the tenths a value label needs would be six characters of noise repeated
// down the left edge.
func axisPercent(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f%%", v)
	}
	return fmt.Sprintf("%.1f%%", v)
}

// roundTripSVG is the message-size chart: what one round trip costs, for both
// clients, at each size the report measures. Grouped bars rather than lines,
// because the reader's job is to compare two magnitudes at a named size, and
// bars from a common baseline do that without implying anything about the
// sizes in between, which were not measured.
func roundTripSVG(rows []row, info machineInfo) string {
	const (
		// The height leaves room under the axis for its caption and then for the
		// machine line, which are two separate pieces of small print and are not
		// allowed to look like one.
		height   = 486
		x0, x1   = 104.0, 688.0
		y0, y1   = 112.0, 400.0
		barH     = 22.0
		barGap   = 2.0
		labelPad = 8.0
	)

	sizes := configsOf(rows)

	maxValue := 0.0
	for _, r := range rows {
		if v := r.microsPerRoundTrip(); v > maxValue {
			maxValue = v
		}
	}
	top, step := niceAxis(maxValue)

	b := svgHead(
		"What a round trip costs, by message size",
		[]string{"One round trip is a send and the matching reply, over loopback TCP. The bar is the median of three whole client processes."},
		"Grouped bar chart of microseconds per round trip against message size, for a CCWIG client and a Go client.",
		height, info)

	// The legend names both series, so identity is never carried by color
	// alone. It sits on its own line rather than beside the title, because the
	// title is long enough that the two would collide on a narrow page.
	chartKey(b, 28, 70, "s1", "CCWIG client (interpreted)")
	chartKey(b, 268, 70, "s2", "Go client (the control)")

	// Gridlines first, so the bars are drawn over them; hairline and one step
	// off the surface, because the grid is not the data.
	for v := 0.0; v <= top+step/2; v += step {
		x := scale(v, top, x0, x1)
		fmt.Fprintf(b, "<line class=\"grid\" x1=\"%s\" y1=\"%s\" x2=\"%s\" y2=\"%s\"/>\n",
			f(x), f(y0), f(x), f(y1))
		fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%.0f\" text-anchor=\"middle\">%s</text>\n",
			f(x), y1+22, commas(int(math.Round(v))))
	}

	groupH := (y1 - y0) / float64(len(sizes))
	content := 2*barH + barGap

	for i, size := range sizes {
		groupTop := y0 + float64(i)*groupH + (groupH-content)/2

		fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%s\" text-anchor=\"end\">%s</text>\n",
			f(x0-14), f(groupTop+content/2+4), commas(size))

		for j, impl := range []string{"ccwig", "go"} {
			r, ok := findRow(rows, impl, size)
			if !ok {
				continue
			}

			y := groupTop + float64(j)*(barH+barGap)
			width := scale(r.microsPerRoundTrip(), top, x0, x1) - x0

			class := "s1"
			if impl == "go" {
				class = "s2"
			}

			fmt.Fprintf(b, "<path class=\"%s\" d=\"%s\"/>\n", class, barPath(x0, y, width, barH))

			// The CCWIG series is the one the chart is about, so it carries
			// its values; the control is carried by the axis, the legend and
			// the table underneath, which is where every value is anyway.
			if impl == "ccwig" {
				fmt.Fprintf(b, "<text class=\"t-value\" x=\"%s\" y=\"%s\">%.1f</text>\n",
					f(x0+width+labelPad), f(y+barH/2+4), r.microsPerRoundTrip())
			}
		}
	}

	fmt.Fprintf(b, "<line class=\"axis\" x1=\"%s\" y1=\"%s\" x2=\"%s\" y2=\"%s\"/>\n",
		f(x0), f(y0), f(x0), f(y1))
	fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%.0f\" text-anchor=\"middle\">microseconds per round trip</text>\n",
		f((x0+x1)/2), y1+42)

	return svgTail(b, height, info)
}

// scale places a value on an axis. Both charts use it, so that a bar and the
// axis it is read against cannot disagree about where a number is.
func scale(v, top, lo, hi float64) float64 {
	if top <= 0 {
		return lo
	}
	return lo + v/top*(hi-lo)
}

// shareSVG is the delay chart: how much of a round trip is still the
// interpreter once the round trip costs what a real one costs.
//
// The axis is linear and starts at zero, including when the data goes below
// it. A logarithmic axis was the first thing tried here and it was wrong, for
// a reason worth writing down: the difference between the two clients can come
// out negative. A run where the interpreted client finished ahead is a run
// whose difference was noise, and the report says so in prose - but a log axis
// has no room for a negative number, so it would have had to draw that row
// somewhere false. On a zero-baseline axis it lands just under the zero line,
// which is exactly what happened and exactly what it means.
//
// What the reader should take from it is the height of the first point and how
// little height everything after it has. The rows that are fractions of one per
// cent are past the resolution of a chart and are read off their labels.
func shareSVG(latency []latencyRow, info machineInfo) string {
	const (
		height = 428
		x0, x1 = 104.0, 688.0
		y0, y1 = 92.0, 324.0
	)

	b := svgHead(
		"The interpreter's share of a round trip, once the network costs something",
		shareSubtitles(latency),
		"Line chart of the interpreter's share of a round trip, in percent, against server delay.",
		height, info)

	lo, hi, step := shareAxis(latency)

	// The y axis carries percent. Its ticks are round numbers, and the zero
	// line is among them: it is the line a reader reads "nothing" off, and
	// every point but the first sits on or just above it.
	for i := 0; ; i++ {
		v := lo + float64(i)*step
		if v > hi+step/1000 {
			break
		}

		y := scaleFrom(v, lo, hi, y1, y0)

		class := "grid"
		if math.Abs(v) < step/1000 {
			class = "axis"
		}

		fmt.Fprintf(b, "<line class=\"%s\" x1=\"%s\" y1=\"%s\" x2=\"%s\" y2=\"%s\"/>\n",
			class, f(x0), f(y), f(x1), f(y))
		fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%s\" text-anchor=\"end\">%s</text>\n",
			f(x0-14), f(y+4), axisPercent(v))
	}

	// A point per row, spread evenly. The delays are equally spaced on the
	// page and are not equally spaced in fact - the axis is the conditions
	// that were run, not a ruler, and the labels say what each one cost.
	positions := make([]float64, len(latency))
	heights := make([]float64, len(latency))

	for i, r := range latency {
		positions[i] = x0 + (float64(i)+0.5)*(x1-x0)/float64(len(latency))
		heights[i] = scaleFrom(r.gapShare(), lo, hi, y1, y0)
	}

	// The line first, then the marks, then the words. Each layer is drawn over
	// the one before it, so no label is struck through by the line it labels.
	var line strings.Builder

	for i := range latency {
		if i == 0 {
			fmt.Fprintf(&line, "M %s %s", f(positions[i]), f(heights[i]))
			continue
		}
		fmt.Fprintf(&line, " L %s %s", f(positions[i]), f(heights[i]))
	}

	b.WriteString("<path class=\"line1\" d=\"" + line.String() + "\"/>\n")

	fmt.Fprintf(b, "<line class=\"axis\" x1=\"%s\" y1=\"%s\" x2=\"%s\" y2=\"%s\"/>\n",
		f(x0), f(y0), f(x0), f(y1))

	for i, r := range latency {
		fmt.Fprintf(b, "<circle class=\"dot1\" cx=\"%s\" cy=\"%s\" r=\"5\"/>",
			f(positions[i]), f(heights[i]))
		fmt.Fprintf(b, "<title>%s of a %s round trip at a %s delay</title>\n",
			percent(r.gapShare()), micros(r.ccwigMicros), delayLabel(r.delayMS))
	}

	for i, r := range latency {
		// Every point is labelled. This is not the flood the mark spec warns
		// about: four points is not a series, and the rows whose difference is
		// a fraction of a percent are past the resolution of the chart and
		// exist only as their numbers.
		//
		// A label goes below its point when the point is the lowest of its
		// neighbours, or when there is no room for it above; otherwise above.
		above := heights[i] - 14
		labelY := above
		if above < y0+12 {
			labelY = heights[i] + 24
		} else if i > 0 && i < len(latency)-1 &&
			r.gapShare() < latency[i-1].gapShare() && r.gapShare() < latency[i+1].gapShare() {
			labelY = heights[i] + 24
		}

		fmt.Fprintf(b, "<text class=\"t-value\" x=\"%s\" y=\"%s\" text-anchor=\"middle\">%s</text>\n",
			f(positions[i]), f(labelY), percent(r.gapShare()))

		fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%.0f\" text-anchor=\"middle\">%s</text>\n",
			f(positions[i]), y1+26, delayLabel(r.delayMS))
		fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%.0f\" text-anchor=\"middle\">%s</text>\n",
			f(positions[i]), y1+42, micros(r.ccwigMicros))
	}

	fmt.Fprintf(b, "<text class=\"t-tick\" x=\"%s\" y=\"%.0f\" text-anchor=\"middle\">server delay, and what a round trip cost at it</text>\n",
		f((x0+x1)/2), y1+68)

	return svgTail(b, height, info)
}

// shareSubtitles is what the figure says under its title. The second line
// appears only when a run actually drew below the zero line, because that is
// the only case in which a reader needs to be told what a negative share means
// - and a disclaimer printed under every chart is a disclaimer nobody reads.
func shareSubtitles(latency []latencyRow) []string {
	lines := []string{
		"CCWIG's round trip less Go's, as a percentage of CCWIG's, against a server told to wait before each reply.",
	}

	for _, r := range latency {
		if r.gapShare() < 0 {
			lines = append(lines, "A point below the zero line is a run where the two clients came out within the noise of each other.")
			break
		}
	}

	return lines
}

// shareAxis picks the ends and the tick step of the percent axis: zero up to a
// round number above the largest share, and below zero only if some run
// measured the interpreted client ahead.
func shareAxis(latency []latencyRow) (lo, hi, step float64) {
	smallest, largest := 0.0, 0.0

	for _, r := range latency {
		smallest = math.Min(smallest, r.gapShare())
		largest = math.Max(largest, r.gapShare())
	}

	if largest <= 0 {
		largest = 1
	}

	step = tickStep(largest - smallest)
	hi = math.Ceil(largest/step) * step
	lo = math.Floor(smallest/step) * step

	return lo, hi, step
}

// tickStep rounds a fifth of a span up to a step a person would choose, so an
// axis carries four or five intervals and no tick reads 3.7.
func tickStep(span float64) float64 {
	if span <= 0 {
		return 1
	}

	raw := span / 5
	decade := math.Pow(10, math.Floor(math.Log10(raw)))

	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if m*decade >= raw {
			return m * decade
		}
	}

	return 10 * decade
}

// scaleFrom places a value between two axis ends. It is separate from scale
// because a percent axis can run downwards, from a negative floor through zero
// to a ceiling, and a helper that assumed the smaller number was at the top
// would draw that axis upside down.
func scaleFrom(v, lo, hi, yAtLo, yAtHi float64) float64 {
	if hi == lo {
		return yAtHi
	}
	return yAtLo + (v-lo)/(hi-lo)*(yAtHi-yAtLo)
}

// delayLabel names one condition of the sweep. The zero row is not a delay of
// zero milliseconds; it is the loopback measurement, carried over from the
// main report, and it is labelled as what it is.
func delayLabel(delayMS int) string {
	if delayMS == 0 {
		return "loopback"
	}
	return strconv.Itoa(delayMS) + " ms"
}

// findRow is the row for one implementation at one message size.
func findRow(rows []row, impl string, size int) (row, bool) {
	for _, r := range rows {
		if r.impl == impl && r.size == size {
			return r, true
		}
	}
	return row{}, false
}

// writeCharts writes both figures.
func writeCharts(dir string, rows []row, latency []latencyRow, info machineInfo) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	figures := []struct {
		name string
		svg  string
	}{
		{"round-trip.svg", roundTripSVG(rows, info)},
		{"interpreter-share.svg", shareSVG(latency, info)},
	}

	for _, figure := range figures {
		path := filepath.Join(dir, figure.name)

		if err := os.WriteFile(path, []byte(figure.svg), 0o644); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}

	return nil
}
