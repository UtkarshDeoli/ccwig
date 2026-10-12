// http-get: fetch a page and report what came back.
//
//     ccwig http-get.cw example.com 80 /
//
// HTTP/1.0 is the version of HTTP a program can speak without keeping state.
// The request is a line, some headers and a blank line; the answer is a status
// line, more headers, another blank line, and then the body - and the server
// closes the connection when the body ends, which is what makes the end of the
// stream the end of the body and saves counting Content-Length.
//
// Speaking the protocol by hand is the fastest way to stop thinking of it as
// magic. There is no magic: it is lines of text over a socket, and the whole
// of the exchange is below.

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

if len(argv) < 3 {
	print("usage: http-get <host> <port> <path>");
	exit(2);
}

let host = argv[0];
let port = toInt(argv[1]);
let path = argv[2];

if port < 0 {
	print("not a port: " + argv[1]);
	exit(2);
}

let fd = net_connect(host, port);

if fd < 0 {
	print("cannot connect to " + host + ":" + str(port) + ": " + net_error());
	exit(1);
}

net_settimeout(fd, 10000);

// The blank line at the end is not decoration. It is what tells the server the
// request is over, and a request without it is a request the server waits on
// until the client gives up.
let request = "GET " + path + " HTTP/1.0\r\n"
	+ "Host: " + host + "\r\n"
	+ "User-Agent: ccwig\r\n"
	+ "Connection: close\r\n"
	+ "\r\n";

if net_send(fd, request) < 0 {
	print("send failed: " + net_error());
	net_close(fd);
	exit(1);
}

let status = net_recvline(fd);

if status == null {
	print("no response: " + net_error());
	net_close(fd);
	exit(1);
}

print("status:   " + status);

// The headers run until the first empty line. Only the two that say what the
// body is and how big it is are worth printing; the rest are for a browser.
let headers = 0;
let line = net_recvline(fd);

while line != null && len(line) > 0 {
	headers = headers + 1;

	if lower(substr(line, 0, 5)) == "conte" {
		print("header:   " + line);
	}

	line = net_recvline(fd);
}

print("headers:  " + str(headers));

// What is left on the connection is the body, read in pieces until the server
// closes. A large page arrives in many pieces, and none of them is guaranteed
// to be a whole anything: TCP hands over bytes, and the lines and headers
// above were an interpretation this program put on them.
let body = "";
let piece = net_recv(fd, 4096);

while piece != null {
	body = body + piece;
	piece = net_recv(fd, 4096);
}

print("body:     " + str(len(body)) + " bytes");
print("first:    " + substr(body, 0, 60));

net_close(fd);
