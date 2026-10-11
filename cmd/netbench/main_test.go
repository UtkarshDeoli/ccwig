package main

import (
	"encoding/csv"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func dial(address string) (net.Conn, error) {
	return net.DialTimeout("tcp", address, 5*time.Second)
}

func writeAll(conn net.Conn, message string) error {
	_, err := conn.Write([]byte(message))
	return err
}

// readAll reads exactly n bytes, because a socket hands over bytes and not
// messages and one read is allowed to return less than was asked for.
func readAll(conn net.Conn, n int) (string, error) {
	buffer := make([]byte, n)

	read := 0
	for read < n {
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		got, err := conn.Read(buffer[read:])
		if err != nil {
			return "", fmt.Errorf("after %d of %d bytes: %w", read, n, err)
		}
		read += got
	}

	return string(buffer), nil
}

// repositoryRoot is two directories up from cmd/netbench. The harness runs
// ccwig as a program with a path to the client script, so a test has to give
// it paths that do not depend on where the test was started.
func repositoryRoot(t *testing.T) string {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// buildCCWIG builds the interpreter the benchmark needs. The harness runs
// ccwig as a program, so the test needs a real binary rather than a function
// it can call.
func buildCCWIG(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "ccwig")

	cmd := exec.Command("go", "build", "-o", binary, "./cmd/ccwig")
	cmd.Dir = repositoryRoot(t)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building ccwig: %v\n%s", err, out)
	}

	return binary
}

// buildSelf builds the harness itself, because the Go baseline is this program
// run as a separate process with -client. Inside a test os.Executable is the
// test binary, which has none of these flags, so the test has to say which
// binary to run.
func buildSelf(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "netbench")

	cmd := exec.Command("go", "build", "-o", binary, "./cmd/netbench")
	cmd.Dir = repositoryRoot(t)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building netbench: %v\n%s", err, out)
	}

	return binary
}

// TestBenchmarkWritesReportsOfAnHonestRun runs the whole harness the way the
// command runs it and then looks at what it wrote. A benchmark whose reports
// have never been read back is a benchmark that might be reporting nonsense.
func TestBenchmarkWritesReportsOfAnHonestRun(t *testing.T) {
	if testing.Short() {
		t.Skip("the benchmark starts real processes and sockets")
	}

	ccwig := buildCCWIG(t)
	out := t.TempDir()

	err := benchmark(options{
		out:     out,
		ccwig:   ccwig,
		script:  filepath.Join(repositoryRoot(t), "examples", "net", "netbench.cw"),
		self:    buildSelf(t),
		reps:    1,
		quick:   true,
		commit:  "test",
		note:    "a test",
		timeout: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("benchmark: %v", err)
	}

	for _, name := range []string{"netbench.csv", "netbench.md", "index.html"} {
		path := filepath.Join(out, name)

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s was not written: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}

	rows := readCSV(t, filepath.Join(out, "netbench.csv"))

	if len(rows) != 2*len(quickConfigs) {
		t.Fatalf("got %d rows, want one per client per configuration (%d)", len(rows), 2*len(quickConfigs))
	}

	for _, r := range rows {
		if r.mismatches != 0 {
			t.Errorf("%s at %d bytes: %d replies did not match their request", r.impl, r.size, r.mismatches)
		}
		if r.wallMS <= 0 {
			t.Errorf("%s at %d bytes: wall time was %.1f ms", r.impl, r.size, r.wallMS)
		}
		if r.roundTripsPerSecond() <= 0 || r.megabytesPerSecond() <= 0 {
			t.Errorf("%s at %d bytes: the report's derived figures are not positive", r.impl, r.size)
		}
	}

	// The two clients must have been asked for the same work at each size, or
	// the report would be comparing two different things.
	for _, cfg := range quickConfigs {
		for _, impl := range []string{"ccwig", "go"} {
			found := false

			for _, r := range rows {
				if r.impl == impl && r.size == cfg.size {
					found = true
					if r.count != cfg.count {
						t.Errorf("%s at %d bytes made %d round trips, not %d", impl, cfg.size, r.count, cfg.count)
					}
				}
			}

			if !found {
				t.Errorf("no %s row at %d bytes", impl, cfg.size)
			}
		}
	}

	// The Markdown and HTML are what a reader sees, so what they say has to be
	// in them: the sizes measured, and the honest note that this was a short
	// run rather than a benchmark.
	markdown := readFile(t, filepath.Join(out, "netbench.md"))

	for _, want := range []string{"ccwig", "| go |", "-quick", "Mismatches"} {
		if !strings.Contains(markdown, want) {
			t.Errorf("the Markdown report does not mention %q", want)
		}
	}

	page := readFile(t, filepath.Join(out, "index.html"))

	for _, want := range []string{"<!doctype html>", "CCWIG network benchmark", "</html>"} {
		if !strings.Contains(page, want) {
			t.Errorf("the HTML report does not contain %q", want)
		}
	}
}

// TestRefusesAQuickRunWithoutAnInterpreter checks the one thing that would
// otherwise make the report silently wrong: measuring something that is not
// there.
func TestRefusesARunWithoutAnInterpreter(t *testing.T) {
	err := benchmark(options{
		out:     t.TempDir(),
		ccwig:   filepath.Join(t.TempDir(), "there-is-no-ccwig-here"),
		reps:    1,
		quick:   true,
		timeout: time.Second,
	})
	if err == nil {
		t.Fatal("the benchmark ran with no interpreter to run")
	}
	if !strings.Contains(err.Error(), "no-ccwig") {
		t.Errorf("the error does not name the missing binary: %v", err)
	}
}

// TestEchoServerEchoes is the one thing every figure in the report rests on:
// that the server sends back exactly what it was given.
func TestEchoServerEchoes(t *testing.T) {
	address, stop, err := startEchoServer()
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	conn, err := dial(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	for _, message := range []string{"hello", "", "a longer message, with punctuation: !?.,", strings.Repeat("x", 10000)} {
		if err := writeAll(conn, message); err != nil {
			t.Fatalf("sending %d bytes: %v", len(message), err)
		}

		reply, err := readAll(conn, len(message))
		if err != nil {
			t.Fatalf("reading %d bytes: %v", len(message), err)
		}

		if reply != message {
			t.Errorf("sent %q, got back %q", message, reply)
		}
	}
}

func readCSV(t *testing.T, path string) []row {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	records, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	if len(records) == 0 {
		t.Fatal("the CSV has no header")
	}

	var rows []row

	for line, record := range records[1:] {
		if len(record) != 10 {
			t.Fatalf("line %d has %d fields, want 10", line+2, len(record))
		}

		rows = append(rows, row{
			impl:       record[0],
			size:       atoi(t, record[1]),
			count:      atoi(t, record[2]),
			reps:       atoi(t, record[3]),
			wallMS:     atof(t, record[4]),
			loopMS:     int64(atoi(t, record[5])),
			mismatches: atoi(t, record[9]),
		})
	}

	return rows
}

func atoi(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%q is not a number: %v", s, err)
	}
	return n
}

func atof(t *testing.T, s string) float64 {
	t.Helper()

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("%q is not a number: %v", s, err)
	}
	return f
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
