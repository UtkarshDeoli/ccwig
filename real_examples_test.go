package ccwig_test

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// The examples under examples/real are checked against the programs they
// replace, not only against a recorded output file. A golden file records what
// a program printed; these tests ask whether what it printed is right, which is
// the one question a program cannot answer about itself.

// TestWCCountsWhatTheSystemCounts checks wc.cw against counting done here, in
// Go, which shares no code with the CCWIG program.
func TestWCCountsWhatTheSystemCounts(t *testing.T) {
	const dir = "examples/real/"

	input, err := os.ReadFile(dir + "wc.in")
	if err != nil {
		t.Fatalf("could not read the input: %v", err)
	}
	text := string(input)

	// A line is what wc counts: a newline, not a run of text.
	lines := strings.Count(text, "\n")
	// Fields splits on runs of whitespace and drops the empty pieces, which is
	// the definition of a word this example is written to.
	words := len(strings.Fields(text))
	chars := utf8.RuneCountInString(text)

	want := fmt.Sprintf("%d %d %d\n", lines, words, chars)
	if got := runFile(t, dir+"wc.cw"); got != want {
		t.Errorf("wc.cw printed %q, but counting the same input here gives %q", got, want)
	}
}

// TestGrepFindsWhatGoFinds checks grep.cw against a filter written here: the
// same lines, numbered the same way, found by different code.
func TestGrepFindsWhatGoFinds(t *testing.T) {
	const dir = "examples/real/"

	args := readLines(t, dir+"grep.args")
	if len(args) != 1 {
		t.Fatalf("grep.args holds %d arguments, but this test knows how to check one pattern", len(args))
	}
	pattern := args[0]

	input, err := os.ReadFile(dir + "grep.in")
	if err != nil {
		t.Fatalf("could not read the input: %v", err)
	}

	var want strings.Builder
	for i, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.Contains(line, pattern) {
			fmt.Fprintf(&want, "%d:%s\n", i+1, line)
		}
	}

	if got := runFile(t, dir+"grep.cw"); got != want.String() {
		t.Errorf("grep.cw printed\n%s\nbut filtering the same input here gives\n%s", got, want.String())
	}
}

// TestJSONIsReindentedAndNothingElse checks the JSON example with a real JSON
// parser: the document that comes out has to be the same document that went
// in, whatever indentation it was given.
func TestJSONIsReindentedAndNothingElse(t *testing.T) {
	const dir = "examples/real/"

	compact, err := os.ReadFile(dir + "json.in")
	if err != nil {
		t.Fatalf("could not read the input: %v", err)
	}

	var before any
	if err := json.Unmarshal(compact, &before); err != nil {
		t.Fatalf("json.in is not valid JSON, so there is nothing to reindent: %v", err)
	}

	pretty := runFile(t, dir+"json.cw")

	var after any
	if err := json.Unmarshal([]byte(pretty), &after); err != nil {
		t.Fatalf("json.cw did not produce valid JSON:\n%s\nerror: %v", pretty, err)
	}

	if !reflect.DeepEqual(before, after) {
		t.Errorf("json.cw changed the document:\n--- before ---\n%#v\n--- after ---\n%#v", before, after)
	}

	// A formatter that printed the input unchanged would pass the check above.
	// The output has to be indented, or the program did nothing.
	if !strings.Contains(pretty, "\n  \"name\"") {
		t.Errorf("json.cw did not indent its output:\n%s", pretty)
	}
}

// TestExprReportsBadInputAndKeepsGoing checks the calculator's two promises
// about a line it cannot read: it says where the line went wrong, and it
// carries on to the next one. A program that stopped at the first bad line
// would be useless on a file of expressions, which is what a calculator is
// for.
func TestExprReportsBadInputAndKeepsGoing(t *testing.T) {
	const src = `examples/real/expr.cw`

	tests := []struct {
		input string
		want  string
	}{
		{"1 + 2\n3 / 0\n", "3\nexpr: division by zero"},
		{"(1 + 2\n", "expected a closing bracket"},
		{"4 * \n", "expected a number"},
		{"2 3\n", "unexpected 3"},
	}

	for _, tt := range tests {
		t.Run(strings.TrimSpace(tt.input), func(t *testing.T) {
			got, status := runWithInput(t, src, tt.input)

			if !strings.Contains(got, tt.want) {
				t.Errorf("input %q printed %q, expected it to mention %q", tt.input, got, tt.want)
			}

			// A file with a line that will not parse must not exit zero, or a
			// script using it cannot tell that anything was wrong.
			if status == 0 {
				t.Errorf("input %q exited zero despite an unreadable line", tt.input)
			}
		})
	}
}

// TestExprLeavesBlankLinesAlone is the other half of reading a file of
// expressions: a blank line is not a line that went wrong, and the last
// expression in a file that ends with a newline still has to be evaluated.
func TestExprLeavesBlankLinesAlone(t *testing.T) {
	got, status := runWithInput(t, "examples/real/expr.cw", "\n\n7\n\n")

	if status != 0 {
		t.Errorf("a file of blank lines and one expression exited with status %d (output %q)", status, got)
	}
	if got != "7\n" {
		t.Errorf("expected just the one answer, got %q", got)
	}
}

// TestGrepUsesTheStatusesGrepUses checks that a program can be used in a shell
// condition: no lines found is a failure to find, and being asked wrongly is a
// different thing again.
func TestGrepUsesTheStatusesGrepUses(t *testing.T) {
	const src = "examples/real/grep.cw"

	tests := []struct {
		name   string
		args   []string
		input  string
		status int
	}{
		{"a line is found", []string{"fox"}, "a fox\n", 0},
		{"no line is found", []string{"unicorn"}, "a fox\n", 1},
		{"asked wrongly", nil, "a fox\n", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, status := runWithArgsAndInput(t, src, tt.args, tt.input)

			if status != tt.status {
				t.Errorf("expected exit status %d, got %d (output %q)", tt.status, status, got)
			}
		})
	}
}

// TestHeadDefaultsToTen is the one thing about head worth pinning that the
// recorded output cannot show: it was run with an argument, so the default is
// never exercised.
func TestHeadDefaultsToTen(t *testing.T) {
	const src = "examples/real/head.cw"

	numbers := make([]string, 0, 25)
	for i := 1; i <= 25; i++ {
		numbers = append(numbers, strconv.Itoa(i))
	}
	input := strings.Join(numbers, "\n") + "\n"

	got, status := runWithInput(t, src, input)
	if status != 0 {
		t.Fatalf("head.cw exited with status %d", status)
	}

	want := strings.Join(numbers[:10], "\n") + "\n"
	if got != want {
		t.Errorf("head.cw with no argument printed\n%s\nexpected the first ten lines:\n%s", got, want)
	}
}
