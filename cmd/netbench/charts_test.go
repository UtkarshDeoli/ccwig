package main

import (
	"encoding/xml"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"
)

// testRows is a small stand-in for a run of the harness, with the shapes that
// matter: two implementations at two sizes, and a sweep that starts high,
// comes down to nothing, and dips below zero at one delay.
var testRows = []row{
	{impl: "ccwig", size: 64, count: 1000, reps: 3, wallMS: 12, loopUS: 9000},
	{impl: "go", size: 64, count: 1000, reps: 3, wallMS: 9, loopUS: 7000},
	{impl: "ccwig", size: 1024, count: 500, reps: 3, wallMS: 14, loopUS: 11000},
	{impl: "go", size: 1024, count: 500, reps: 3, wallMS: 10, loopUS: 8000},
}

var testSweep = []latencyRow{
	{delayMS: 0, size: 1024, count: 500, reps: 3, ccwigMicros: 20, goMicros: 15},
	{delayMS: 1, size: 1024, count: 600, reps: 3, ccwigMicros: 1300, goMicros: 1400},
	{delayMS: 10, size: 1024, count: 100, reps: 3, ccwigMicros: 11000, goMicros: 10900},
	{delayMS: 50, size: 1024, count: 40, reps: 3, ccwigMicros: 51000, goMicros: 50900},
}

var testInfo = machineInfo{
	Go: "go1.24.4", OS: "darwin", Arch: "arm64", CPUs: 10,
	Date: "2026-10-12 04:00 UTC", Commit: "abc1234", Reps: 3,
}

// TestFiguresAreWellFormedXML is the floor under every other test of these
// files. A figure is rendered by a browser that is far more forgiving than a
// repository: it shrugs at a stray angle bracket and draws something anyway,
// and what it draws is not what was measured.
func TestFiguresAreWellFormedXML(t *testing.T) {
	figures := map[string]string{
		"round-trip.svg":        roundTripSVG(testRows, testInfo),
		"interpreter-share.svg": shareSVG(testSweep, testInfo),
	}

	for name, figure := range figures {
		var root struct {
			XMLName xml.Name
			Aria    string `xml:"aria-label,attr"`
		}

		if err := xml.Unmarshal([]byte(figure), &root); err != nil {
			t.Errorf("%s is not well-formed XML: %v", name, err)
			continue
		}
		if root.XMLName.Local != "svg" {
			t.Errorf("%s has a <%s> at its root", name, root.XMLName.Local)
		}

		// A figure nobody can describe out loud is a figure a screen reader
		// cannot read.
		if strings.TrimSpace(root.Aria) == "" {
			t.Errorf("%s has no aria-label", name)
		}
	}
}

// TestFiguresCarryTheirOwnValues checks the one thing a hand-written caption
// cannot: that the numbers in the picture are the numbers in the data. This is
// the same rule the tables are held to, and a figure that skips it is a second
// description of the measurement that is free to drift from the first.
func TestFiguresCarryTheirOwnValues(t *testing.T) {
	bar := roundTripSVG(testRows, testInfo)

	for _, r := range testRows {
		if r.impl != "ccwig" {
			continue
		}

		value := fmt.Sprintf("%.1f", r.microsPerRoundTrip())
		if !hasLabeledText(bar, "t-value", value) {
			t.Errorf("the round-trip figure does not label the CCWIG bar at %d bytes with %s", r.size, value)
		}
	}

	// Every row of the sweep is in the figure, including the one drawn below
	// the zero line - which is the row a chart is most likely to drop, because
	// a log axis cannot draw it at all.
	line := shareSVG(testSweep, testInfo)

	for _, r := range testSweep {
		if value := percent(r.gapShare()); !hasLabeledText(line, "t-value", value) {
			t.Errorf("the share figure does not label the row at %d ms (%s) with %s", r.delayMS, delayLabel(r.delayMS), value)
		}
	}
}

// TestSweepFigureExplainsANegativeRun checks the one sentence the figure adds
// about itself, and that it adds it only when there is something to explain: a
// run where the interpreted client finished ahead is not a run where the
// interpreter was faster, and without the note the point reads as though it
// were.
func TestSweepFigureExplainsANegativeRun(t *testing.T) {
	withNegative := shareSVG(testSweep, testInfo)

	if !strings.Contains(withNegative, "below the zero line") {
		t.Error("the sweep includes a run where the interpreted client came out ahead, and the figure does not say what that means")
	}

	positive := make([]latencyRow, len(testSweep))
	copy(positive, testSweep)
	positive[1].goMicros = 1000 // still below the CCWIG client

	if figure := shareSVG(positive, testInfo); strings.Contains(figure, "below the zero line") {
		t.Error("the figure explains a negative share in a run where every share is positive")
	}
}

// TestSweepAxisHoldsEveryPoint is the check that a figure cannot fail by being
// drawn outside itself. The loopback row is a fifth of its round trip on a
// quiet machine and was a third of it on a busy one, so an axis end that was
// chosen by hand rather than by the data draws that point over the title.
func TestSweepAxisHoldsEveryPoint(t *testing.T) {
	const y0, y1 = 92.0, 324.0

	for _, sweep := range [][]latencyRow{
		testSweep,
		{{ccwigMicros: 18, goMicros: 14}, {ccwigMicros: 1300, goMicros: 1400}, {ccwigMicros: 52000, goMicros: 51900}},
		{{ccwigMicros: 18, goMicros: 14}, {ccwigMicros: 1300, goMicros: 1300}, {ccwigMicros: 52000, goMicros: 53100}},
		{{ccwigMicros: 18, goMicros: 17}, {ccwigMicros: 1300, goMicros: 1290}},
	} {
		lo, hi, step := shareAxis(sweep)

		if lo > 0 {
			t.Errorf("the axis starts at %s, above the zero line every share is read against", axisPercent(lo))
		}
		if hi <= 0 {
			t.Errorf("the axis ends at %s, which leaves nowhere to draw a share", axisPercent(hi))
		}
		if step <= 0 {
			t.Errorf("the axis has a tick step of %s", axisPercent(step))
		}

		for _, r := range sweep {
			y := scaleFrom(r.gapShare(), lo, hi, y1, y0)

			if y < y0 || y > y1 {
				t.Errorf("a share of %s is drawn at y=%s, outside the plot (%s to %s)",
					percent(r.gapShare()), f(y), f(y0), f(y1))
			}
		}

		if ticks := int((hi - lo) / step); ticks < 3 || ticks > 7 {
			t.Errorf("an axis from %s to %s in steps of %s is %d intervals, want 3 to 7",
				axisPercent(lo), axisPercent(hi), axisPercent(step), ticks)
		}
	}
}

// TestTickStepIsARoundNumber is about the reader rather than the drawing. A
// step of 3.7 needs a label on every gridline to be readable at all, and there
// is no room for that on an axis.
func TestTickStepIsARoundNumber(t *testing.T) {
	for _, span := range []float64{0.02, 0.7, 3, 9, 18.27, 24.4, 100, 12345} {
		step := tickStep(span)

		if step <= 0 {
			t.Fatalf("a span of %g has a step of %g", span, step)
		}

		// A round step is one or two significant figures off a 1-2-2.5-5
		// ladder: 0.5, 2, 2.5, 20. It is not 0.37.
		decade := math.Pow(10, math.Floor(math.Log10(step)+1e-9))
		digit := step / decade

		round := false
		for _, want := range []float64{1, 2, 2.5, 5, 10} {
			if math.Abs(digit-want) < 1e-9 {
				round = true
			}
		}

		if !round {
			t.Errorf("a span of %g has a tick step of %g, which is not a number a reader would choose", span, step)
		}

		if intervals := span / step; intervals < 2 || intervals > 12 {
			t.Errorf("a span of %g in steps of %g is %g intervals", span, step, intervals)
		}
	}
}

// TestBarEndsAreRoundedAndBaselinesAreSquare checks the mark spec at the level
// of the path, because it is the kind of thing that is invisible in a diff and
// obvious in a screenshot: a bar with both ends rounded reads as a lozenge and
// stops being measurable from the baseline.
func TestBarEndsAreRoundedAndBaselinesAreSquare(t *testing.T) {
	d := barPath(100, 10, 120, 22)

	if !strings.HasPrefix(d, "M 100.0 10.0 H ") {
		t.Errorf("the bar does not start square at its baseline: %s", d)
	}
	// Two rounded corners, both on the end that carries the value.
	if got := strings.Count(d, "A "); got != 2 {
		t.Errorf("the bar has %d rounded corners, want 2: %s", got, d)
	}
	if !strings.Contains(d, "H 216.0") {
		t.Errorf("the bar's data-end is not at 216 for a 120-wide bar on a 100 baseline: %s", d)
	}

	// A bar shorter than its own radius gives the corner up rather than
	// overshooting into the neighbouring group.
	if d := barPath(0, 0, 3, 22); strings.Contains(d, "A 4") {
		t.Errorf("a 3-wide bar used a 4 radius: %s", d)
	}
}

// hasLabeledText looks for a text element of the given class whose content is
// exactly this string. The class is part of the match so that a value label is
// not confused with an axis tick that happens to read the same.
func hasLabeledText(svg, class, text string) bool {
	pattern := regexp.MustCompile(`<text class="` + regexp.QuoteMeta(class) + `"[^>]*>` + regexp.QuoteMeta(text) + `</text>`)
	return pattern.MatchString(svg)
}
