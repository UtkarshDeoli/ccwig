package main

import (
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestReadmeQuotesTheReports holds the readme to its own numbers.
//
// The readme is the front door: it is what someone reads before they read
// anything else, and the two tables in it are the whole of what most readers
// will ever know about this benchmark. They are also hand-pasted from a run,
// which is the one thing in this repository that is. Everything else is
// computed from the measurement by the program that made it, and a figure or a
// table that drifts from the data is a claim nobody checked.
//
// So the tables are checked here, against the reports they were copied out of.
// It lives beside the reports rather than beside the readme because the reports
// are what it needs to read, and this is where the readers for them are.
//
// The prose is not checked, and cannot be: "about a fifth of the wait" is a
// judgement, and a test for it would be a test for a phrase. What is checked is
// every number, which is where a stale paste is actually visible.
func TestReadmeQuotesTheReports(t *testing.T) {
	root := repositoryRoot(t)

	mainReport := filepath.Join(root, "reports", "netbench.csv")
	sweepReport := filepath.Join(root, "reports", "latency.csv")

	if _, err := os.Stat(mainReport); err != nil {
		t.Skipf("there are no published reports to check the readme against: %v", err)
	}

	rows := readCSV(t, mainReport)
	sweep := readLatencyCSV(t, sweepReport)
	readme := readFile(t, filepath.Join(root, "readme.md"))

	checked := 0

	// The main table: one line per size, both clients on it.
	table := readmeTable(t, readme, "| message size |")

	if len(table) != len(configsOf(rows)) {
		t.Fatalf("the readme's round-trip table has %d rows, and the report measured %d sizes",
			len(table), len(configsOf(rows)))
	}

	for _, cells := range table {
		if len(cells) != 5 {
			t.Fatalf("a row of the readme's round-trip table has %d columns, want 5: %v", len(cells), cells)
		}

		size := number(t, cells[0])

		for i, impl := range []string{"ccwig", "go"} {
			r, ok := findRow(rows, impl, size)
			if !ok {
				t.Fatalf("the readme quotes %d-byte messages, which the report does not have", size)
			}

			// The columns of the readme table after the size are the two
			// clients' µs per round trip and then their MB/s.
			want(t, cells[1+i], "netbench.csv", r.microsPerRoundTrip())
			want(t, cells[3+i], "netbench.csv", r.megabytesPerSecond())
			checked += 2
		}
	}

	// The sweep table: one line per delay.
	sweepTable := readmeTable(t, readme, "| server delay |")

	if len(sweepTable) != len(sweep) {
		t.Fatalf("the readme's delay table has %d rows, and the sweep measured %d delays",
			len(sweepTable), len(sweep))
	}

	for i, cells := range sweepTable {
		if len(cells) != 5 {
			t.Fatalf("a row of the readme's delay table has %d columns, want 5: %v", len(cells), cells)
		}

		r := sweep[i]

		if got := number(t, cells[0]); got != r.delayMS {
			t.Errorf("row %d of the readme's delay table is at %d ms, and the sweep's is at %d ms", i+1, got, r.delayMS)
		}

		want(t, cells[1], "latency.csv", r.ccwigMicros)
		want(t, cells[2], "latency.csv", r.goMicros)
		want(t, cells[3], "latency.csv", r.publishedGap)
		want(t, cells[4], "latency.csv", r.publishedShare)
		checked += 4
	}

	if checked < 20 {
		t.Errorf("only %d numbers were checked, which is fewer than these two tables hold", checked)
	}
}

// readmeTable reads the first Markdown table in the readme whose header line
// contains the given text, and returns its rows split into cells.
func readmeTable(t *testing.T, readme, header string) [][]string {
	t.Helper()

	lines := strings.Split(readme, "\n")

	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), header) {
			continue
		}

		var table [][]string

		// The line after the header is the alignment row, and everything after
		// that is data until a line that is not part of the table.
		for _, row := range lines[i+2:] {
			row = strings.TrimSpace(row)

			if !strings.HasPrefix(row, "|") {
				break
			}

			var cells []string
			for _, cell := range strings.Split(strings.Trim(row, "|"), "|") {
				cells = append(cells, strings.TrimSpace(cell))
			}

			table = append(table, cells)
		}

		return table
	}

	t.Fatalf("the readme has no table headed %q", header)
	return nil
}

// number reads a number out of a readme cell. The cells carry units - "64 B",
// "1,024 B", "19.2%" - and the number is the part being checked.
func number(t *testing.T, cell string) int {
	t.Helper()

	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.SplitN(cell, ".", 2)[0])

	if digits == "" {
		t.Fatalf("%q has no number in it", cell)
	}

	n, err := strconv.Atoi(digits)
	if err != nil {
		t.Fatalf("%q is not a number: %v", cell, err)
	}
	return n
}

// want is a number in the readme against the same number in a report, at the
// precision the readme publishes it to.
//
// The readme is a table for a reader and the report is data, so the readme
// rounds - one decimal, or a whole number for the differences. The tolerance
// is therefore half of the last digit the readme wrote: what is being ruled out
// is a number pasted from an older run, not a rounding.
func want(t *testing.T, cell, where string, got float64) {
	t.Helper()

	published, decimals := cellNumber(t, cell)

	tolerance := 0.5 / math.Pow(10, float64(decimals))
	tolerance += tolerance / 1000 // for a report value sitting on a rounding boundary

	if diff := published - got; diff > tolerance || diff < -tolerance {
		t.Errorf("the readme says %s and %s says %.2f - the readme quotes a run that is not the one in reports/", cell, where, got)
	}
}

// cellNumber reads the number out of a readme cell and how many decimals it
// was written to. The cells carry thousands separators and units - "1,024 B",
// "19.2%", "1,425.7" - none of which are part of the number.
func cellNumber(t *testing.T, cell string) (value float64, decimals int) {
	t.Helper()

	end := 0
	for end < len(cell) && (isDigit(cell[end]) || cell[end] == '.' || cell[end] == '-' || cell[end] == ',') {
		end++
	}

	number := strings.ReplaceAll(cell[:end], ",", "")
	if number == "" {
		t.Fatalf("%q has no number in it", cell)
	}

	if point := strings.Index(number, "."); point >= 0 {
		decimals = len(number) - point - 1
	}

	value, err := strconv.ParseFloat(number, 64)
	if err != nil {
		t.Fatalf("%q is not a number: %v", cell, err)
	}

	return value, decimals
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }
