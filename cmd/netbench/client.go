package main

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

// The Go client is the baseline the CCWIG client is compared against. It does
// the same exchange - send a payload, read exactly that many bytes back, a
// fixed number of times - over the same socket to the same server, so that the
// only difference between two rows of the report is the language the client
// was written in.
//
// The comparison is not flattering to CCWIG and is not meant to be. A
// tree-walking interpreter calling a builtin per operation cannot be as fast
// as compiled Go, and a benchmark designed to avoid saying so would be
// measuring something other than the thing anyone wants to know.
func runGoClient(peer string, size, count int) error {
	if peer == "" || size < 1 || count < 1 {
		return fmt.Errorf("the client needs -peer, -size and -count")
	}

	conn, err := net.DialTimeout("tcp", peer, 10*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	payload := payloadOf(size)
	// The reply is read into a buffer that is reused between round trips, so
	// that allocating is not part of what is being timed.
	buffer := make([]byte, size)

	started := time.Now()
	mismatches := 0

	for i := 0; i < count; i++ {
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("send failed after %d round trips: %w", i, err)
		}

		// TCP hands over bytes, not messages, so one read can return less than
		// was asked for. Reading until the payload is complete is what makes
		// the loop independent of how the bytes were split up.
		read := 0
		for read < size {
			conn.SetDeadline(time.Now().Add(30 * time.Second))

			n, err := conn.Read(buffer[read:])
			if err != nil {
				return fmt.Errorf("the connection ended after %d round trips: %w", i, err)
			}
			read += n
		}

		if !bytes.Equal(buffer[:read], payload) {
			mismatches++
		}
	}

	elapsed := time.Since(started)

	// The same line the CCWIG client prints, so the harness reads one format
	// from either one. The loop is timed from just before the first write to
	// just after the last read, which is the same span the CCWIG client times,
	// and both are given in milliseconds and in microseconds so that the two
	// reports can each read the one they need.
	fmt.Printf("netbench size=%d count=%d ms=%d us=%d mismatches=%d\n",
		size, count, elapsed.Milliseconds(), elapsed.Microseconds(), mismatches)

	return nil
}

// payloadOf builds a payload of exactly size bytes from a repeating block,
// character for character the way examples/net/netbench.cw builds it. The two
// clients have to send the same bytes, or the comparison is between two
// different amounts of work.
func payloadOf(size int) []byte {
	const block = "0123456789abcdef"

	out := make([]byte, 0, size+len(block))
	for len(out) < size {
		out = append(out, block...)
	}
	return out[:size]
}
