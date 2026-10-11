// netbench: the CCWIG half of the loopback benchmark the reports are made from.
//
//     ccwig netbench.cw 127.0.0.1 9000 1024 5000
//
// It connects to an echo server, sends a payload of `size` bytes and waits for
// the same number of bytes to come back, `count` times, and prints one line of
// numbers that the harness reads:
//
//     netbench size=1024 count=5000 ms=812 mismatches=0
//
// The harness measures the wall-clock time of the whole process; the `ms` here
// is what the program itself saw between the first send and the last receive.
// The two numbers differ by the interpreter starting up and reading the
// source, which is a real cost and is left in rather than hidden: a benchmark
// that quietly discards the parts it does not like is not a benchmark.
//
// `time()` counts milliseconds, so a run has to last long enough for a
// millisecond to be a small part of it. The counts the harness uses are chosen
// for that, not because the numbers are round.

fn toInt(s) {
	let value = 0;

	for let i = 0; i < len(s); i = i + 1 {
		let ch = s[i];

		if ch < "0" || ch > "9" {
			return -1;
		}

		value = value * 10 + (ord(ch) - ord("0"));
	}

	return value;
}

// payloadOf builds a payload of exactly `size` bytes from a repeating block.
// Repeated content is what makes a mismatch easy to see in a trace, and a
// fixed payload means two runs differ only in the speed of the machine.
fn payloadOf(size) {
	let block = "0123456789abcdef";
	let out = "";

	while len(out) < size {
		out = out + block;
	}

	return substr(out, 0, size);
}

let argv = args();

if len(argv) < 4 {
	print("usage: netbench <host> <port> <size> <count>");
	exit(2);
}

let host = argv[0];
let port = toInt(argv[1]);
let size = toInt(argv[2]);
let count = toInt(argv[3]);

if port < 0 || size < 1 || count < 1 {
	print("usage: netbench <host> <port> <size> <count>");
	exit(2);
}

let payload = payloadOf(size);

let fd = net_connect(host, port);

if fd < 0 {
	print("cannot connect to " + host + ":" + str(port) + ": " + net_error());
	exit(1);
}

// No timeout worth the name: a benchmark that gives up part way through and
// reports the time it took to give up would be worse than one that hangs. The
// deadline here is only to keep a mistake from waiting forever.
net_settimeout(fd, 30000);

let started = time();
let mismatches = 0;

for let i = 0; i < count; i = i + 1 {
	if net_send(fd, payload) != size {
		print("send failed after " + str(i) + " round trips: " + net_error());
		exit(1);
	}

	// TCP hands over bytes, not messages, so a reply can arrive in pieces and
	// each read can return less than was asked for. Reading until the payload
	// is complete is what makes a slow machine and a split packet the same
	// thing to this loop.
	let received = "";
	let piece = net_recv(fd, size);

	while piece != null && len(received) < size {
		received = received + piece;
		piece = net_recv(fd, size - len(received));
	}

	if len(received) != size {
		print("the connection ended after " + str(i) + " round trips: " + net_error());
		exit(1);
	}

	if received != payload {
		mismatches = mismatches + 1;
	}
}

let elapsed = time() - started;

net_close(fd);

print("netbench size=" + str(size) + " count=" + str(count)
	+ " ms=" + str(elapsed)
	+ " mismatches=" + str(mismatches));
