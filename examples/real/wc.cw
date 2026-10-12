// wc: count the lines, words and characters of standard input, the way the
// Unix tool does.
//
//     ccwig wc.cw < file
//
// The scan is one pass with no lookahead: a "word" is any run of characters
// that are not whitespace, so a word ends where the whitespace after it
// begins. That is the whole of the definition, and getting it right is what
// makes the count agree with the system's wc on ordinary text.
//
// One difference is deliberate: `len` counts characters, not bytes, so the
// last column matches wc's only while every character in the input is a
// single byte. The examples here are ASCII, which is why they can be compared
// against wc at all.

fn whitespace(ch) {
	return ch == " " || ch == "\t" || ch == "\n" || ch == "\r";
}

fn count(text) {
	let lines = 0;
	let words = 0;
	let inWord = false;

	for let i = 0; i < len(text); i = i + 1 {
		let ch = text[i];

		if ch == "\n" {
			lines = lines + 1;
		}

		if whitespace(ch) {
			inWord = false;
		} else {
			// Only the first character of a run starts a word.
			if !inWord {
				words = words + 1;
			}
			inWord = true;
		}
	}

	return [lines, words, len(text)];
}

let counts = count(readAll());
print(str(counts[0]) + " " + str(counts[1]) + " " + str(counts[2]));
