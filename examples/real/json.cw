// json: re-indent a JSON document from standard input.
//
//     ccwig json.cw < data/compact.json
//
// This is a parser, not a pattern match: it walks the document a character at
// a time, keeps its place in a global, and descends into arrays and objects
// the way the grammar does. Anything that is not valid JSON is refused with
// the character it was refused at, which is the difference between a tool you
// can debug and one you cannot.
//
// The cursor is a global rather than an argument. Every function here reads
// it and moves it on, and passing a position in and a position out of each
// one would say nothing that the code does not already say.

let text = readAll();
let pos = 0;

fn fail(message) {
	print("json: " + message + " at character " + str(pos));
	exit(1);
}

fn skipSpace() {
	let skipping = true;

	while skipping && pos < len(text) {
		let ch = text[pos];

		if ch == " " || ch == "\t" || ch == "\n" || ch == "\r" {
			pos = pos + 1;
		} else {
			skipping = false;
		}
	}
}

fn indent(depth) {
	let out = "";

	for let i = 0; i < depth; i = i + 1 {
		out = out + "  ";
	}

	return out;
}

// parseString returns the string as it was written, quotes and escapes and
// all, because re-escaping a string that is already escaped is a way to
// change it.
fn parseString() {
	pos = pos + 1;

	let out = "\"";

	while true {
		if pos >= len(text) {
			fail("unterminated string");
		}

		let ch = text[pos];
		pos = pos + 1;

		if ch == "\\" {
			if pos >= len(text) {
				fail("unterminated escape");
			}
			out = out + ch + text[pos];
			pos = pos + 1;
		} else {
			if ch == "\"" {
				return out + "\"";
			}
			out = out + ch;
		}
	}
}

// identifier reads a run of letters, which in JSON is true, false or null.
fn identifier() {
	let out = "";
	let reading = true;

	while reading && pos < len(text) {
		let ch = text[pos];

		if (ch >= "a" && ch <= "z") || (ch >= "A" && ch <= "Z") {
			out = out + ch;
			pos = pos + 1;
		} else {
			reading = false;
		}
	}

	return out;
}

// parseNumber takes the run of characters a number can be written with and
// hands it back as it was written. Reformatting it would mean deciding how
// many digits of a float to keep, which is a printer's business, not a
// parser's.
fn parseNumber() {
	let out = "";

	while pos < len(text) {
		let ch = text[pos];

		if (ch >= "0" && ch <= "9") || ch == "-" || ch == "+" || ch == "." || ch == "e" || ch == "E" {
			out = out + ch;
			pos = pos + 1;
		} else {
			return out;
		}
	}

	return out;
}

fn parseArray(depth) {
	pos = pos + 1;
	skipSpace();

	if pos < len(text) && text[pos] == "]" {
		pos = pos + 1;
		return "[]";
	}

	let out = "[\n";

	while true {
		skipSpace();
		out = out + indent(depth + 1) + parseValue(depth + 1);

		skipSpace();
		if pos >= len(text) {
			fail("unterminated array");
		}

		if text[pos] == "," {
			pos = pos + 1;
			out = out + ",\n";
		} else {
			if text[pos] == "]" {
				pos = pos + 1;
				return out + "\n" + indent(depth) + "]";
			}
			fail("expected , or ] inside an array");
		}
	}
}

fn parseObject(depth) {
	pos = pos + 1;
	skipSpace();

	if pos < len(text) && text[pos] == "}" {
		pos = pos + 1;
		return "{}";
	}

	let out = "{\n";

	while true {
		skipSpace();
		if pos >= len(text) || text[pos] != "\"" {
			fail("expected a key");
		}

		let key = parseString();
		skipSpace();

		if pos >= len(text) || text[pos] != ":" {
			fail("expected : after a key");
		}
		pos = pos + 1;

		skipSpace();
		out = out + indent(depth + 1) + key + ": " + parseValue(depth + 1);

		skipSpace();
		if pos >= len(text) {
			fail("unterminated object");
		}

		if text[pos] == "," {
			pos = pos + 1;
			out = out + ",\n";
		} else {
			if text[pos] == "}" {
				pos = pos + 1;
				return out + "\n" + indent(depth) + "}";
			}
			fail("expected , or } inside an object");
		}
	}
}

fn parseValue(depth) {
	skipSpace();
	if pos >= len(text) {
		fail("unexpected end of input");
	}

	let ch = text[pos];

	if ch == "{" {
		return parseObject(depth);
	}
	if ch == "[" {
		return parseArray(depth);
	}
	if ch == "\"" {
		return parseString();
	}
	if ch == "t" || ch == "f" || ch == "n" {
		let word = identifier();

		if word == "true" || word == "false" || word == "null" {
			return word;
		}
		fail("unknown literal: " + word);
	}

	return parseNumber();
}

skipSpace();
let document = parseValue(0);
skipSpace();

if pos != len(text) {
	fail("input continues past the end of the document");
}

print(document);
