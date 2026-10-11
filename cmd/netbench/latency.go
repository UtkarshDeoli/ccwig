package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The first report answers a question only loopback asks: how much of a round
// trip is the interpreter. On loopback the answer is "a few microseconds out
// of twenty", which is both the largest share the interpreter ever has and the
// only case where anyone would notice it. Send the same bytes to a server
// across a city and a round trip costs milliseconds, of which the interpreter
// is a rounding error -- but that is a claim until it is measured, and a claim
// about performance that was not measured is the kind this repository does not
// make.
//
// So the echo server here is told to wait before it answers, and the same two
// clients are measured against it. The delay is chosen by the harness and
// therefore known exactly, which is what makes the decomposition honest: what
// a row shows beyond the delay is what is not the network, and the only thing
// outside the network that differs between the two rows is the language.

// latencySize is the message size the sweep is made at. One size, so that the
// one thing changing down the table is the delay.
const latencySize = 1024

// latencyConfig is one server delay and the number of round trips to make
// against it. The counts fall as the delay rises so that no row takes minutes,
// but not as fast as the delay rises: the difference between the two clients
// is divided by the number of round trips, so a row with too few of them
// reports the machine's jitter with a decimal point on it. The counts here are
// the most round trips that fit in about two seconds at each delay.
type latencyConfig struct {
	delay time.Duration
	count int
}

// fullLatency ends at 50ms, which is roughly a round trip from a phone to a
// server on another continent -- the point past which the interpreter is no
// longer a thing anybody can feel.
//
// There is no zero-delay entry. The table's first row is the main report's
// 1,024-byte rows, taken from their loop clocks, because that is the one point
// where the difference between the two clients is a number rather than a
// ceiling and it deserves the fifteen thousand round trips the main report
// already spent on it. Measuring it again here with two thousand would be
// publishing the noisier of two numbers already in hand.
var fullLatency = []latencyConfig{
	{time.Millisecond, 600},
	{10 * time.Millisecond, 100},
	{50 * time.Millisecond, 40},
}

// quickLatency is a version short enough to run inside a test.
var quickLatency = []latencyConfig{
	{time.Millisecond, 100},
	{5 * time.Millisecond, 40},
}

// latencyRow is one server delay, with both clients measured against it.
//
// The microsecond figures are taken from each client's own clock rather than
// from the harness's, because the sweep's runs are short by design: at a 50 ms
// delay only ten round trips fit in half a second, and starting the
// interpreter - a cost that does not care how far away the server is - would
// otherwise be charged to each of those ten round trips. A column meant to be
// about the network would have become a column about process start-up.
type latencyRow struct {
	delayMS     int
	size        int
	count       int
	reps        int
	ccwigMicros float64
	goMicros    float64
	mismatches  int
}

// gapMicros is what the interpreter costs on top of the Go client: the same
// exchange, the same server, the same machine, one of the two interpreted.
func (l latencyRow) gapMicros() float64 { return l.ccwigMicros - l.goMicros }

// gapShare is that difference as a percentage of the round trip, which is the
// only figure a person can act on: microseconds are not what anybody feels,
// a proportion of the wait is.
func (l latencyRow) gapShare() float64 {
	if l.ccwigMicros <= 0 {
		return 0
	}
	return l.gapMicros() / l.ccwigMicros * 100
}

// latencySweep measures both clients once per delay. Each delay gets its own
// echo server, because the delay lives in the server: one server cannot be
// both fast and slow, and a report that measured a slow row against a server
// that had been warmed up by a fast one would be comparing two servers.
func latencySweep(opts options, ccwigPath, script string, configs []latencyConfig) ([]latencyRow, error) {
	var rows []latencyRow

	for _, cfg := range configs {
		address, stop, err := startEchoServer(cfg.delay)
		if err != nil {
			return nil, err
		}

		measured := make([]row, 0, 2)

		for _, impl := range []string{"ccwig", "go"} {
			// Every row of this table meets a brand new server, so every row
			// pays the cost of the first connection to one. Throwing a short
			// run away is what keeps that cost from being read as the
			// interpreter being slower at high latency than at low.
			if _, err := measure(opts.self, impl, ccwigPath, script, address,
				config{latencySize, warmUpTrips(cfg)}, 1, opts.timeout); err != nil {
				stop()
				return nil, fmt.Errorf("the warm-up run of %s at a %s delay failed: %w", impl, cfg.delay, err)
			}

			m, err := measure(opts.self, impl, ccwigPath, script, address,
				config{latencySize, cfg.count}, opts.reps, opts.timeout)
			if err != nil {
				stop()
				return nil, fmt.Errorf("%s at a %s delay: %w", impl, cfg.delay, err)
			}

			measured = append(measured, m)
		}

		stop()

		one := latencyRow{
			delayMS:     int(cfg.delay.Milliseconds()),
			size:        latencySize,
			count:       cfg.count,
			reps:        opts.reps,
			ccwigMicros: measured[0].microsPerRoundTripInLoop(),
			goMicros:    measured[1].microsPerRoundTripInLoop(),
			mismatches:  measured[0].mismatches + measured[1].mismatches,
		}
		rows = append(rows, one)

		fmt.Printf("delay %-5s %6d round trips  ccwig %10.1f us  go %10.1f us  gap %8.1f us  (%.2f%%)\n",
			cfg.delay, cfg.count, one.ccwigMicros, one.goMicros, one.gapMicros(), one.gapShare())
	}

	return rows, nil
}

// warmUpTrips is a quarter of the measured run, so that a delay that makes a
// round trip expensive does not make the warm-up expensive too, with a floor
// of five trips so that there is a warm-up at all.
func warmUpTrips(cfg latencyConfig) int {
	trips := cfg.count / 4
	if trips < 5 {
		trips = 5
	}
	return trips
}

// zeroDelayRow builds the first row of the sweep out of the main report's rows
// for latencySize, which are measured with many more round trips than a sweep
// row can afford and are therefore the better measurement of the same thing.
//
// It reports false when the main report did not measure that size, which is
// the case only if the configurations of the two reports have been changed to
// disagree -- and then the sweep simply starts at its own first delay, which
// is a shorter table rather than a wrong one.
func zeroDelayRow(rows []row) (latencyRow, bool) {
	var ccwig, goRow *row

	for i := range rows {
		if rows[i].size != latencySize {
			continue
		}

		switch rows[i].impl {
		case "ccwig":
			ccwig = &rows[i]
		case "go":
			goRow = &rows[i]
		}
	}

	if ccwig == nil || goRow == nil {
		return latencyRow{}, false
	}

	return latencyRow{
		delayMS:     0,
		size:        latencySize,
		count:       ccwig.count,
		reps:        ccwig.reps,
		ccwigMicros: ccwig.microsPerRoundTripInLoop(),
		goMicros:    goRow.microsPerRoundTripInLoop(),
		mismatches:  ccwig.mismatches + goRow.mismatches,
	}, true
}

// writeLatencyReports writes the sweep as data and as prose. It is separate
// from writeReports because the two tables have different columns and answer
// different questions, and a CSV that tried to hold both would be a CSV that
// held neither well.
func writeLatencyReports(dir string, rows []latencyRow, info machineInfo) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	writers := []struct {
		name string
		fn   func(*os.File) error
	}{
		{"latency.csv", func(f *os.File) error { return writeLatencyCSV(f, rows) }},
		{"latency.md", func(f *os.File) error { return writeLatencyMarkdown(f, rows, info) }},
	}

	for _, w := range writers {
		path := filepath.Join(dir, w.name)

		f, err := os.Create(path)
		if err != nil {
			return err
		}

		if err := w.fn(f); err != nil {
			f.Close()
			return fmt.Errorf("%s: %w", path, err)
		}

		if err := f.Close(); err != nil {
			return err
		}
	}

	return nil
}

func writeLatencyCSV(f *os.File, rows []latencyRow) error {
	w := csv.NewWriter(f)

	head := []string{
		"server_delay_ms", "message_size_bytes", "round_trips", "runs",
		"ccwig_micros_per_round_trip", "go_micros_per_round_trip",
		"difference_micros", "difference_percent_of_round_trip", "mismatches",
	}
	if err := w.Write(head); err != nil {
		return err
	}

	for _, r := range rows {
		record := []string{
			strconv.Itoa(r.delayMS),
			strconv.Itoa(r.size),
			strconv.Itoa(r.count),
			strconv.Itoa(r.reps),
			fmt.Sprintf("%.2f", r.ccwigMicros),
			fmt.Sprintf("%.2f", r.goMicros),
			fmt.Sprintf("%.2f", r.gapMicros()),
			fmt.Sprintf("%.3f", r.gapShare()),
			strconv.Itoa(r.mismatches),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}

func writeLatencyMarkdown(f *os.File, rows []latencyRow, info machineInfo) error {
	var b strings.Builder

	b.WriteString("# What a round trip costs when the network costs something\n\n")
	b.WriteString("The [main report](netbench.md) puts two clients against each other on loopback,\n")
	b.WriteString("where a round trip is about twenty microseconds and a few of them are the\n")
	b.WriteString("interpreter. Loopback is the one network on which that share is large, and the\n")
	b.WriteString("one network on which it does not matter to anybody.\n\n")
	b.WriteString("These are the same two clients and the same exchange against a server that waits\n")
	b.WriteString("before it answers, so that a round trip costs what a real one costs. Both clients\n")
	b.WriteString("meet that server, in the same order, on the same machine, with the same wait in\n")
	b.WriteString("front of each reply; the only thing that differs between the two rows of a line is\n")
	b.WriteString("the language the client was written in.\n\n")

	b.WriteString("| server delay | CCWIG µs/round trip | Go µs/round trip | difference | CCWIG's share |\n")
	b.WriteString("| ---: | ---: | ---: | ---: | ---: |\n")

	for _, r := range rows {
		fmt.Fprintf(&b, "| %d ms | %.1f | %.1f | %.1f | %.2f%% |\n",
			r.delayMS, r.ccwigMicros, r.goMicros, r.gapMicros(), r.gapShare())
	}

	b.WriteString("\n")
	writeLatencySentence(&b, rows)

	b.WriteString("## What is measured\n\n")
	b.WriteString("- Every figure is the client's own clock divided by the round trips, median of the\n")
	b.WriteString("  runs. That clock starts at the first send and stops at the last receive, and is\n")
	b.WriteString("  the one difference from the [main report](netbench.md), which measures the whole\n")
	b.WriteString("  process from `fork` to exit.\n")
	b.WriteString("- The main report charges every round trip for starting the interpreter, which is\n")
	b.WriteString("  right when a run is twenty thousand round trips and the start-up is spread thin.\n")
	b.WriteString("  Here a run is as few as a few dozen, so the same charge would put the start-up cost\n")
	b.WriteString("  into a column about the network - which would look exactly like the interpreter\n")
	b.WriteString("  getting slower as the server got further away.\n")
	b.WriteString("- The first row is not measured here. It is the main report's rows for this\n")
	b.WriteString("  message size, read off their own clocks, because the zero-delay point is the one\n")
	b.WriteString("  point in this table where the difference between the clients is a number rather\n")
	b.WriteString("  than a ceiling, and it is worth the many thousand round trips the main report\n")
	b.WriteString("  spends on it rather than the few hundred a swept row can afford.\n")
	b.WriteString("- The message is 1,024 bytes at every delay, so the one thing changing down the\n")
	b.WriteString("  table is the delay.\n")
	b.WriteString("- The round trip is not the delay. A thread that asks to sleep for a millisecond\n")
	b.WriteString("  is woken late, on a shared machine by more than a millisecond, so a row's round\n")
	b.WriteString("  trip is longer than the delay it was asked for - and by an amount that is the\n")
	b.WriteString("  machine's, not the language's. That is why the difference column compares the two\n")
	b.WriteString("  clients against each other rather than either one against the delay. Both clients\n")
	b.WriteString("  meet the same server with the same overrun; the only thing that differs is the\n")
	b.WriteString("  language the client was written in.\n\n")

	b.WriteString("## What this does not say\n\n")
	b.WriteString("A `time.Sleep` is not a network. A real link has jitter, bandwidth that falls under\n")
	b.WriteString("load, and a delay that is not the same in both directions; this has a fixed pause\n")
	b.WriteString("before each reply and nothing else. What it reproduces is the one property that\n")
	b.WriteString("decides whether the interpreter matters -- a round trip that costs time the\n")
	b.WriteString("interpreter is not spending -- and it does not reproduce the rest.\n\n")

	b.WriteString("## How to reproduce\n\n")
	b.WriteString("```sh\nmake report\n```\n\n")

	b.WriteString("## The machine this ran on\n\n")
	b.WriteString("    " + info.describe() + "\n\n")

	if info.Quick {
		b.WriteString("**This report was made with `-quick`**, which shortens every run. The figures are\n")
		b.WriteString("noisy and are here as a smoke check rather than as a benchmark.\n\n")
	}

	b.WriteString("## Mismatches\n\n")
	if bad := latencyMismatches(rows); bad > 0 {
		fmt.Fprintf(&b, "**%d replies did not match their request.** The numbers above are worth nothing\n", bad)
		b.WriteString("until that is understood.\n")
	} else {
		b.WriteString("Every reply matched its request byte for byte, at every delay, for both clients.\n")
	}

	if _, err := io.WriteString(f, b.String()); err != nil {
		return err
	}
	return nil
}

// writeLatencySentence is the paragraph under the table, and every number in it
// is read off the rows. A sentence about a measurement that was written by hand
// is a sentence that starts lying the first time the measurement changes.
//
// The claim is stated as a bound rather than as a value, and that is the
// honest form for this table. A run of forty round trips cannot resolve a
// difference of a few microseconds: one scheduling hiccup costs more than the
// thing being looked for, so a row's difference column is as likely to be
// noise as signal. What the table can support is the largest difference it
// saw, which is an upper bound on what the interpreter costs and needs no
// faith in any single row.
func writeLatencySentence(b *strings.Builder, rows []latencyRow) {
	first, last, ok := latencyEnds(rows)
	if !ok {
		b.WriteString("The difference between the two clients is in the CSV, where every figure is.\n\n")
		return
	}

	worst := latencyWidestGap(rows)

	fmt.Fprintf(b, "The round trip grows from **%.0f µs** to **%.0f µs** down this table, and the\n", first.ccwigMicros, last.ccwigMicros)
	fmt.Fprintf(b, "difference between the two clients never exceeded **%.0f µs** of it. At the longest\n", worst)
	fmt.Fprintf(b, "round trip here - %d ms of delay - %.0f µs is **%.2f%%** of what a person would wait.\n\n", last.delayMS, worst, worst/last.ccwigMicros*100)

	b.WriteString("That is a bound rather than a measurement, and deliberately. The difference is\n")
	b.WriteString("divided by the number of round trips, so the fewer of them a row has, the more of\n")
	b.WriteString("the machine's own jitter is in it: one scheduling hiccup of a millisecond, spread\n")
	b.WriteString("over a few dozen round trips, is tens of microseconds that have nothing to do with\n")
	b.WriteString("the language. A row that short states a ceiling the noise decided, which is still\n")
	b.WriteString("an honest thing to state, and is the only honest thing that length supports.\n\n")

	if first.gapMicros() > 0 {
		fmt.Fprintf(b, "The one row where the difference is a number rather than a ceiling is the first,\n")
		fmt.Fprintf(b, "because it is the one row with enough round trips in it: **%.1f µs** of the %.1f µs\n", first.gapMicros(), first.ccwigMicros)
		fmt.Fprintf(b, "round trip, or **%.1f%%**. That is the interpreter, measured where it can be: on\n", first.gapShare())
		b.WriteString("loopback, which is the only network where the interpreter is not the small part.\n\n")
	} else {
		fmt.Fprintf(b, "Even the row with no delay in it shows the two clients within %.1f µs of each other,\n", math.Abs(first.gapMicros()))
		b.WriteString("which means this run could not resolve the interpreter anywhere - the machine was\n")
		b.WriteString("too busy to be measured on.\n\n")
	}
}

// latencyEnds returns the rows with the smallest and largest delay, which is
// what a sentence about a table like this is about.
func latencyEnds(rows []latencyRow) (first, last latencyRow, ok bool) {
	for i, r := range rows {
		if i == 0 {
			first, last, ok = r, r, true
			continue
		}
		if r.delayMS < first.delayMS {
			first = r
		}
		if r.delayMS > last.delayMS {
			last = r
		}
	}
	return first, last, ok
}

// latencyWidestGap is the largest difference between the clients anywhere in
// the table, sign ignored: a run where the interpreted client came out ahead
// is a run whose difference was noise, and noise is what an upper bound is
// for.
func latencyWidestGap(rows []latencyRow) float64 {
	worst := 0.0

	for _, r := range rows {
		if gap := math.Abs(r.gapMicros()); gap > worst {
			worst = gap
		}
	}

	return worst
}

func latencyMismatches(rows []latencyRow) int {
	total := 0
	for _, r := range rows {
		total += r.mismatches
	}
	return total
}
