// echo-server: send every line back with the number it arrived in.
//
//     ccwig echo-server.cw          listen on a port the system picks
//     ccwig echo-server.cw 8080     listen on 8080
//
// A server on a single-threaded interpreter is not a limited thing: it is
// accept a connection, serve it until the client goes away, accept the next.
// That is how servers were written before threads, and it is why `net_accept`
// and `net_recvline` block rather than returning a future.
//
// The server prints the address it is bound to before it accepts anything, so
// that a program started with no port of its own can say which port it got.
// That line is what a test waits for, and what a person needs in order to
// connect to a server that chose its own port.

// toInt reads a whole number out of the text a command line arrives as, or -1
// if it is not one. Every program that takes a number as an argument needs
// this and has to carry its own copy, there being no library to put it in.
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

let argv = args();
let port = 0;

if len(argv) > 0 {
	port = toInt(argv[0]);

	if port < 0 {
		print("not a port: " + argv[0]);
		exit(2);
	}
}

let listener = net_listen(port);

if listener < 0 {
	print("cannot listen: " + net_error());
	exit(1);
}

print("listening on " + net_addr(listener));

// A timeout is what keeps this program a program that ends. Without one, an
// accept with no client on the way waits forever, and a server that has to be
// killed is a server nobody can test.
net_settimeout(listener, 10000);

let conn = net_accept(listener);

if conn < 0 {
	print("no client arrived: " + net_error());
	net_close(listener);
	exit(1);
}

print("client from " + net_addr(conn));
net_settimeout(conn, 5000);

let line = net_recvline(conn);
let lines = 0;
let bytes = 0;
let talking = true;

// A way for the client to say it is finished, as a word of its own rather than
// by going quiet. A client that can only hang up by closing is a client that
// leaves the server reading a socket until the deadline it set runs out.
// The language has no `break`, so the loop carries a flag.
while talking && line != null {
	if line == "quit" {
		net_send(conn, "goodbye\n");
		talking = false;
	} else {
		lines = lines + 1;
		// The newline that ended the line is not part of it, and is part of
		// what went over the wire, so the byte count adds it back.
		bytes = bytes + len(line) + 1;

		if net_send(conn, str(lines) + " " + line + "\n") < 0 {
			print("send failed: " + net_error());
			net_close(conn);
			net_close(listener);
			exit(1);
		}

		line = net_recvline(conn);
	}
}

// The client closed, which is the ordinary way a connection ends. Whether it
// went away politely or was cut off, `net_error` has the reason if anyone
// wants it, and the line the loop stopped on is null either way.
net_close(conn);
net_close(listener);

print("served " + str(lines) + " lines, " + str(bytes) + " bytes");
