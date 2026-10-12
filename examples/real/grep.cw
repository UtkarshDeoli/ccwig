// grep: print the lines of standard input that contain a pattern.
//
//     ccwig grep.cw fox < file
//
// The pattern comes from the command line rather than being written into the
// program, which is the difference between a tool and an example: the same
// file answers a different question depending on what it is pointed at.
//
// Matching is a plain substring search. That is all grep did before regular
// expressions arrived, and it is still what most uses of grep need.
//
// The exit status is the one grep uses: 0 for lines found, 1 for lines not
// found, 2 for being asked wrongly. A shell can tell the second from the
// third, which is what makes `if grep pattern < file; then ...` work.

let argv = args();
if len(argv) == 0 {
	print("usage: grep <pattern>");
	exit(2);
}

let pattern = argv[0];
let line = readLine();
let number = 0;
let found = 0;

while line != null {
	number = number + 1;

	if find(line, pattern) >= 0 {
		print(str(number) + ":" + line);
		found = found + 1;
	}

	line = readLine();
}

if found == 0 {
	exit(1);
}
