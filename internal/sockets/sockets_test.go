package sockets

import (
	"net"
	"strconv"
	"strings"
	"testing"
)

// listenAndConnect brings up a listener and a client connected to it, and
// returns the listening descriptor, the accepted descriptor and the client's.
//
// Connecting before accepting works because the kernel completes the handshake
// into the listen backlog without waiting for Accept, so one goroutine can set
// the pair up without deadlocking.
func listenAndConnect(t *testing.T, r *Registry) (listenerFD, serverFD, clientFD int) {
	t.Helper()

	listenerFD = r.Listen(0)
	if listenerFD < 0 {
		t.Fatalf("Listen failed: %s", r.LastError())
	}

	host, port := splitAddr(t, r.Addr(listenerFD))

	// Every socket carries a deadline so that a mistake in a test reports
	// itself instead of hanging the suite.
	r.SetTimeout(listenerFD, testTimeoutMS)

	clientFD = r.Connect(host, port)
	if clientFD < 0 {
		t.Fatalf("Connect failed: %s", r.LastError())
	}
	r.SetTimeout(clientFD, testTimeoutMS)

	serverFD = r.Accept(listenerFD)
	if serverFD < 0 {
		t.Fatalf("Accept failed: %s", r.LastError())
	}
	r.SetTimeout(serverFD, testTimeoutMS)

	return listenerFD, serverFD, clientFD
}

// testTimeoutMS bounds every socket a test opens, so that a read that would
// otherwise wait forever fails with a timeout instead of hanging the suite.
const testTimeoutMS = 5000

// splitAddr turns "127.0.0.1:1234" into its parts.
func splitAddr(t *testing.T, addr string) (string, int) {
	t.Helper()

	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("cannot split address %q: %v", addr, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("cannot read port from %q: %v", addr, err)
	}
	return host, port
}

func TestListenConnectSendRecv(t *testing.T) {
	r := NewRegistry()
	listenerFD, serverFD, clientFD := listenAndConnect(t, r)

	if got := r.Send(clientFD, "ping"); got != 4 {
		t.Fatalf("expected to send 4 bytes, sent %d (%s)", got, r.LastError())
	}

	got, eof := r.Recv(serverFD, 1024)
	if eof {
		t.Fatal("unexpected end of stream")
	}
	if got != "ping" {
		t.Errorf("expected %q, got %q", "ping", got)
	}

	for _, fd := range []int{listenerFD, serverFD, clientFD} {
		if !r.Close(fd) {
			t.Errorf("closing fd %d failed", fd)
		}
	}
	if r.Open() != 0 {
		t.Errorf("expected every socket closed, %d still open", r.Open())
	}
}

func TestDescriptorsStartAtThree(t *testing.T) {
	// 0, 1 and 2 are standard input, output and error, and a C programmer
	// reading this code expects the first socket to be 3.
	r := NewRegistry()

	fd := r.Listen(0)
	defer r.Close(fd)

	if fd != 3 {
		t.Errorf("expected the first descriptor to be 3, got %d", fd)
	}
}

func TestRecvLineReadsWholeLines(t *testing.T) {
	r := NewRegistry()
	_, serverFD, clientFD := listenAndConnect(t, r)
	defer r.Close(serverFD)
	defer r.Close(clientFD)

	if got := r.Send(clientFD, "first\nsecond\nthird without newline"); got < 0 {
		t.Fatalf("Send failed: %s", r.LastError())
	}

	// The last line has no terminator, so it only becomes readable once the
	// sender is done: that is what tells the reader there is nothing more.
	r.Close(clientFD)

	for i, want := range []string{"first", "second", "third without newline"} {
		got, eof := r.RecvLine(serverFD)
		if eof {
			t.Fatalf("line %d: unexpected end of stream", i)
		}
		if got != want {
			t.Errorf("line %d: expected %q, got %q", i, want, got)
		}
	}
}

func TestRecvAndRecvLineShareTheBuffer(t *testing.T) {
	// Mixing the two must not lose bytes: a line read after a partial read
	// starts where the partial read stopped.
	r := NewRegistry()
	_, serverFD, clientFD := listenAndConnect(t, r)
	defer r.Close(serverFD)
	defer r.Close(clientFD)

	r.Send(clientFD, "abcdef\nrest\n")

	got, _ := r.Recv(serverFD, 3)
	if got != "abc" {
		t.Fatalf("expected %q, got %q", "abc", got)
	}

	got, _ = r.RecvLine(serverFD)
	if got != "def" {
		t.Errorf("expected the rest of the line, got %q", got)
	}
}

func TestEndOfStreamIsReported(t *testing.T) {
	r := NewRegistry()
	_, serverFD, clientFD := listenAndConnect(t, r)
	defer r.Close(serverFD)

	r.Send(clientFD, "last")
	r.Close(clientFD)

	got, eof := r.Recv(serverFD, 1024)
	if eof {
		t.Fatal("the data before the end of stream was lost")
	}
	if got != "last" {
		t.Errorf("expected %q, got %q", "last", got)
	}

	// The next read sees the end.
	if _, eof := r.Recv(serverFD, 1024); !eof {
		t.Error("expected the end of the stream to be reported")
	}
}

func TestConnectToNothingFails(t *testing.T) {
	r := NewRegistry()

	// Take a port, then close it, so the port is almost certainly unused.
	fd := r.Listen(0)
	_, port := splitAddr(t, r.Addr(fd))
	r.Close(fd)

	if got := r.Connect("127.0.0.1", port); got != -1 {
		t.Errorf("expected -1 connecting to a closed port, got %d", got)
	}
	if r.LastError() == "" {
		t.Error("expected the failure to be described")
	}
}

func TestOperationsOnTheWrongKindOfSocketFail(t *testing.T) {
	r := NewRegistry()
	listenerFD, serverFD, clientFD := listenAndConnect(t, r)
	defer r.Close(listenerFD)
	defer r.Close(serverFD)
	defer r.Close(clientFD)

	if got := r.Send(listenerFD, "x"); got != -1 {
		t.Errorf("expected sending on a listener to fail, got %d", got)
	}
	if got := r.Accept(clientFD); got != -1 {
		t.Errorf("expected accepting on a connection to fail, got %d", got)
	}
	if _, eof := r.Recv(listenerFD, 10); eof {
		t.Error("receiving on a listener should be an error, not an end of stream")
	}
	if r.LastError() == "" {
		t.Error("expected the failure to be described")
	}
}

func TestUnknownDescriptorFails(t *testing.T) {
	r := NewRegistry()

	if got := r.Send(99, "x"); got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
	if !strings.Contains(r.LastError(), "99") {
		t.Errorf("expected the descriptor to be named, got %q", r.LastError())
	}
	if r.Close(99) {
		t.Error("closing a descriptor that was never open should report that")
	}
}

func TestAcceptTimeout(t *testing.T) {
	r := NewRegistry()
	fd := r.Listen(0)
	defer r.Close(fd)

	if !r.SetTimeout(fd, 50) {
		t.Fatalf("SetTimeout failed: %s", r.LastError())
	}

	// Nothing connects, so Accept must give up rather than hang.
	if got := r.Accept(fd); got != -1 {
		t.Fatalf("expected the accept to time out, got fd %d", got)
	}
	if r.LastError() == "" {
		t.Error("expected the timeout to be described")
	}
}

func TestRecvSizeIsBounded(t *testing.T) {
	r := NewRegistry()
	_, serverFD, clientFD := listenAndConnect(t, r)
	defer r.Close(serverFD)
	defer r.Close(clientFD)

	if _, eof := r.Recv(serverFD, 0); eof {
		t.Error("a zero size should be an error, not an end of stream")
	}
	if !strings.Contains(r.LastError(), "recv size") {
		t.Errorf("expected the size to be explained, got %q", r.LastError())
	}
	if _, eof := r.Recv(serverFD, RecvMax+1); eof {
		t.Error("an oversized read should be an error, not an end of stream")
	}
}

func TestNegativeTimeoutIsRefused(t *testing.T) {
	r := NewRegistry()
	fd := r.Listen(0)
	defer r.Close(fd)

	if r.SetTimeout(fd, -1) {
		t.Error("expected a negative timeout to be refused")
	}
}

func TestAddrReportsTheListenerPort(t *testing.T) {
	r := NewRegistry()
	fd := r.Listen(0)
	defer r.Close(fd)

	addr := r.Addr(fd)
	if !strings.Contains(addr, ":") {
		t.Fatalf("expected host:port, got %q", addr)
	}
	if r.Addr(99) != "" {
		t.Error("expected an unknown descriptor to have no address")
	}
}
