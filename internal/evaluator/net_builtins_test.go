package evaluator

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/parser"
)

// The networking builtins are tested against real sockets, not a stand-in: a
// CCWIG program talks to a Go server, and a Go client talks to a CCWIG server.
// Anything less would test the harness rather than the language.

func TestNetAsAClient(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	defer listener.Close()

	// A one-shot Go server: read a line, answer it, hang up.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		line, _ := bufio.NewReader(conn).ReadString('\n')
		_, _ = io.WriteString(conn, "pong "+strings.TrimSpace(line)+"\n")
	}()

	_, port := hostPort(t, listener.Addr().String())

	src := `
let fd = net_connect("127.0.0.1", ` + strconv.Itoa(port) + `);
if fd < 0 {
    print("connect failed: " + net_error());
    exit(1);
}
net_settimeout(fd, 5000);
if net_send(fd, "ping\n") < 0 {
    print("send failed: " + net_error());
    exit(1);
}
let reply = net_recvline(fd);
if reply == null {
    print("no reply: " + net_error());
    exit(1);
}
print(reply);
net_close(fd);
`

	if got := evalOutput(t, src); got != "pong ping\n" {
		t.Errorf("expected the server's reply, got %q", got)
	}
}

func TestNetAsAServer(t *testing.T) {
	// The port is only known once the program has listened, so the program
	// prints it and the test reads it back through a pipe before connecting.
	src := `
let listener = net_listen(0);
if listener < 0 {
    print("listen failed: " + net_error());
    exit(1);
}
print(net_addr(listener));
net_settimeout(listener, 5000);

let conn = net_accept(listener);
if conn < 0 {
    print("accept failed: " + net_error());
    exit(1);
}
net_settimeout(conn, 5000);

let line = net_recvline(conn);
while line != null {
    net_send(conn, upper(line) + "\n");
    line = net_recvline(conn);
}
net_close(conn);
net_close(listener);
print("served");
`

	pr, pw := io.Pipe()
	ev := New(pw)
	ev.MaxSteps = 50_000_000

	done := make(chan error, 1)
	go func() {
		defer pw.Close()
		done <- runSource(ev, src)
	}()

	lines := bufio.NewScanner(pr)
	if !lines.Scan() {
		t.Fatalf("the program printed no address: %v", runError(done))
	}
	_, port := hostPort(t, lines.Text())

	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 5*time.Second)
	if err != nil {
		t.Fatalf("cannot connect to the CCWIG server: %v", err)
	}

	_, _ = io.WriteString(conn, "hello\n")
	reply, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf("reading the server's reply: %v", err)
	}
	if reply != "HELLO\n" {
		t.Errorf("expected %q, got %q", "HELLO\n", reply)
	}

	conn.Close()

	if !lines.Scan() || lines.Text() != "served" {
		t.Errorf("expected the server to finish, got %q (%v)", lines.Text(), runError(done))
	}
}

func TestConnectToAClosedPortReportsWhy(t *testing.T) {
	// Take a port and give it straight back, so it is almost certainly free.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot listen: %v", err)
	}
	_, port := hostPort(t, listener.Addr().String())
	listener.Close()

	src := `
let fd = net_connect("127.0.0.1", ` + strconv.Itoa(port) + `);
print(fd < 0);
print(net_error() != "");
`

	if got := evalOutput(t, src); got != "true\ntrue\n" {
		t.Errorf("expected a failed connect with a reason, got %q", got)
	}
}

func TestNetBuiltinTypeErrors(t *testing.T) {
	assertError(t, eval(t, `net_listen("http")`), "for the port must be an integer")
	assertError(t, eval(t, `net_connect(1, 80)`), "first argument to `net_connect` must be a string")
	assertError(t, eval(t, `net_send(1, 2)`), "must be a string, got INTEGER")
	assertError(t, eval(t, `net_recvline("x")`), "must be an integer")
	assertError(t, eval(t, `net_error(1)`), "wrong number of arguments")
}

func TestClosingAnUnknownDescriptorIsHarmless(t *testing.T) {
	// The C reading of a close on a descriptor that was never open is that
	// nothing happens and the error is there for anyone who looks.
	assertInspect(t, eval(t, "net_close(99)"), "null")
	assertInspect(t, eval(t, "net_addr(99)"), "")
}

// runSource parses and evaluates src on ev, reporting a parse failure or an
// error value as an error for the caller.
//
// It is separate from evalWith because a background run has no *testing.T to
// fail: the error travels back over the channel instead.
func runSource(ev *Evaluator, src string) error {
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		return fmt.Errorf("parser errors: %v", errs)
	}

	if errObj, ok := ev.Eval(program, object.NewEnvironment()).(*object.Error); ok {
		return fmt.Errorf("%s", errObj.Message)
	}
	return nil
}

// runError waits briefly for a background run to finish and returns whatever
// it reported, so that a failure in the background names its cause.
func runError(done chan error) error {
	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		return nil
	}
}

// hostPort splits "host:port", which may be a bracketed IPv6 address.
func hostPort(t *testing.T, addr string) (string, int) {
	t.Helper()

	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("cannot split %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("cannot read the port from %q: %v", addr, err)
	}
	return host, port
}
