package ccwig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The to-do list is the program in this repository that is a tool rather than
// an example: it keeps state between runs, takes its orders from a command
// line, and is expected to work the second time it is run. So it is tested the
// way a tool is - run it, look at what it printed, look at what it wrote, and
// run it again.

// todo runs the application with the given arguments against a list in a
// fresh temporary directory, and returns what it printed, its exit status, and
// the contents of the list afterwards.
func todo(t *testing.T, listPath string, args ...string) (string, int, string) {
	t.Helper()

	out, status := runWith(t, filepath.Join("apps", "todo", "todo.cw"),
		append([]string{"-f", listPath}, args...), nil)

	contents := ""
	if data, err := os.ReadFile(listPath); err == nil {
		contents = string(data)
	}

	return out, status, contents
}

// listPath is a path to a list that does not exist yet, which is the state a
// tool is in the first time it is run.
func listPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "todo.txt")
}

func TestTodoStartsEmpty(t *testing.T) {
	out, status, contents := todo(t, listPath(t), "list")

	if status != 0 {
		t.Errorf("listing a list that does not exist exited %d, want 0:\n%s", status, out)
	}
	if !strings.Contains(out, "nothing to do") {
		t.Errorf("an empty list printed %q", out)
	}
	// Listing is a read. It must not create the file as a side effect.
	if contents != "" {
		t.Errorf("listing created the file with %q in it", contents)
	}
}

func TestTodoRemembersWhatWasAdded(t *testing.T) {
	path := listPath(t)

	out, status, contents := todo(t, path, "add", "buy", "milk")
	if status != 0 {
		t.Fatalf("add exited %d:\n%s", status, out)
	}

	// The words after `add` are one task, put back together with the spaces the
	// shell took out.
	if contents != "0|buy milk\n" {
		t.Errorf("the file holds %q", contents)
	}

	// A second run, with the file that the first one wrote.
	out, status, contents = todo(t, path, "add", "ship the report")
	if status != 0 {
		t.Fatalf("the second add exited %d:\n%s", status, out)
	}
	if contents != "0|buy milk\n0|ship the report\n" {
		t.Errorf("the file holds %q", contents)
	}

	out, status, _ = todo(t, path, "list")
	if status != 0 {
		t.Fatalf("list exited %d:\n%s", status, out)
	}

	for _, want := range []string{"1. [ ] buy milk", "2. [ ] ship the report", "2 task(s), 2 left"} {
		if !strings.Contains(out, want) {
			t.Errorf("the list does not show %q:\n%s", want, out)
		}
	}
}

func TestTodoMarksAndUnmarks(t *testing.T) {
	path := listPath(t)

	todo(t, path, "add", "one")
	todo(t, path, "add", "two")

	out, status, contents := todo(t, path, "done", "1")
	if status != 0 {
		t.Fatalf("done exited %d:\n%s", status, out)
	}
	if contents != "1|one\n0|two\n" {
		t.Errorf("after marking the first, the file holds %q", contents)
	}

	out, status, _ = todo(t, path, "list")
	if !strings.Contains(out, "1. [x] one") || !strings.Contains(out, "2. [ ] two") {
		t.Errorf("the list does not show what was marked:\n%s", out)
	}
	// What is left is what is not finished, and it is counted by the program
	// rather than by whoever is reading.
	if !strings.Contains(out, "2 task(s), 1 left") {
		t.Errorf("the count is wrong:\n%s", out)
	}

	_, status, contents = todo(t, path, "undone", "1")
	if status != 0 {
		t.Errorf("undone exited %d", status)
	}
	if contents != "0|one\n0|two\n" {
		t.Errorf("after unmarking, the file holds %q", contents)
	}
}

func TestTodoRefusesANumberThatIsNotThere(t *testing.T) {
	path := listPath(t)

	todo(t, path, "add", "only one")

	for _, args := range [][]string{
		{"done", "0"},
		{"done", "2"},
		{"done", "99"},
		{"remove", "2"},
		{"done", "not-a-number"},
	} {
		out, status, _ := todo(t, path, args...)

		if status != 1 {
			t.Errorf("todo %v exited %d, want 1:\n%s", args, status, out)
		}
		if !strings.Contains(out, "no task") {
			t.Errorf("todo %v did not say which task was missing:\n%s", args, out)
		}
	}
}

func TestTodoRemovesAndPrunes(t *testing.T) {
	path := listPath(t)

	todo(t, path, "add", "keep me")
	todo(t, path, "add", "remove me")
	todo(t, path, "add", "finish me")

	_, status, contents := todo(t, path, "remove", "2")
	if status != 0 {
		t.Fatalf("remove exited %d", status)
	}
	if contents != "0|keep me\n0|finish me\n" {
		t.Errorf("after removing the second, the file holds %q", contents)
	}

	todo(t, path, "done", "2")

	out, status, contents := todo(t, path, "prune")
	if status != 0 {
		t.Fatalf("prune exited %d:\n%s", status, out)
	}
	if contents != "0|keep me\n" {
		t.Errorf("after pruning, the file holds %q", contents)
	}
	if !strings.Contains(out, "pruned 1, 1 left") {
		t.Errorf("prune reported %q", out)
	}
}

func TestTodoRefusesACommandLineItCannotRead(t *testing.T) {
	path := listPath(t)

	for _, args := range [][]string{
		{"nope"},
		{"add"},
		{"done"},
		{"remove"},
		{"-f"}, // an option with no file after it
	} {
		out, status, _ := todo(t, path, args...)

		if status != 2 {
			t.Errorf("todo %v exited %d, want 2:\n%s", args, status, out)
		}
	}
}

func TestTodoKeepsATaskThatContainsTheSeparator(t *testing.T) {
	path := listPath(t)

	// The file is `1|text`, and a task is allowed to contain the separator.
	// This is what the file format has to survive, and it is why the separator
	// is looked for with `find` rather than split on.
	_, status, contents := todo(t, path, "add", "a | b | c")
	if status != 0 {
		t.Fatalf("add exited %d", status)
	}
	if contents != "0|a | b | c\n" {
		t.Errorf("the file holds %q", contents)
	}

	out, status, _ := todo(t, path, "list")
	if status != 0 {
		t.Fatalf("list exited %d", status)
	}
	if !strings.Contains(out, "1. [ ] a | b | c") {
		t.Errorf("the task came back as something else:\n%s", out)
	}
}

func TestTodoReadsAFileSomebodyElseWrote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todo.txt")

	// Line endings from another machine, a blank line, and a finished task.
	// The format is meant to be readable and writable by hand, so it has to
	// survive a file that was not written by this program.
	handwritten := "0|written by hand\r\n\r\n1|and finished\r\n"

	if err := os.WriteFile(path, []byte(handwritten), 0o644); err != nil {
		t.Fatal(err)
	}

	out, status, _ := todo(t, path, "list")

	if status != 0 {
		t.Fatalf("list exited %d:\n%s", status, out)
	}
	for _, want := range []string{
		"1. [ ] written by hand",
		"2. [x] and finished",
		"2 task(s), 1 left",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the list does not show %q:\n%s", want, out)
		}
	}
}

func TestTodoSaysWhatItCanDo(t *testing.T) {
	out, status, _ := todo(t, listPath(t), "help")

	if status != 0 {
		t.Fatalf("help exited %d", status)
	}

	// Every command it offers is one it actually has. A help text that lists a
	// command that does not exist is worse than no help text.
	for _, command := range []string{"list", "add", "done", "undone", "remove", "prune", "help"} {
		if !strings.Contains(out, command) {
			t.Errorf("the help text does not mention %q:\n%s", command, out)
		}
	}
}
