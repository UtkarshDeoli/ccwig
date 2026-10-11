// Package sockets gives a CCWIG program real TCP connections.
//
// The shape is deliberately C's: a connection is an integer file descriptor,
// a call that fails returns a negative number or an empty result, and the
// reason is kept aside for `net_error` to hand back. That is not how the rest
// of the language reports failure — everywhere else an error is a value that
// halts the program — but socket code written the C way composes: `if fd < 0`
// reads the same here as it does in a C program, and a server can refuse one
// connection and carry on serving.
//
// Sockets start at 3, so that 0, 1 and 2 are what a C programmer assumes they
// are. A descriptor is never reused within a run, which costs nothing and
// removes every stale-descriptor aliasing bug.
//
// A Registry is not safe for concurrent use. It does not need to be: the
// evaluator that drives it is single-threaded, so calls block the interpreter
// the way a one-client-at-a-time C server blocks. No goroutines are started
// here, and none are hidden.
package sockets

import (
	"bufio"
	"errors"
	"io"
	"net"
	"strconv"
	"time"
)

// firstFD is the first descriptor handed out. It matches the Unix convention
// that 0, 1 and 2 are already taken.
const firstFD = 3

// RecvMax caps a single read, so that a program cannot ask for a buffer larger
// than the machine has and be surprised by the answer.
const RecvMax = 1 << 20

// dialTimeout bounds `Connect`, so that an unreachable host fails in a
// reported way rather than hanging the interpreter.
const dialTimeout = 10 * time.Second

// socket is one open descriptor: either a listening socket or a connection.
type socket struct {
	fd       int
	listener net.Listener
	conn     net.Conn
	// reader buffers reads, so that `recv` and `recvline` can be mixed on one
	// connection without either losing bytes the other had already pulled in.
	reader *bufio.Reader
}

// Registry holds the open sockets and the last error to occur on one.
type Registry struct {
	open    map[int]*socket
	nextFD  int
	lastErr string
	// lastTimeout remembers whether that error was a deadline passing rather
	// than the socket breaking. The two are different answers to "should I try
	// again?", and a program should not have to read the operating system's
	// message to find out which one it got.
	lastTimeout bool
}

// NewRegistry returns a Registry with no sockets open.
func NewRegistry() *Registry {
	return &Registry{open: make(map[int]*socket), nextFD: firstFD}
}

// LastError returns the most recent socket error, or the empty string if the
// last socket call succeeded.
func (r *Registry) LastError() string { return r.lastErr }

// TimedOut reports whether the last failure was a deadline passing.
//
// A timeout is not a broken socket: the connection is still good and the same
// call may work a moment later. This is that difference, made visible, so that
// a server polling many clients can tell "nothing has arrived yet" from "this
// one is gone" without matching on the text of a message that belongs to the
// operating system.
func (r *Registry) TimedOut() bool { return r.lastTimeout }

// clear forgets the last error, which every socket call does on its way in.
func (r *Registry) clear() {
	r.lastErr = ""
	r.lastTimeout = false
}

// Fail records msg as the last error and returns the -1 a failed descriptor
// call yields.
func (r *Registry) Fail(msg string) int {
	r.clear()
	r.lastErr = msg
	return -1
}

// FailErr records an error from the network stack, keeping whether it was a
// deadline passing, and returns the -1 a failed descriptor call yields.
func (r *Registry) FailErr(err error) int {
	r.lastErr = err.Error()
	r.lastTimeout = isTimeout(err)
	return -1
}

// isTimeout asks the error itself rather than reading its message: the message
// is the operating system's, and it is not this package's to parse.
func isTimeout(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

// Listen opens a listening socket on the given port. Port 0 asks the system
// for a free one, which `Addr` then reports.
func (r *Registry) Listen(port int) int {
	r.clear()

	if port < 0 || port > 65535 {
		return r.Fail("port out of range: " + strconv.Itoa(port))
	}

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return r.Fail(err.Error())
	}

	return r.add(&socket{listener: listener})
}

// Accept waits for a connection on a listening socket and returns a
// descriptor for it, or -1 if it fails or the timeout is reached.
func (r *Registry) Accept(fd int) int {
	r.clear()

	s, ok := r.open[fd]
	if !ok {
		return r.Fail("not an open socket: " + strconv.Itoa(fd))
	}
	if s.listener == nil {
		return r.Fail("not a listening socket: " + strconv.Itoa(fd))
	}

	conn, err := s.listener.Accept()
	if err != nil {
		return r.FailErr(err)
	}

	return r.add(&socket{conn: conn, reader: bufio.NewReader(conn)})
}

// Connect opens a connection to host:port, or returns -1.
func (r *Registry) Connect(host string, port int) int {
	r.clear()

	if port < 0 || port > 65535 {
		return r.Fail("port out of range: " + strconv.Itoa(port))
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), dialTimeout)
	if err != nil {
		return r.FailErr(err)
	}

	return r.add(&socket{conn: conn, reader: bufio.NewReader(conn)})
}

// Send writes all of data and returns the number of bytes written, or -1.
//
// It writes everything rather than however much the first write accepted,
// because a partial write that a caller ignores is a bug that shows up as a
// truncated message much later.
func (r *Registry) Send(fd int, data string) int {
	r.clear()

	s, err := r.connection(fd)
	if err != nil {
		return r.Fail(err.Error())
	}

	written, err := io.WriteString(s.conn, data)
	if err != nil {
		return r.FailErr(err)
	}
	return written
}

// Recv reads up to max bytes, returning what it read and whether the stream
// ended.
//
// A read of zero bytes that is not an end of stream (a timeout, say) is
// reported as an error: the caller gets no data and LastError names the
// reason, rather than being handed an end of stream that never happened.
func (r *Registry) Recv(fd, max int) (string, bool) {
	r.clear()

	if max < 1 || max > RecvMax {
		r.Fail("recv size must be between 1 and " + strconv.Itoa(RecvMax))
		return "", false
	}

	s, err := r.connection(fd)
	if err != nil {
		r.Fail(err.Error())
		return "", false
	}

	buf := make([]byte, max)
	n, err := s.reader.Read(buf)

	if n > 0 {
		// Data arrived. A non-nil error is kept for the next call to report;
		// handing back the bytes now is what makes a final short read before
		// an end of stream useful rather than lost.
		if err != nil {
			r.FailErr(err)
		}
		return string(buf[:n]), false
	}

	if errors.Is(err, io.EOF) {
		return "", true
	}
	if err != nil {
		r.FailErr(err)
		return "", false
	}

	// A read of nothing and no error: treat it as the end of the stream, which
	// is what a reader that returns zero and no error means.
	return "", true
}

// RecvLine reads one line, without its terminator, returning whether the
// stream ended.
//
// The end of the stream is only reported when it comes with no data; a final
// line that was never terminated by a newline is still returned.
func (r *Registry) RecvLine(fd int) (string, bool) {
	r.clear()

	s, err := r.connection(fd)
	if err != nil {
		r.Fail(err.Error())
		return "", false
	}

	line, err := s.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		r.FailErr(err)
		return "", false
	}
	if line == "" && errors.Is(err, io.EOF) {
		return "", true
	}

	return trimLineEnding(line), false
}

// Close releases a descriptor. It reports whether the descriptor was open.
func (r *Registry) Close(fd int) bool {
	r.clear()

	s, ok := r.open[fd]
	if !ok {
		return false
	}

	if s.conn != nil {
		_ = s.conn.Close()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	delete(r.open, fd)

	return true
}

// SetTimeout gives a socket a deadline ms milliseconds from now. Zero removes
// the deadline, which is the default.
func (r *Registry) SetTimeout(fd, ms int) bool {
	r.clear()

	s, ok := r.open[fd]
	if !ok {
		r.Fail("not an open socket: " + strconv.Itoa(fd))
		return false
	}
	if ms < 0 {
		r.Fail("timeout must not be negative: " + strconv.Itoa(ms))
		return false
	}

	deadline := time.Time{}
	if ms > 0 {
		deadline = time.Now().Add(time.Duration(ms) * time.Millisecond)
	}

	var err error
	switch {
	case s.conn != nil:
		err = s.conn.SetDeadline(deadline)
	case s.listener != nil:
		err = s.listener.(*net.TCPListener).SetDeadline(deadline)
	}
	if err != nil {
		r.Fail(err.Error())
		return false
	}

	return true
}

// Addr returns the address at the other end of a connection, or a listening
// socket's own address, or the empty string.
func (r *Registry) Addr(fd int) string {
	s, ok := r.open[fd]
	if !ok {
		return ""
	}
	if s.conn != nil {
		return s.conn.RemoteAddr().String()
	}
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}

// Open reports how many sockets are open, which the tests use to check that a
// program that closes what it opens leaks nothing.
func (r *Registry) Open() int { return len(r.open) }

// connection returns the socket at fd, insisting that it is a connection
// rather than a listening socket.
func (r *Registry) connection(fd int) (*socket, error) {
	s, ok := r.open[fd]
	if !ok {
		return nil, errors.New("not an open socket: " + strconv.Itoa(fd))
	}
	if s.conn == nil {
		return nil, errors.New("not a connected socket: " + strconv.Itoa(fd))
	}
	return s, nil
}

// add registers a socket and returns its descriptor.
func (r *Registry) add(s *socket) int {
	fd := r.nextFD
	r.nextFD++

	s.fd = fd
	r.open[fd] = s

	return fd
}

// trimLineEnding removes a trailing newline and any carriage return before it.
func trimLineEnding(line string) string {
	if n := len(line); n > 0 && line[n-1] == '\n' {
		line = line[:n-1]
	}
	if n := len(line); n > 0 && line[n-1] == '\r' {
		line = line[:n-1]
	}
	return line
}
