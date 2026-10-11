// Command netbench measures how fast a CCWIG program can move bytes over a
// loopback TCP connection, against the same exchange written in Go.
//
// It starts an echo server, has each client send a payload and wait for it to
// come back, a fixed number of times, and writes what it measured to
// reports/netbench.csv, reports/netbench.md and reports/index.html.
//
// It then does it again against a server that waits before it answers, which
// puts the same exchange on a network that costs what a real one costs, and
// writes that to reports/latency.csv, reports/latency.md. The main report
// answers how much of a loopback round trip is the interpreter; the sweep
// answers whether that matters, and the sweep is the one with an interesting
// answer.
//
//	netbench                                   run it and write the reports
//	netbench -quick                            a short run, for a smoke check
//	netbench -client -peer H:P -size N -count M
//	                                           act as the Go client, which is
//	                                           how the harness runs the
//	                                           baseline it compares against
//
// The baseline runs as a separate process rather than in the harness, because
// every figure in the report is the wall-clock time of a whole client process:
// a client measured inside the harness would be missing the start-up cost that
// every other row includes, and would look faster for a reason that is not
// about the network.
//
// What this measures is not the kernel's TCP. It is a tree-walking interpreter
// calling two builtins per round trip, and the report says so, because a
// benchmark that does not say what it measured is a number without a meaning.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"time"
)

// config is one message size and how many round trips of it to make.
type config struct {
	size  int
	count int
}

// fullConfigs are the sizes the real report is built from. The counts are
// chosen so that each run lasts a few hundred milliseconds: the clock the
// CCWIG client reads counts milliseconds, and a benchmark whose own timing
// granularity is a tenth of its total is a benchmark measuring its clock.
var fullConfigs = []config{
	{64, 20000},
	{256, 20000},
	{1024, 15000},
	{4096, 10000},
	{16384, 5000},
}

// quickConfigs are small enough to run inside a test.
var quickConfigs = []config{
	{64, 1000},
	{1024, 500},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "netbench:", err)
		os.Exit(1)
	}
}

// options is everything the benchmark does, so that the command line and a
// test can ask for the same run.
type options struct {
	out     string
	ccwig   string
	script  string
	self    string
	reps    int
	quick   bool
	note    string
	commit  string
	timeout time.Duration
}

func run() error {
	var (
		out     = flag.String("out", "reports", "directory to write the reports to")
		ccwig   = flag.String("ccwig", filepath.Join("bin", "ccwig"), "path to the ccwig binary")
		script  = flag.String("script", filepath.Join("examples", "net", "netbench.cw"), "the client program to measure")
		self    = flag.String("self", "", "the binary to run as the Go client; empty means this one")
		reps    = flag.Int("reps", 3, "how many times to run each measurement")
		quick   = flag.Bool("quick", false, "use short runs, for a smoke check")
		note    = flag.String("note", "", "a line to record in the report, such as what the machine was")
		commit  = flag.String("commit", "unknown", "the revision the report was made from")
		timeout = flag.Duration("timeout", 120*time.Second, "how long one client run may take")
		client  = flag.Bool("client", false, "act as the Go client instead of running the benchmark")
		peer    = flag.String("peer", "", "host:port to connect to, with -client")
		size    = flag.Int("size", 0, "message size in bytes, with -client")
		count   = flag.Int("count", 0, "round trips to make, with -client")
	)
	flag.Parse()

	if *client {
		return runGoClient(*peer, *size, *count)
	}

	return benchmark(options{
		out:     *out,
		ccwig:   *ccwig,
		script:  *script,
		self:    *self,
		reps:    *reps,
		quick:   *quick,
		note:    *note,
		commit:  *commit,
		timeout: *timeout,
	})
}

func benchmark(opts options) error {
	if opts.reps < 1 {
		return errors.New("reps must be at least 1")
	}

	absolute, script, err := resolve(opts)
	if err != nil {
		return err
	}

	configs := fullConfigs
	if opts.quick {
		configs = quickConfigs
	}

	address, stop, err := startEchoServer(0)
	if err != nil {
		return err
	}
	defer stop()

	// The reports are about a machine as much as about a program, so the
	// machine goes in them. A throughput figure with no machine attached is a
	// number nobody can compare with anything.
	info := machineInfo{
		Go:     runtime.Version(),
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		CPUs:   runtime.NumCPU(),
		Date:   time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		Commit: opts.commit,
		Note:   opts.note,
		Reps:   opts.reps,
		Quick:  opts.quick,
	}

	var rows []row

	// The first run of a program on a machine is slower than the rest: the
	// binary is read from disk for the first time, the pages it touches are not
	// in memory yet, and the server has never accepted a connection. Running
	// each client once and throwing the result away costs a fraction of a second
	// and keeps that first-run cost out of the report, where it would look like
	// a fact about the language.
	for _, impl := range []string{"ccwig", "go"} {
		warmUp := config{size: 64, count: 200}
		if _, err := measure(opts.self, impl, absolute, script, address, warmUp, 1, opts.timeout); err != nil {
			return fmt.Errorf("the warm-up run of %s failed: %w", impl, err)
		}
	}

	for _, cfg := range configs {
		for _, impl := range []string{"ccwig", "go"} {
			measured, err := measure(opts.self, impl, absolute, script, address, cfg, opts.reps, opts.timeout)
			if err != nil {
				return fmt.Errorf("%s at %d bytes: %w", impl, cfg.size, err)
			}
			rows = append(rows, measured)
			fmt.Printf("%-6s %7d bytes %6d round trips  %8.1f ms  %7.1f us/round trip  %8.2f MB/s\n",
				measured.impl, measured.size, measured.count,
				measured.wallMS, measured.microsPerRoundTrip(), measured.megabytesPerSecond())
		}
	}

	if err := writeReports(opts.out, rows, info); err != nil {
		return err
	}

	// The second question, with a server that waits before it answers.
	latencyConfigs := fullLatency
	if opts.quick {
		latencyConfigs = quickLatency
	}

	fmt.Println()

	latency, err := latencySweep(opts, absolute, script, latencyConfigs)
	if err != nil {
		return err
	}

	// The zero-delay row is the main report's, re-read rather than re-measured.
	if zero, ok := zeroDelayRow(rows); ok {
		fmt.Printf("delay %-5s %6d round trips  ccwig %10.1f us  go %10.1f us  gap %8.1f us  (%.2f%%)\n",
			"0s", zero.count, zero.ccwigMicros, zero.goMicros, zero.gapMicros(), zero.gapShare())
		latency = append([]latencyRow{zero}, latency...)
	}

	if err := writeLatencyReports(opts.out, latency, info); err != nil {
		return err
	}

	fmt.Printf("\nwrote %s, %s, %s, %s and %s\n",
		filepath.Join(opts.out, "netbench.csv"),
		filepath.Join(opts.out, "netbench.md"),
		filepath.Join(opts.out, "latency.csv"),
		filepath.Join(opts.out, "latency.md"),
		filepath.Join(opts.out, "index.html"))

	return nil
}

// resolve turns the harness's options into absolute, checked paths.
//
// Both paths are resolved here rather than where they are used, because a run
// of a program that is not there fails inside the client: it looks like a
// failure of the thing being measured rather than of the measurement, which is
// the one way this harness could report a wrong number without saying so.
func resolve(opts options) (ccwigPath, script string, err error) {
	ccwigPath, err = filepath.Abs(opts.ccwig)
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(ccwigPath); err != nil {
		return "", "", fmt.Errorf("%s: %w (build it with `make build`, or pass -ccwig)", opts.ccwig, err)
	}

	script, err = filepath.Abs(opts.script)
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(script); err != nil {
		return "", "", fmt.Errorf("%s: %w (run the harness from the repository root, or pass -script)", opts.script, err)
	}

	return ccwigPath, script, nil
}

// measure runs one implementation against one configuration, reps times, and
// keeps the median.
//
// The median rather than the mean, because a loopback benchmark on a machine
// doing anything else has occasional slow runs and no occasional fast ones: the
// distribution has a long tail on the slow side, and an average would let one
// interrupted run decide the number.
func measure(self, impl, ccwigPath, script, address string, cfg config, reps int, timeout time.Duration) (row, error) {
	host, portText, err := net.SplitHostPort(address)
	if err != nil {
		return row{}, err
	}

	var walls, loopsUS []float64
	mismatches := 0

	for i := 0; i < reps; i++ {
		var (
			loopUS int64
			bad    int
			wall   time.Duration
		)

		if impl == "ccwig" {
			loopUS, bad, wall, err = runClient(ccwigPath, []string{
				script, host, portText,
				strconv.Itoa(cfg.size), strconv.Itoa(cfg.count),
			}, timeout)
		} else {
			loopUS, bad, wall, err = runSelfClient(self, address, cfg, timeout)
		}
		if err != nil {
			return row{}, err
		}

		walls = append(walls, float64(wall.Microseconds())/1000)
		loopsUS = append(loopsUS, float64(loopUS))
		mismatches += bad
	}

	return row{
		impl:       impl,
		size:       cfg.size,
		count:      cfg.count,
		reps:       reps,
		wallMS:     medianFloat(walls),
		loopUS:     int64(medianFloat(loopsUS)),
		mismatches: mismatches,
	}, nil
}

// runClient runs a client program and reads the line of numbers it prints.
//
// The program is expected to print its own timing as well as make the exchange,
// so that a wall-clock figure can be checked against what the client itself
// saw. Two numbers that disagree by a lot mean something outside the loop is
// taking the time, and the report would rather show that than hide it.
//
// The timeout is real: a client that hangs would otherwise hang the whole
// benchmark, and the point of a deadline is to turn a mistake into an error
// message rather than into a wait.
func runClient(path string, args []string, timeout time.Duration) (loopUS int64, mismatches int, wall time.Duration, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, args...)

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	started := time.Now()
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return 0, 0, 0, fmt.Errorf("%s %v: took longer than %s and was stopped", path, args, timeout)
		}
		return 0, 0, 0, fmt.Errorf("%s %v: %w: %s", path, args, err, firstLine(errOut.String()))
	}
	wall = time.Since(started)

	loopUS, mismatches, err = parseReport(out.String())
	return loopUS, mismatches, wall, err
}

// runSelfClient runs this binary as the Go client, which is the baseline the
// CCWIG numbers are compared against.
func runSelfClient(self, address string, cfg config, timeout time.Duration) (int64, int, time.Duration, error) {
	if self == "" {
		path, err := os.Executable()
		if err != nil {
			return 0, 0, 0, err
		}
		self = path
	}

	return runClient(self, []string{
		"-client", "-peer", address,
		"-size", strconv.Itoa(cfg.size), "-count", strconv.Itoa(cfg.count),
		"-timeout", timeout.String(),
	}, timeout)
}

// clientLine is what both clients print, so that the harness reads one format
// whatever it ran.
//
// The client prints its own timing twice, in milliseconds and in microseconds.
// The harness reads the microseconds, because that is the clock the latency
// sweep needs; the milliseconds are in the line for whoever runs a client by
// hand, and the pattern requires them so that a client which stopped printing
// either clock is a mismatch rather than a quiet zero.
var clientLine = regexp.MustCompile(`netbench size=(\d+) count=(\d+) ms=(\d+) us=(\d+) mismatches=(\d+)`)

func parseReport(out string) (loopUS int64, mismatches int, err error) {
	m := clientLine.FindStringSubmatch(out)
	if m == nil {
		return 0, 0, fmt.Errorf("the client printed nothing the harness could read: %q", firstLine(out))
	}

	loopUS, err = strconv.ParseInt(m[4], 10, 64)
	if err != nil {
		return 0, 0, err
	}

	bad, err := strconv.Atoi(m[5])
	if err != nil {
		return 0, 0, err
	}

	return loopUS, bad, nil
}

// startEchoServer brings up a loopback echo server and returns its address and
// a function that shuts it down. A delay above zero makes it wait that long
// before answering each read, which is how the latency sweep puts a real
// round trip on a socket that is not a real network.
//
// The server is deliberately not the thing under test: it does the same work
// for every client, copies bytes back as they arrive, and is the same machine
// for every row of the report.
func startEchoServer(delay time.Duration) (string, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go func() {
				defer conn.Close()

				// Everything that arrives goes straight back. io.Copy does the
				// reading and the writing, so the echo cannot be biased by
				// anything this program thinks it knows about message sizes.
				//
				// With a delay there is something to bias, and io.Copy cannot
				// express it, so the echo is written out by hand instead: read
				// a chunk, wait, write it back.
				if delay <= 0 {
					_, _ = io.Copy(conn, conn)
					return
				}

				echoSlowly(conn, delay)
			}()
		}
	}()

	stop := func() {
		_ = listener.Close()
		<-done
	}
	return listener.Addr().String(), stop, nil
}

// echoSlowly copies bytes back, waiting delay before each reply.
//
// The wait is on the reply and not on the read, so what a client measures is
// the delay plus the loopback underneath it - which is the number the sweep is
// about. A real link would charge a sender's delay too, but that delay is not
// the interpreter's either, and adding it would only make every number in the
// table larger by the same amount.
func echoSlowly(conn net.Conn, delay time.Duration) {
	buffer := make([]byte, 64<<10)

	for {
		read, err := conn.Read(buffer)

		if read > 0 {
			time.Sleep(delay)

			if _, err := conn.Write(buffer[:read]); err != nil {
				return
			}
		}

		if err != nil {
			return
		}
	}
}

// firstLine is the first line of some output, for an error message that stays
// on one line.
func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

func medianFloat(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	return sorted[len(sorted)/2]
}
