// head: print the first N lines of standard input, ten by default.
//
//     ccwig head.cw 3 < file
//
// The count arrives as text, because everything on a command line does, so
// this program has to turn it into a number itself and decide what to do
// about something that is not one. Refusing it out loud is better than
// silently treating "three" as zero lines and printing nothing.

let argv = args();
let want = 10;

if len(argv) > 0 {
	want = 0;

	for let i = 0; i < len(argv[0]); i = i + 1 {
		let ch = argv[0][i];

		if ch < "0" || ch > "9" {
			print("head: not a number: " + argv[0]);
			exit(1);
		}

		// Digits are contiguous in the code point order, so a digit's value is
		// its distance from "0".
		want = want * 10 + (ord(ch) - ord("0"));
	}
}

let line = readLine();
let printed = 0;

while line != null && printed < want {
	print(line);
	printed = printed + 1;
	line = readLine();
}
