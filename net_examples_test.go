package ccwig_test

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UtkarshDeoli/ccwig/internal/interp"
)

// The programs in examples/net are the ones that need a peer, so the
// golden-file harness that runs everything else cannot run them. Each test
// here supplies the peer itself and reads what the program did with it. What
// is being checked is not whether an echo echoes - it is that a program
// written in this language can hold up its end of a real conversation, with a
// peer that knows nothing about this language.

// netExample resolves a program in examples/net, failing if it is not there.
func netExample(t *testing.T, name string) string {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("examples", "net", name))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return path
}

// program is a program running in the interpreter with its standard output
// split into lines as they are written. A server prints where it is listening
// and then waits, so a test cannot run it to completion and look at the output
// afterwards; it has to read along as it goes.
type program struct {
	lines chan string
	done  chan error
}

// startProgram starts a program and returns it while it is still running.
func startProgram(t *testing.T, path string, args ...string) *program {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	// A pipe rather than a buffer: the program blocks on its own writes until
	// the test reads them, which is exactly the back-pressure a real pipe
	// gives, and it means no line is ever lost to a buffer being drained late.
	reader, writer := io.Pipe()

	in := interp.New(writer)
	in.Args = args

	p := &program{
		lines: make(chan string, 1024),
		done:  make(chan error, 1),
	}

	go func() {
		defer close(p.lines)

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			p.lines <- scanner.Text()
		}
	}()

	go func() {
		err := in.Run(string(src), path)
		writer.Close()
		p.done <- err
	}()

	return p
}

// waitFor reads lines until one matches, and fails rather than hangs if the
// program ends or goes quiet first. A test that hangs on a broken server is
// worse than one that fails, because the first one is still there an hour
// later.
func (p *program) waitFor(t *testing.T, pattern string) string {
	t.Helper()

	want := regexp.MustCompile(pattern)
	deadline := time.After(10 * time.Second)

	for {
		select {
		case line, ok := <-p.lines:
			if !ok {
				// The program ended. If it ended badly, that is the useful
				// thing to report, not the pattern that never came. The error
				// arrives just after the last line does, so this waits a moment
				// for it rather than racing it.
				select {
				case err := <-p.done:
					t.Fatalf("the program failed while waiting for %q: %v", pattern, err)
				case <-time.After(2 * time.Second):
					t.Fatalf("the program ended without printing anything matching %q", pattern)
				}
			}
			if want.MatchString(line) {
				return line
			}
		case <-deadline:
			t.Fatalf("waited ten seconds for a line matching %q", pattern)
		}
	}
}

// waitForEnd waits for the program to finish and returns how it ended.
func (p *program) waitForEnd(t *testing.T) error {
	t.Helper()

	select {
	case err := <-p.done:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the program did not finish within ten seconds")
		return nil
	}
}

// TestEchoServerAnswersEveryLine drives the echo server with a Go client and
// checks the reply to every line, and then the summary the server prints from
// its own count of what it moved.
func TestEchoServerAnswersEveryLine(t *testing.T) {
	p := startProgram(t, netExample(t, "echo-server.cw"), "0")

	address := strings.TrimPrefix(p.waitFor(t, `^listening on `), "listening on ")

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatalf("connecting to the echo server at %s: %v", address, err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)

	lines := []string{"hello", "a second line", "x", strings.Repeat("long ", 40)}
	bytesSent := 0

	for i, message := range lines {
		if _, err := fmt.Fprintf(conn, "%s\n", message); err != nil {
			t.Fatalf("sending line %d: %v", i+1, err)
		}
		bytesSent += len(message) + 1

		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		reply, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("reading the reply to line %d: %v", i+1, err)
		}

		// The shape is the server's own: a number, a space, then what it heard.
		if want := fmt.Sprintf("%d %s\n", i+1, message); reply != want {
			t.Errorf("line %d: got %q, want %q", i+1, reply, want)
		}
	}

	conn.Close()

	// The summary is the server's own report of its own work, and it has to
	// agree with what this test sent - the same reason the other examples are
	// checked against something outside the language.
	summary := p.waitFor(t, `^served `)

	if err := p.waitForEnd(t); err != nil {
		t.Fatalf("the echo server failed: %v", err)
	}

	if want := fmt.Sprintf("served %d lines, %d bytes", len(lines), bytesSent); summary != want {
		t.Errorf("the server reported %q, the test sent %d bytes in %d lines", summary, bytesSent, len(lines))
	}
}

// TestEchoServerStopsAtAQuote checks the polite way out, because a server that
// cannot be asked to stop is a server a script has to kill.
func TestEchoServerStopsAtAQuote(t *testing.T) {
	p := startProgram(t, netExample(t, "echo-server.cw"), "0")

	address := strings.TrimPrefix(p.waitFor(t, `^listening on `), "listening on ")

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if _, err := io.WriteString(conn, "quit\n"); err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("reading the answer to quit: %v", err)
	}
	if !strings.Contains(reply, "goodbye") {
		t.Errorf("quit was answered with %q, which does not say goodbye", reply)
	}

	if err := p.waitForEnd(t); err != nil {
		t.Fatalf("the echo server failed: %v", err)
	}
}

// TestHTTPGetReadsWhatGoServes points the HTTP client at a Go HTTP server.
// That is the strongest check available here: the bytes it parses were written
// by an implementation of the protocol that was not written to agree with it.
func TestHTTPGetReadsWhatGoServes(t *testing.T) {
	body := "region,units,revenue\nwest,44,2010\nsouth,42,1753\n"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/sales.csv" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/csv")
		fmt.Fprint(w, body)
	}))
	defer server.Close()

	host, port, err := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}

	out, status := runWith(t, netExample(t, "http-get.cw"), []string{host, port, "/sales.csv"}, nil)

	if status != 0 {
		t.Fatalf("http-get exited %d:\n%s", status, out)
	}

	for _, want := range []string{
		// HTTP/1.0, not 1.1: the client asks in 1.0, and a server answers in
		// the version it was asked in.
		"status:   HTTP/1.0 200 OK",
		"header:   Content-Length: " + strconv.Itoa(len(body)),
		"body:     " + strconv.Itoa(len(body)) + " bytes",
		"region,units,revenue",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the client did not report %q:\n%s", want, out)
		}
	}
}

// TestHTTPGetSaysWhatIsWrong covers the two ways it can go wrong: nothing to
// connect to, and a server that answers with something other than what was
// asked for.
func TestHTTPGetSaysWhatIsWrong(t *testing.T) {
	t.Run("nothing is listening", func(t *testing.T) {
		// A port that was just released, so something was bound to it a moment
		// ago and nothing is bound to it now.
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		listener.Close()

		host, port, _ := net.SplitHostPort(address)

		out, status := runWith(t, netExample(t, "http-get.cw"), []string{host, port, "/"}, nil)

		if status == 0 {
			t.Errorf("a client that could not connect exited 0:\n%s", out)
		}
		if !strings.Contains(out, "cannot connect") {
			t.Errorf("the failure was not explained:\n%s", out)
		}
	})

	t.Run("the path is missing", func(t *testing.T) {
		server := httptest.NewServer(http.NotFoundHandler())
		defer server.Close()

		host, port, _ := net.SplitHostPort(strings.TrimPrefix(server.URL, "http://"))

		out, status := runWith(t, netExample(t, "http-get.cw"), []string{host, port, "/nothing"}, nil)

		// The protocol answered. That is a success at the protocol level and a
		// report of a 404, not a broken client.
		if status != 0 {
			t.Errorf("http-get exited %d on a 404:\n%s", status, out)
		}
		if !strings.Contains(out, "404") {
			t.Errorf("the 404 was not reported:\n%s", out)
		}
	})

	t.Run("asked with no arguments", func(t *testing.T) {
		out, status := runWith(t, netExample(t, "http-get.cw"), nil, nil)

		if status != 2 {
			t.Errorf("http-get with no arguments exited %d, want 2:\n%s", status, out)
		}
		if !strings.Contains(out, "usage") {
			t.Errorf("no usage was printed:\n%s", out)
		}
	})
}

// TestChatServerTalksToTwoClients is the check that matters for a server with
// more than one client: what one sends, everybody hears, through one thread
// that never blocks on any single connection.
func TestChatServerTalksToTwoClients(t *testing.T) {
	// Twenty seconds of runway, so the conversation ends the server and the
	// clock never does. The limit is there only so a mistake cannot hang it.
	p := startProgram(t, netExample(t, "chat-server.cw"), "0", "20000")

	address := strings.TrimPrefix(p.waitFor(t, `^chat server listening on `), "chat server listening on ")

	dial := func() (net.Conn, *bufio.Reader) {
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			t.Fatalf("connecting to the chat server at %s: %v", address, err)
		}
		// Every connection this test opens carries a deadline, so that a
		// server that stops answering fails the test instead of hanging it.
		conn.SetDeadline(time.Now().Add(10 * time.Second))
		return conn, bufio.NewReader(conn)
	}

	// Alice arrives first and waits, because a chat of one is just an echo.
	alice, aliceReader := dial()
	defer alice.Close()

	p.waitFor(t, `^guest1 joined`)

	bob, bobReader := dial()
	defer bob.Close()

	p.waitFor(t, `^guest2 joined`)

	// What each client is sent, in the order it is sent. Every line is read and
	// checked, because a client that is behind by one line would make every
	// later check pass for the wrong reason.
	//
	// Alice, the first in: her greeting, the roster she is already on, and her
	// own arrival announced to the room - which at that point is her.
	if got := readLineWithin(t, aliceReader); got != "welcome guest1, 1 here" {
		t.Fatalf("the first client was greeted with %q", got)
	}
	if got := readLineWithin(t, aliceReader); got != "here: guest1" {
		t.Errorf("the first client was told the room holds %q", got)
	}
	if got := readLineWithin(t, aliceReader); got != "guest1 joined" {
		t.Errorf("the first client was told %q about its own arrival", got)
	}

	// Bob, the second in: greeted by name, told who is already here, and told
	// about his own arrival.
	if got := readLineWithin(t, bobReader); got != "welcome guest2, 2 here" {
		t.Fatalf("the second client was greeted with %q", got)
	}
	if got := readLineWithin(t, bobReader); got != "here: guest1,guest2" {
		t.Errorf("the second client was told the room holds %q, not who was in it", got)
	}
	if got := readLineWithin(t, bobReader); got != "guest2 joined" {
		t.Errorf("the second client was told %q about its own arrival", got)
	}

	// Alice is told about the arrival too. This is the line that proves the
	// poll loop reached her while it was also accepting someone else.
	if got := readLineWithin(t, aliceReader); got != "guest2 joined" {
		t.Errorf("the first client was told %q about the new arrival", got)
	}

	if _, err := io.WriteString(alice, "hello bob\n"); err != nil {
		t.Fatal(err)
	}

	// Everyone in the room hears everything, the speaker included.
	if got := readLineWithin(t, aliceReader); got != "guest1: hello bob" {
		t.Errorf("the speaker heard %q back", got)
	}
	if got := readLineWithin(t, bobReader); got != "guest1: hello bob" {
		t.Errorf("the other client heard %q", got)
	}

	// A client that goes away is noticed without anyone announcing it. This is
	// the poll loop telling an end of stream from a deadline that passed.
	alice.Close()

	if got := readLineWithin(t, bobReader); got != "guest1 left" {
		t.Errorf("the departure was announced as %q", got)
	}

	if _, err := io.WriteString(bob, "quit\n"); err != nil {
		t.Fatal(err)
	}

	if got := readLineWithin(t, bobReader); got != "the server is closing" {
		t.Errorf("quitting was answered with %q", got)
	}

	summary := p.waitFor(t, `^chat server done`)

	if err := p.waitForEnd(t); err != nil {
		t.Fatalf("the chat server failed: %v", err)
	}

	if summary != "chat server done: 2 client(s), 1 message(s)" {
		t.Errorf("the server summarised its run as %q", summary)
	}
}

// TestChatServerTurnsAwayTheNinthClient checks that being full is a thing the
// server says, rather than a thing it does by accepting and then dropping.
func TestChatServerTurnsAwayTheNinthClient(t *testing.T) {
	p := startProgram(t, netExample(t, "chat-server.cw"), "0", "20000")

	address := strings.TrimPrefix(p.waitFor(t, `^chat server listening on `), "chat server listening on ")

	var conns []net.Conn
	defer func() {
		for _, conn := range conns {
			conn.Close()
		}
	}()

	// The room holds eight, and the example says so; the test does not assume
	// the number, it takes the first message the ninth client gets as the
	// answer.
	for i := 0; i < 8; i++ {
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			t.Fatalf("connecting client %d: %v", i+1, err)
		}
		conn.SetDeadline(time.Now().Add(10 * time.Second))
		conns = append(conns, conn)

		// Each one is read so the server's greeting does not sit unread, and
		// so this test knows the server got as far as counting it.
		if got := readLineWithin(t, bufio.NewReader(conn)); !strings.Contains(got, "welcome") {
			t.Fatalf("client %d was greeted with %q", i+1, got)
		}
	}

	// The ninth client connects before the server is asked about it: a
	// connection completes as soon as the machine accepts it, whether or not
	// the program has got round to accepting it, so this dial returns even
	// while the server is busy with the other eight.
	ninth, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	ninth.SetDeadline(time.Now().Add(10 * time.Second))
	conns = append(conns, ninth)

	got := readLineWithin(t, bufio.NewReader(ninth))

	if !strings.Contains(got, "full") {
		t.Errorf("the ninth client was told %q, which is not that the room is full", got)
	}
}

// TestNetbenchMovesWhatItSaysItMoved runs the benchmark client against a Go
// echo server for a short burst, and checks the two things that would make
// every figure in reports/ a lie: bytes that did not come back, and a count
// the client did not actually make.
func TestNetbenchMovesWhatItSaysItMoved(t *testing.T) {
	address, stop := startGoEcho(t)
	defer stop()

	host, port, _ := net.SplitHostPort(address)

	const size, count = 512, 200

	out, status := runWith(t, netExample(t, "netbench.cw"), []string{host, port, strconv.Itoa(size), strconv.Itoa(count)}, nil)

	if status != 0 {
		t.Fatalf("netbench exited %d:\n%s", status, out)
	}

	m := regexp.MustCompile(`netbench size=(\d+) count=(\d+) ms=(\d+) mismatches=(\d+)`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("netbench printed no line the harness could read:\n%s", out)
	}

	if got := m[1]; got != strconv.Itoa(size) {
		t.Errorf("the client reported size %s, it was told %d", got, size)
	}
	if got := m[2]; got != strconv.Itoa(count) {
		t.Errorf("the client reported count %s, it was told %d", got, count)
	}
	if got := m[4]; got != "0" {
		t.Errorf("the client reported %s mismatches: it is not moving the bytes it thinks it is", got)
	}
}

// TestNetbenchRefusesNonsense checks that a wrong command line is a usage
// message, rather than a run against a port that is not there.
func TestNetbenchRefusesNonsense(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"127.0.0.1", "not-a-port", "64", "10"},
		{"127.0.0.1", "9000", "0", "10"},
		{"127.0.0.1", "9000", "64", "0"},
	} {
		out, status := runWith(t, netExample(t, "netbench.cw"), args, nil)

		if status != 2 {
			t.Errorf("netbench %v exited %d, want 2:\n%s", args, status, out)
		}
		if !strings.Contains(out, "usage") {
			t.Errorf("netbench %v printed no usage:\n%s", args, out)
		}
	}
}

// readLineWithin reads one line from a connection, failing rather than hanging
// when the other end goes quiet.
func readLineWithin(t *testing.T, reader *bufio.Reader) string {
	t.Helper()

	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading a line: %v", err)
	}

	return strings.TrimRight(line, "\r\n")
}

// startGoEcho is the peer the examples are measured against: a Go echo server,
// deliberately the simplest one that works, because everything that arrives
// has to go back unchanged.
func startGoEcho(t *testing.T) (string, func()) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
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
				_, _ = io.Copy(conn, conn)
			}()
		}
	}()

	stop := func() {
		listener.Close()
		<-done
	}

	return listener.Addr().String(), stop
}

// TestNetExampleProgramsExist keeps this list and the directory in step: a
// program added to examples/net with no test here would otherwise go
// unchecked, which for a program that talks to the network is the worst place
// to leave a gap.
func TestNetExampleProgramsExist(t *testing.T) {
	found, err := filepath.Glob(filepath.Join("examples", "net", "*.cw"))
	if err != nil {
		t.Fatal(err)
	}

	tested := map[string]bool{
		"echo-server.cw": true,
		"http-get.cw":    true,
		"netbench.cw":    true,
		"chat-server.cw": true,
	}

	for _, path := range found {
		name := filepath.Base(path)
		if !tested[name] {
			t.Errorf("%s is not covered by a test in net_examples_test.go", name)
		}
		delete(tested, name)
	}

	for name := range tested {
		t.Errorf("net_examples_test.go tests %s, which is not in examples/net", name)
	}
}
