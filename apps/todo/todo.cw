// todo: a to-do list that keeps itself in a file.
//
//     ccwig apps/todo/todo.cw add buy milk
//     ccwig apps/todo/todo.cw list
//     ccwig apps/todo/todo.cw done 1
//     ccwig apps/todo/todo.cw -f work.todo add ship the report
//
// Commands:
//
//     list             show every task, numbered
//     add <text...>    add a task, from the rest of the command line
//     done <n>         mark task n finished
//     undone <n>       mark task n unfinished
//     remove <n>       delete task n
//     prune            delete every finished task
//     help             the list above
//
// The status is the one a shell expects: 0 when the command did what it was
// asked, 1 when it could not (a task number that is not there), and 2 when the
// command line itself was wrong. A tool that always exits 0 cannot be used in
// a script, which is most of what makes something a tool.
//
// The file is one task per line, `1|text` for finished and `0|text` for not.
// The separator is looked for with `find` rather than with `split`, because a
// task is allowed to contain the separator and a split would cut it in two.
//
// This is the shape of program the language had not been used for: it reads,
// changes and rewrites a file of its own, takes its orders from a command
// line, and is expected to be run twice in a row and remember. Writing it
// turned up one thing the language was missing, `exists`, because `readFile`
// of a file that is not there is an error that stops the program, and a tool
// whose file is allowed to be missing has nothing to catch it with.

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

// The list, in two parallel arrays that are always the same length. The
// language has no structs, so a task is a question asked of both.
let doneFlags = [];
let texts = [];
let count = 0;

// load reads the file into the arrays. A file that is not there is an empty
// list, which is what the first run of a tool looks like.
fn load(path) {
	doneFlags = [];
	texts = [];
	count = 0;

	if exists(path) == false {
		return null;
	}

	let lines = split(readFile(path), "\n");

	// There is no `continue` in this language, so a line that is not a task is
	// one the body skips over rather than one the loop is told to skip.
	for let k = 0; k < len(lines); k = k + 1 {
		let line = lines[k];

		// A line ending that arrived from somewhere else, and the empty line a
		// trailing newline leaves behind, are both not tasks.
		if len(line) > 0 && line[len(line) - 1] == "\r" {
			line = substr(line, 0, len(line) - 1);
		}

		if len(line) > 0 {
			let bar = find(line, "|");

			if bar >= 1 {
				doneFlags = push(doneFlags, toInt(substr(line, 0, bar)));
				texts = push(texts, substr(line, bar + 1));
				count = count + 1;
			}
		}
	}

	return null;
}

// save writes the arrays back, one task per line.
fn save(path) {
	let out = "";

	for let i = 0; i < count; i = i + 1 {
		out = out + str(doneFlags[i]) + "|" + texts[i] + "\n";
	}

	writeFile(path, out);
	return null;
}

// indexOf turns the number a person typed into the index the arrays use, or -1
// if there is no such task. The list is numbered from one because that is what
// is printed, and a tool whose numbers do not match what it shows is a tool
// nobody trusts.
fn indexOf(text) {
	let n = toInt(text);

	if n < 1 || n > count {
		return -1;
	}

	return n - 1;
}

fn removeAt(i) {
	let newDone = [];
	let newTexts = [];

	for let j = 0; j < count; j = j + 1 {
		if j != i {
			newDone = push(newDone, doneFlags[j]);
			newTexts = push(newTexts, texts[j]);
		}
	}

	doneFlags = newDone;
	texts = newTexts;
	count = count - 1;

	return null;
}

fn unfinished() {
	let n = 0;

	for let i = 0; i < count; i = i + 1 {
		if doneFlags[i] == 0 {
			n = n + 1;
		}
	}

	return n;
}

fn usage() {
	print("usage: todo [-f file] <command> [arguments]");
	print("");
	print("  list             show every task, numbered");
	print("  add <text...>    add a task");
	print("  done <n>         mark task n finished");
	print("  undone <n>       mark task n unfinished");
	print("  remove <n>       delete task n");
	print("  prune            delete every finished task");
	print("  help             this");
	return null;
}

// list prints the tasks, then a line saying how many are left. A list that
// does not say what is left is a list a person has to count.
fn show() {
	if count == 0 {
		print("nothing to do");
		return null;
	}

	for let i = 0; i < count; i = i + 1 {
		let mark = "[ ]";

		if doneFlags[i] == 1 {
			mark = "[x]";
		}

		print(str(i + 1) + ". " + mark + " " + texts[i]);
	}

	print("");
	print(str(count) + " task(s), " + str(unfinished()) + " left");

	return null;
}

// --- the command line -------------------------------------------------------

let argv = args();

let path = "todo.txt";
let rest = [];

// Options first, so that `todo -f work.todo add x` and `todo add x` are the
// same command with a different file. The language has no flag library, and
// this is the whole of what one is for here.
let i = 0;
let readingOptions = true;

while i < len(argv) {
	if readingOptions && argv[i] == "-f" {
		if i + 1 >= len(argv) {
			print("no file after -f");
			exit(2);
		}
		path = argv[i + 1];
		i = i + 2;
	} else {
		readingOptions = false;
		rest = push(rest, argv[i]);
		i = i + 1;
	}
}

if len(rest) == 0 {
	show();
	exit(0);
}

let command = rest[0];
load(path);

if command == "list" || command == "ls" {
	show();
	exit(0);
}

if command == "add" {
	if len(rest) < 2 {
		print("add what?");
		exit(2);
	}

	// The rest of the command line is the task, joined back into one string.
	// There is no `join` for a slice of an array, so this is the loop that
	// puts the spaces back where the shell took them out.
	let text = "";

	for let j = 1; j < len(rest); j = j + 1 {
		if j > 1 {
			text = text + " ";
		}
		text = text + rest[j];
	}

	texts = push(texts, text);
	doneFlags = push(doneFlags, 0);
	count = count + 1;
	save(path);

	print("added, " + str(unfinished()) + " left");
	exit(0);
}

if command == "done" || command == "undone" {
	if len(rest) < 2 {
		print(command + " which task?");
		exit(2);
	}

	let at = indexOf(rest[1]);

	if at < 0 {
		print("no task " + rest[1] + " (there are " + str(count) + ")");
		exit(1);
	}

	if command == "done" {
		doneFlags[at] = 1;
	} else {
		doneFlags[at] = 0;
	}

	save(path);
	print(str(at + 1) + ". " + texts[at]);
	exit(0);
}

if command == "remove" || command == "rm" {
	if len(rest) < 2 {
		print("remove which task?");
		exit(2);
	}

	let at = indexOf(rest[1]);

	if at < 0 {
		print("no task " + rest[1] + " (there are " + str(count) + ")");
		exit(1);
	}

	let gone = texts[at];
	removeAt(at);
	save(path);

	print("removed: " + gone + ", " + str(count) + " left");
	exit(0);
}

if command == "prune" {
	let before = count;
	let kept = [];

	for let j = 0; j < count; j = j + 1 {
		if doneFlags[j] == 0 {
			kept = push(kept, j);
		}
	}

	let newDone = [];
	let newTexts = [];

	for let j = 0; j < len(kept); j = j + 1 {
		newDone = push(newDone, 0);
		newTexts = push(newTexts, texts[kept[j]]);
	}

	doneFlags = newDone;
	texts = newTexts;
	count = len(newTexts);
	save(path);

	print("pruned " + str(before - count) + ", " + str(count) + " left");
	exit(0);
}

if command == "help" {
	usage();
	exit(0);
}

print("no such command: " + command);
print("");
usage();
exit(2);
