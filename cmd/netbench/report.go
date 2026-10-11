package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// row is one line of the report: one client implementation, run at one message
// size. Everything in the writers below is derived from these five numbers
// plus the count, so there is one place where a figure is defined.
type row struct {
	impl       string
	size       int
	count      int
	reps       int
	wallMS     float64
	loopUS     int64
	mismatches int
}

// microsPerRoundTrip is the latency a round trip was measured at. It is the
// number a person means by "how fast is this": send, wait, come back.
func (r row) microsPerRoundTrip() float64 { return r.wallMS * 1000 / float64(r.count) }

// microsPerRoundTripInLoop is the same figure taken from the client's own
// clock rather than from the harness's. It leaves out the cost of starting the
// interpreter, which the wall clock charges to every round trip and which has
// nothing to do with how far away the server is - which is what makes it the
// right figure for the latency sweep, where the number of round trips in a run
// is small enough for the start-up cost to be a large part of each one.
func (r row) microsPerRoundTripInLoop() float64 {
	return float64(r.loopUS) / float64(r.count)
}

func (r row) roundTripsPerSecond() float64 { return float64(r.count) / (r.wallMS / 1000) }

// megabytesPerSecond counts both directions. Every byte is sent and comes
// back, and a figure that counted only one of those would be half the traffic
// that actually crossed the socket.
func (r row) megabytesPerSecond() float64 {
	return float64(r.count*r.size*2) / (r.wallMS / 1000) / (1 << 20)
}

// machineInfo is what the report says about the machine the numbers came from.
// A throughput figure with no machine attached cannot be compared with
// anything, which makes it a decoration rather than a measurement.
type machineInfo struct {
	Go     string
	OS     string
	Arch   string
	CPUs   int
	Date   string
	Commit string
	Note   string
	Reps   int
	Quick  bool
}

func (m machineInfo) describe() string {
	parts := []string{
		fmt.Sprintf("go %s", strings.TrimPrefix(m.Go, "go")),
		fmt.Sprintf("%s/%s", m.OS, m.Arch),
		fmt.Sprintf("%d CPUs", m.CPUs),
		fmt.Sprintf("commit %s", m.Commit),
		fmt.Sprintf("%s, median of %d runs", m.Date, m.Reps),
	}
	if m.Note != "" {
		parts = append(parts, m.Note)
	}
	return strings.Join(parts, " · ")
}

// writeReports writes the three files of the main report. The CSV is the data,
// the Markdown is what GitHub renders, and the HTML is what a GitHub Pages
// site serves; all three come from the same rows.
func writeReports(dir string, rows []row, info machineInfo) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	writers := []struct {
		name string
		fn   func(*os.File) error
	}{
		{"netbench.csv", func(f *os.File) error { return writeCSV(f, rows) }},
		{"netbench.md", func(f *os.File) error { return writeMarkdown(f, rows, info) }},
		{"index.html", func(f *os.File) error { return writeHTML(f, rows, info) }},
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

func writeCSV(f *os.File, rows []row) error {
	w := csv.NewWriter(f)

	head := []string{
		"impl", "message_size_bytes", "round_trips", "runs",
		"wall_ms", "loop_us", "micros_per_round_trip",
		"round_trips_per_second", "megabytes_per_second", "mismatches",
	}
	if err := w.Write(head); err != nil {
		return err
	}

	for _, r := range rows {
		record := []string{
			r.impl,
			strconv.Itoa(r.size),
			strconv.Itoa(r.count),
			strconv.Itoa(r.reps),
			fmt.Sprintf("%.1f", r.wallMS),
			strconv.FormatInt(r.loopUS, 10),
			fmt.Sprintf("%.2f", r.microsPerRoundTrip()),
			fmt.Sprintf("%.0f", r.roundTripsPerSecond()),
			fmt.Sprintf("%.2f", r.megabytesPerSecond()),
			strconv.Itoa(r.mismatches),
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}

	w.Flush()
	return w.Error()
}

func writeMarkdown(f *os.File, rows []row, info machineInfo) error {
	var b strings.Builder

	b.WriteString("# CCWIG network benchmark\n\n")
	b.WriteString("How fast can a CCWIG program move bytes over a loopback TCP connection?\n\n")
	b.WriteString("Each row is the same exchange - send a payload of *message size* bytes, wait for\n")
	b.WriteString("the same number of bytes to come back - made *round trips* times, over the same\n")
	b.WriteString("socket to the same echo server. The `go` rows are that same exchange written in\n")
	b.WriteString("Go and run as a separate process. They are the control: without them a latency\n")
	b.WriteString("figure says how fast the machine is, and with them it says how much of the time\n")
	b.WriteString("is the language.\n\n")

	b.WriteString("| client | message size | round trips | wall ms | µs per round trip | round trips/s | MB/s |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")

	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %.0f | %.1f | %s | %.1f |\n",
			r.impl, commas(r.size), commas(r.count), r.wallMS,
			r.microsPerRoundTrip(), commas(int(r.roundTripsPerSecond())), r.megabytesPerSecond())
	}

	b.WriteString("\n")

	if ratio, size, ok := widestRatio(rows); ok {
		if ratio >= 1 {
			fmt.Fprintf(&b, "At %s-byte messages a round trip costs the CCWIG client **%.2f× what it costs\n", commas(size), ratio)
			b.WriteString("the Go one**. The difference is not the socket: both clients make the same system\n")
			b.WriteString("calls, in the same order, to the same server. It is what happens between them -\n")
			b.WriteString("a tree-walking interpreter calling two builtins per round trip, and building and\n")
			b.WriteString("comparing payloads a byte at a time.\n\n")
		} else {
			fmt.Fprintf(&b, "**At every size measured the CCWIG client kept up with the Go one** - at its\n")
			fmt.Fprintf(&b, "worst, %s-byte messages, a round trip cost it %.2f× what it cost Go. That is what\n", commas(size), ratio)
			b.WriteString("it looks like when the work being timed is the kernel's: at these sizes a loopback\n")
			b.WriteString("round trip is mostly the two system calls, and both clients pay the same price for\n")
			b.WriteString("them.\n\n")
		}
	} else {
		b.WriteString("The ratios between the two clients are in the CSV, where every figure is.\n\n")
	}

	b.WriteString("## What is measured\n\n")
	b.WriteString("- **wall ms** is the whole client process, from `fork` to exit: reading the source,\n")
	b.WriteString("  parsing it, connecting, and doing the work.\n")
	b.WriteString("- **µs per round trip** is `wall ms` divided by the number of round trips. It is the\n")
	b.WriteString("  number a person means by latency.\n")
	b.WriteString("- **MB/s** counts both directions, because every byte is sent and comes back.\n")
	b.WriteString("- **loop_us** in the CSV is what the client itself timed with its own clock, between\n")
	b.WriteString("  the first send and the last receive. It is lower than `wall ms` by the cost of\n")
	b.WriteString("  starting up, and the two agreeing closely is what says nothing outside the loop\n")
	b.WriteString("  is taking the time. It is in microseconds rather than milliseconds because the\n")
	b.WriteString("  [latency sweep](latency.md) reads its first row off this column, and that row is\n")
	b.WriteString("  a few microseconds wide.\n")
	b.WriteString("- Every figure is the **median of the runs**, not the fastest: a loopback benchmark on\n")
	b.WriteString("  a machine doing anything else has occasional slow runs and no occasional fast ones,\n")
	b.WriteString("  so the middle of the distribution is the honest number and the best one is not.\n\n")

	b.WriteString("## What is not measured\n\n")
	b.WriteString("This is not a measurement of the kernel's TCP. Loopback is the fastest and least\n")
	b.WriteString("interesting network there is, and the echo server is a copy in both directions with\n")
	b.WriteString("nothing in between. What the numbers are about is the interpreter: how much work\n")
	b.WriteString("stands between a program saying `net_send` and the bytes leaving the process.\n\n")
	b.WriteString("Nothing here is a comparison with another language's TCP library, and the Go rows\n")
	b.WriteString("are not a claim that Go is faster than CCWIG in general. They are the control: the\n")
	b.WriteString("same work, same machine, no interpreter.\n\n")

	b.WriteString("## How to reproduce\n\n")
	b.WriteString("```sh\nmake report\n```\n\n")
	b.WriteString("or, without make:\n\n")
	b.WriteString("```sh\ngo build -o bin/ccwig ./cmd/ccwig\ngo run ./cmd/netbench -out reports\n```\n\n")

	b.WriteString("## The machine this ran on\n\n")
	b.WriteString("    " + info.describe() + "\n\n")

	if info.Quick {
		b.WriteString("**This report was made with `-quick`**, which shortens every run. The figures are\n")
		b.WriteString("noisy and are here as a smoke check rather than as a benchmark.\n\n")
	}

	b.WriteString("## Mismatches\n\n")
	if bad := totalMismatches(rows); bad > 0 {
		fmt.Fprintf(&b, "**%d replies did not match their request.** The numbers above are worth nothing\n", bad)
		b.WriteString("until that is understood.\n")
	} else {
		b.WriteString("Every reply matched its request byte for byte, in every run of every row. A\n")
		b.WriteString("benchmark of a client that is not moving the right bytes is a benchmark of the\n")
		b.WriteString("wrong program.\n")
	}

	if _, err := io.WriteString(f, b.String()); err != nil {
		return err
	}
	return nil
}

func writeHTML(f *os.File, rows []row, info machineInfo) error {
	fastest := 0.0
	for _, r := range rows {
		if mb := r.megabytesPerSecond(); mb > fastest {
			fastest = mb
		}
	}

	var b strings.Builder

	b.WriteString(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>CCWIG network benchmark</title>
<style>
  :root { color-scheme: light dark; }
  body { margin: 0 auto; padding: 2rem 1.5rem 4rem; max-width: 60rem;
         font: 16px/1.5 ui-sans-serif, system-ui, -apple-system, sans-serif; }
  h1 { font-size: 1.6rem; margin-bottom: .25rem; }
  p.lead { margin-top: 0; opacity: .8; }
  table { border-collapse: collapse; width: 100%; margin: 1.5rem 0; font-variant-numeric: tabular-nums; }
  th, td { padding: .5rem .6rem; border-bottom: 1px solid rgba(128,128,128,.3); text-align: right; }
  th:first-child, td:first-child { text-align: left; }
  thead th { border-bottom-width: 2px; }
  .bar { background: rgba(128,128,128,.25); border-radius: 2px; height: .7rem; min-width: 1px; }
  .impl-ccwig .bar { background: #4a7fe0; }
  .impl-go .bar { background: #d2713c; }
  .bar-cell { width: 30%; }
  ul { padding-left: 1.2rem; }
  li { margin: .3rem 0; }
  footer { margin-top: 2rem; opacity: .7; font-size: .9rem; }
  code { font-size: .95em; }
</style>
</head>
<body>
`)

	b.WriteString("<h1>CCWIG network benchmark</h1>\n")
	b.WriteString("<p class=\"lead\">How fast a CCWIG program moves bytes over a loopback TCP connection, against the same exchange written in Go.</p>\n")

	b.WriteString("<table>\n<thead><tr>")
	b.WriteString("<th>client</th><th>message size</th><th>round trips</th><th>wall ms</th>")
	b.WriteString("<th>µs per round trip</th><th>round trips/s</th><th>MB/s</th><th class=\"bar-cell\">throughput</th>")
	b.WriteString("</tr></thead>\n<tbody>\n")

	for _, r := range rows {
		width := 0.0
		if fastest > 0 {
			width = r.megabytesPerSecond() / fastest * 100
		}

		fmt.Fprintf(&b, "<tr class=\"impl-%s\"><td>%s</td><td>%s</td><td>%s</td><td>%.0f</td><td>%.1f</td><td>%s</td><td>%.1f</td>",
			html.EscapeString(r.impl), html.EscapeString(r.impl), commas(r.size), commas(r.count),
			r.wallMS, r.microsPerRoundTrip(), commas(int(r.roundTripsPerSecond())), r.megabytesPerSecond())
		fmt.Fprintf(&b, "<td class=\"bar-cell\"><div class=\"bar\" style=\"width:%.1f%%\"></div></td></tr>\n", width)
	}

	b.WriteString("</tbody>\n</table>\n")

	b.WriteString("<h2>What is measured</h2>\n<ul>\n")
	b.WriteString("<li><strong>wall ms</strong> is the whole client process: reading the source, parsing it, connecting, and doing the work.</li>\n")
	b.WriteString("<li><strong>µs per round trip</strong> is <code>wall ms</code> divided by the round trips &mdash; the number a person means by latency.</li>\n")
	b.WriteString("<li><strong>MB/s</strong> counts both directions, because every byte is sent and comes back.</li>\n")
	b.WriteString("<li>Each row is the middle of the runs, not the fastest: a loopback benchmark on a busy machine has occasional slow runs and no occasional fast ones.</li>\n")
	b.WriteString("</ul>\n")

	b.WriteString("<h2>What is not measured</h2>\n")
	b.WriteString("<p>Not the kernel's TCP. Loopback is the fastest and least interesting network there is. The numbers are about the interpreter: how much work stands between a program saying <code>net_send</code> and the bytes leaving the process. The Go rows are the control for that &mdash; the same work, same machine, no interpreter.</p>\n")

	b.WriteString("<h2>When the network costs something</h2>\n")
	b.WriteString("<p>Loopback is the one network on which the interpreter's share of a round trip is large, and the one on which it does not matter to anybody. The <a href=\"latency.md\">latency sweep</a> asks the other question: the same two clients, the same exchange, against a server that waits before it answers, so that a round trip costs what a real one costs.</p>\n")

	b.WriteString("<h2>How to reproduce</h2>\n")
	b.WriteString("<pre><code>make report</code></pre>\n")
	b.WriteString("<p>The data behind the table is <a href=\"netbench.csv\">netbench.csv</a>; the sweep's data is <a href=\"latency.csv\">latency.csv</a>.</p>\n")

	fmt.Fprintf(&b, "<footer>%s", html.EscapeString(info.describe()))

	if bad := totalMismatches(rows); bad > 0 {
		fmt.Fprintf(&b, "<br><strong>%d replies did not match their request; the figures above are worth nothing until that is understood.</strong>", bad)
	} else {
		b.WriteString("<br>Every reply matched its request byte for byte.")
	}

	b.WriteString("</footer>\n</body>\n</html>\n")

	if _, err := io.WriteString(f, b.String()); err != nil {
		return err
	}
	return nil
}

// widestRatio returns how many times a CCWIG round trip cost what a Go round
// trip cost, at the size where the gap was widest, which is the comparison
// worth putting in a sentence. A ratio below 1 means CCWIG was not the slower
// of the two.
func widestRatio(rows []row) (ratio float64, size int, ok bool) {
	for _, cfg := range configsOf(rows) {
		var ccwig, goRow *row

		for i := range rows {
			if rows[i].size != cfg {
				continue
			}
			switch rows[i].impl {
			case "ccwig":
				ccwig = &rows[i]
			case "go":
				goRow = &rows[i]
			}
		}

		if ccwig == nil || goRow == nil || goRow.microsPerRoundTrip() == 0 {
			continue
		}

		r := ccwig.microsPerRoundTrip() / goRow.microsPerRoundTrip()
		if !ok || r > ratio {
			ratio, size, ok = r, cfg, true
		}
	}

	return ratio, size, ok
}

// configsOf lists the message sizes present in the rows, in the order they
// were measured.
func configsOf(rows []row) []int {
	var sizes []int
	seen := make(map[int]bool)

	for _, r := range rows {
		if !seen[r.size] {
			seen[r.size] = true
			sizes = append(sizes, r.size)
		}
	}

	return sizes
}

func totalMismatches(rows []row) int {
	total := 0
	for _, r := range rows {
		total += r.mismatches
	}
	return total
}

// commas writes a number with thousands separators, because a report full of
// six-digit numbers is a report nobody can read at a glance.
func commas(n int) string {
	digits := strconv.Itoa(n)
	if len(digits) <= 3 {
		return digits
	}

	var b strings.Builder

	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}

	return b.String()
}
