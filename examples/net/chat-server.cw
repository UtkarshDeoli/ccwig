// chat-server: many clients, one thread.
//
//     ccwig chat-server.cw [port] [run-for-ms]
//
// This is the thing a C program would do with select() or poll(), done with
// what the language has: a short deadline on every socket, and a loop that
// asks each one whether anything has happened. There are no threads, no
// coroutines and no callback registry - one loop, one client at a time, and
// the deadline is what keeps any single client from holding it.
//
//     net_settimeout(listener, POLL_MS);
//     let conn = net_accept(listener);      // gives up after POLL_MS
//
// The loop does three things, in this order, over and over:
//
//   1. look for a new arrival, without waiting for one
//   2. read one line from each client that is already here, without waiting
//   3. repeat
//
// A deadline is an instant, not an interval, and it stays where it was put.
// The second pass through this loop would find the deadline already passed and
// every read returning nothing, so the deadline is set again before every
// call. That is easy to get wrong, and a server that got it wrong would look
// like one that had stopped hearing its clients.
//
// A single-threaded server is the honest way to see what a deadline is for.
// With threads, a blocking read costs one thread; here it costs the whole
// server, which is why there is not one.

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

// POLL_MS is how long one socket is waited on before the loop moves on. Small
// enough that eight clients are still visited several times a second, large
// enough that an idle server is not spinning.
//
// There is no `const` in this language, so these are ordinary names that
// nothing enforces. The convention of writing them in capitals is the whole of
// the protection.
let POLL_MS = 2;
let MAX_CLIENTS = 8;

let argv = args();

let port = 0;
let runFor = 5000;

if len(argv) > 0 {
	port = toInt(argv[0]);

	if port < 0 {
		print("not a port: " + argv[0]);
		exit(2);
	}
}

if len(argv) > 1 {
	runFor = toInt(argv[1]);

	if runFor < 1 {
		print("not a number of milliseconds: " + argv[1]);
		exit(2);
	}
}

let listener = net_listen(port);

if listener < 0 {
	print("cannot listen: " + net_error());
	exit(1);
}

// Printed before anything is accepted, so a test or a person knows where to
// connect without guessing at the port.
print("chat server listening on " + net_addr(listener));

// The clients, in parallel arrays. The language has no structs, so what would
// be one array of records is two arrays that are kept in step: fds[i] belongs
// to names[i], always, and every place that moves a client moves both.
let fds = fill(MAX_CLIENTS, -1);
let names = fill(MAX_CLIENTS, "");
let count = 0;

let started = time();
let served = 0;
let messages = 0;
let running = true;

// sayAll sends one line to every client. A client that cannot be written to is
// left alone rather than dropped here: dropping is the reader's job, and two
// places that both remove clients is how a server comes to lose one.
fn sayAll(text) {
	for let i = 0; i < count; i = i + 1 {
		net_settimeout(fds[i], POLL_MS);
		net_send(fds[i], text + "\n");
	}
}

// roster is everyone in the room, in one line. A client that joins is told who
// is already here rather than only being told how many, which is the
// difference between a room and a counter.
fn roster() {
	let text = "";

	for let i = 0; i < count; i = i + 1 {
		if i > 0 {
			text = text + ",";
		}
		text = text + names[i];
	}

	return text;
}

// drop removes the client at index i by moving the last one into its place.
// The arrays are a set, not a queue, so which client sits at which index does
// not matter - but it does mean the caller must not advance i, because the
// client now at i is one that has not been looked at yet.
fn drop(i) {
	net_close(fds[i]);

	count = count - 1;
	fds[i] = fds[count];
	names[i] = names[count];

	fds[count] = -1;
	names[count] = "";
}

while running {
	// 1. Someone new?
	net_settimeout(listener, POLL_MS);

	let conn = net_accept(listener);

	if conn >= 0 {
		if count < MAX_CLIENTS {
			let name = "guest" + str(served + 1);
			served = served + 1;

			fds[count] = conn;
			names[count] = name;
			count = count + 1;

			print(name + " joined from " + net_addr(conn) + ", " + str(count) + " here");

			net_settimeout(conn, POLL_MS);
			net_send(conn, "welcome " + name + ", " + str(count) + " here\n");
			net_send(conn, "here: " + roster() + "\n");
			sayAll(name + " joined");
		} else {
			net_settimeout(conn, POLL_MS);
			net_send(conn, "the room is full (" + str(MAX_CLIENTS) + ")\n");
			net_close(conn);
			print("turned away a client: the room is full");
		}
	} else {
		// An accept that gave up waiting is the normal case, not a failure.
		// Everything else - the listener closed, the machine out of
		// descriptors - is worth stopping for. `net_timedout` is what tells
		// the two apart without reading the error message, which belongs to
		// the operating system and is not a program's business.
		if net_timedout() == false {
			print("cannot accept: " + net_error());
			running = false;
		}
	}

	// 2. Did anyone say anything?
	let i = 0;

	while running && i < count {
		// Refreshed before every read, because the last one has expired.
		net_settimeout(fds[i], POLL_MS);

		let line = net_recvline(fds[i]);

		if line == null {
			if net_timedout() {
				// A deadline that passed with nothing on the wire. This is
				// the ordinary case for a quiet client, and the reason the
				// server can hear from anybody at all.
				i = i + 1;
			} else {
				// Nothing left to read, and no deadline to blame: the client
				// is gone. It is `net_timedout` that separates the two, and
				// without it a server cannot tell a quiet client from a
				// departed one.
				sayAll(names[i] + " left");
				print(names[i] + " left, " + str(count - 1) + " here");
				drop(i);
			}
		} else if line == "quit" {
			running = false;
		} else {
			messages = messages + 1;
			print(names[i] + ": " + line);
			sayAll(names[i] + ": " + line);
			i = i + 1;
		}
	}

	// 3. A run has to end by itself. Without this, a server with nobody
	// talking to it would sit here forever, and an example that cannot finish
	// is an example nobody can run.
	if time() - started > runFor {
		running = false;
	}
}

sayAll("the server is closing");

for let i = 0; i < count; i = i + 1 {
	net_close(fds[i]);
}

net_close(listener);

print("chat server done: " + str(served) + " client(s), " + str(messages) + " message(s)");
