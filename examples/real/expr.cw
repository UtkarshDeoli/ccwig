// expr: evaluate arithmetic expressions, one per line, from standard input.
//
//     ccwig expr.cw < expr.in
//
// This is the recursion every language implementation begins with: an
// expression is a sum of terms, a term is a product of factors, and a factor
// is a number or a bracketed expression. Written out, the precedence rules a
// reader usually takes on trust become a shape in the code that can be
// pointed at.
//
// The arithmetic is floating point throughout, which is why 0.1 + 0.2 prints
// more digits than a pocket calculator would. The extra digits are the truth
// about binary floating point; rounding them away would be the calculator
// lying about the sum it just did.
//
// A line that will not parse is reported with the column it went wrong at and
// the program carries on to the next line. The exit status is the one a
// compiler uses: zero if every line was evaluated, one if any line was not.

let line = "";
let pos = 0;
let failures = 0;

// at is the character the cursor is on, or the empty string past the end.
// Reading past the end has to be a non-event here, because half the tests in
// this program are "is there more?", and they would all have to be written
// twice if it were an error.
fn at() {
	if pos < len(line) {
		return line[pos];
	}
	return "";
}

fn isDigit(ch) {
	return ch >= "0" && ch <= "9";
}

fn skipSpace() {
	while at() == " " {
		pos = pos + 1;
	}
}

// report counts a failure and says where it happened. Columns are counted
// from one, as they are in every compiler's message.
fn report(message) {
	print("expr: " + message + " (column " + str(pos + 1) + ")");
	failures = failures + 1;
}

// number reads a whole number with an optional fraction. The whole part is
// built as an integer and the fraction is added a digit at a time, each digit
// worth a tenth of the one before it.
fn number() {
	let digits = 0;
	let whole = 0;

	while isDigit(at()) {
		whole = whole * 10 + (ord(at()) - ord("0"));
		pos = pos + 1;
		digits = digits + 1;
	}

	let value = whole * 1.0;

	if at() == "." {
		pos = pos + 1;
		let scale = 1.0;

		while isDigit(at()) {
			scale = scale / 10;
			value = value + (ord(at()) - ord("0")) * scale;
			pos = pos + 1;
			digits = digits + 1;
		}
	}

	// A lone "." or "-" is not a number.
	if digits == 0 {
		return null;
	}

	return value;
}

// factor is the tightest level: a number, a bracketed expression, or either
// of those negated.
fn factor() {
	skipSpace();

	if at() == "(" {
		pos = pos + 1;

		let inner = expression();
		if inner == null {
			return null;
		}

		skipSpace();
		if at() != ")" {
			report("expected a closing bracket");
			return null;
		}
		pos = pos + 1;

		return inner;
	}

	if at() == "-" || at() == "+" {
		let sign = at();
		pos = pos + 1;

		let value = factor();
		if value == null {
			return null;
		}

		if sign == "-" {
			return 0 - value;
		}
		return value;
	}

	let value = number();
	if value == null {
		report("expected a number");
		return null;
	}

	return value;
}

// term is a run of factors joined by * and /, which bind tighter than + and -.
fn term() {
	let left = factor();
	if left == null {
		return null;
	}

	let running = true;

	while running {
		skipSpace();
		let op = at();

		if op == "*" || op == "/" {
			pos = pos + 1;

			let right = factor();
			if right == null {
				return null;
			}

			if op == "*" {
				left = left * right;
			} else {
				// The language makes division by zero an error that stops the
				// program, which is right for a program and wrong for a
				// calculator that has more lines to get through.
				if right == 0 {
					report("division by zero");
					return null;
				}
				left = left / right;
			}
		} else {
			running = false;
		}
	}

	return left;
}

// expression is a run of terms joined by + and -, which is the loosest level
// and therefore the one the whole line is read at.
fn expression() {
	let left = term();
	if left == null {
		return null;
	}

	let running = true;

	while running {
		skipSpace();
		let op = at();

		if op == "+" || op == "-" {
			pos = pos + 1;

			let right = term();
			if right == null {
				return null;
			}

			if op == "+" {
				left = left + right;
			} else {
				left = left - right;
			}
		} else {
			running = false;
		}
	}

	return left;
}

line = readLine();

while line != null {
	if len(line) > 0 {
		pos = 0;

		let value = expression();

		if value != null {
			skipSpace();

			if pos != len(line) {
				report("unexpected " + at());
			} else {
				print(str(value));
			}
		}
	}

	line = readLine();
}

if failures > 0 {
	exit(1);
}
