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

	for _, name := range []string{"netbench.csv", "netbench.md", "latency.csv", "latency.md", "index.html"} {
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

	for _, want := range []string{"<!doctype html>", "CCWIG network benchmark", "</html>", "latency.md"} {
		if !strings.Contains(page, want) {
			t.Errorf("the HTML report does not contain %q", want)
		}
	}

	checkSweepRows(t, out)
}

// checkSweepRows reads the latency sweep back and checks the claim the sweep
// exists to make: that the interpreter does not get slower because the network
// did.
//
// The bound on the difference is the delay itself, which is loose to the point
// of looking careless, and is the only bound this test has any business
// asserting. The failure being ruled out is the interpreter paying for the
// wait - a difference that would approach the delay rather than sit under it,
// and would show up here as a factor of hundreds. Anything tighter would be a
// bound on the machine's jitter: the report says in as many words that a row
// of a hundred round trips cannot resolve a few microseconds, and a test that
// demanded it could would fail on a busy runner while the report beside it was
// telling the truth. It did exactly that before this comment was written.
func checkSweepRows(t *testing.T, out string) {
	t.Helper()

	sweep := readLatencyCSV(t, filepath.Join(out, "latency.csv"))

	// One row per delay in the sweep, plus the zero-delay row carried over from
	// the main report.
	if len(sweep) != len(quickLatency)+1 {
		t.Fatalf("the sweep has %d rows, want the delays plus the zero-delay row (%d)",
			len(sweep), len(quickLatency)+1)
	}

	if sweep[0].delayMS != 0 {
		t.Errorf("the first row of the sweep is at %d ms of delay, want the zero-delay row", sweep[0].delayMS)
	}

	// The zero-delay row is the main report's row for the sweep's message size,
	// so it has to be the same measurement rather than a second one.
	mainRows := readCSV(t, filepath.Join(out, "netbench.csv"))

	zero, ok := zeroDelayRow(mainRows)
	if !ok {
		t.Fatal("the main report has no rows at the sweep's message size, which is what the sweep's first row is built from")
	}
	if zero.count != sweep[0].count {
		t.Errorf("the zero-delay row made %d round trips but the main report made %d",
			sweep[0].count, zero.count)
	}

	// The zero-delay row is the one row of the sweep computed from another
	// report's rows, so the two published files have to agree about it. A CSV
	// column coarse enough to lose those microseconds would publish a first row
	// that nothing in the repository could reproduce - which is how this check
	// came to exist.
	if diff := zero.ccwigMicros - sweep[0].ccwigMicros; diff > 0.01 || diff < -0.01 {
		t.Errorf("the main report gives the CCWIG client %.2f µs per round trip at %d bytes but the sweep's first row says %.2f µs",
			zero.ccwigMicros, latencySize, sweep[0].ccwigMicros)
	}
	if diff := zero.goMicros - sweep[0].goMicros; diff > 0.01 || diff < -0.01 {
		t.Errorf("the main report gives the Go client %.2f µs per round trip at %d bytes but the sweep's first row says %.2f µs",
			zero.goMicros, latencySize, sweep[0].goMicros)
	}
	for _, r := range sweep[1:] {
		if zero.count <= r.count {
			t.Errorf("the zero-delay row made %d round trips, which is no more than the %d of the row at %d ms of delay",
				zero.count, r.count, r.delayMS)
		}
	}

	for _, r := range sweep {
		if r.mismatches != 0 {
			t.Errorf("at %d ms of delay: %d replies did not match their request", r.delayMS, r.mismatches)
		}
		if r.count <= 0 || r.ccwigMicros <= 0 || r.goMicros <= 0 {
			t.Errorf("at %d ms of delay: the row is not a measurement (%+v)", r.delayMS, r)
			continue
		}
		if r.size != latencySize {
			t.Errorf("at %d ms of delay: the message was %d bytes, want %d", r.delayMS, r.size, latencySize)
		}

		// The round trip has to include the delay, or the delay was never
		// applied and everything below is about a server that ignores it.
		if r.delayMS > 0 {
			want := float64(r.delayMS) * 1000

			if r.ccwigMicros < want*0.9 {
				t.Errorf("at %d ms of delay the CCWIG round trip was %.1f µs, which is not the delay", r.delayMS, r.ccwigMicros)
			}

			// The difference has to stay underneath the wait rather than
			// approaching it, which is what would happen if the interpreter
			// were paying for the wait instead of the network.
			if gap := r.gapMicros(); gap > want {
				t.Errorf("at %d ms of delay the interpreter accounts for %.1f µs of the round trip, which is as much as the wait itself",
					r.delayMS, gap)
			}
		}
	}

	// The table has to be about something: a sweep where every delay measured
	// the same round trip is a sweep whose delay did nothing.
	if len(sweep) > 1 {
		first, last := sweep[0], sweep[len(sweep)-1]

		if last.ccwigMicros <= first.ccwigMicros {
			t.Errorf("the round trip at %d ms of delay (%.1f µs) is not longer than at %d ms (%.1f µs)",
				last.delayMS, last.ccwigMicros, first.delayMS, first.ccwigMicros)
		}
	}
}

// TestEchoServerWaitsBeforeItAnswers is the sweep's one dependency: if the
// server ignores the delay, every row of the sweep is the same row and the
// report says nothing while looking like it says something.
func TestEchoServerWaitsBeforeItAnswers(t *testing.T) {
	const delay = 60 * time.Millisecond

	address, stop, err := startEchoServer(delay)
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	conn, err := dial(address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// The delay is per reply, so three round trips of it are three delays.
	const trips = 3

	started := time.Now()

	for i := 0; i < trips; i++ {
		if err := writeAll(conn, "ping"); err != nil {
			t.Fatal(err)
		}

		reply, err := readAll(conn, len("ping"))
		if err != nil {
			t.Fatal(err)
		}
		if reply != "ping" {
			t.Fatalf("round trip %d got %q back, want %q", i, reply, "ping")
		}
	}

	elapsed := time.Since(started)

	if elapsed < trips*delay {
		t.Errorf("%d round trips took %s, which is less than the %s the server was told to wait",
			trips, elapsed, trips*delay)
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
	address, stop, err := startEchoServer(0)
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
			loopUS:     int64(atoi(t, record[5])),
			mismatches: atoi(t, record[9]),
		})
	}

	return rows
}

func readLatencyCSV(t *testing.T, path string) []latencyRow {
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
		t.Fatal("the latency CSV has no header")
	}

	var rows []latencyRow

	for line, record := range records[1:] {
		if len(record) != 9 {
			t.Fatalf("line %d has %d fields, want 9", line+2, len(record))
		}

		rows = append(rows, latencyRow{
			delayMS:     atoi(t, record[0]),
			size:        atoi(t, record[1]),
			count:       atoi(t, record[2]),
			reps:        atoi(t, record[3]),
			ccwigMicros: atof(t, record[4]),
			goMicros:    atof(t, record[5]),
			mismatches:  atoi(t, record[8]),
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
