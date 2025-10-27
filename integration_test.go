package ccwig_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/interp"
)

// runFile executes an example program and returns its output.
func runFile(t *testing.T, path string) string {
	t.Helper()

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	var out bytes.Buffer
	if err := interp.New(&out).Run(string(src), path); err != nil {
		t.Fatalf("%s failed: %v", path, err)
	}

	return out.String()
}

// TestEveryExampleHasExpectedOutput keeps the example programs and their
// expected output in step: adding a program without recording what it prints
// fails here rather than silently going untested.
func TestEveryExampleHasExpectedOutput(t *testing.T) {
	sources, err := filepath.Glob("examples/*.cw")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no example programs found; expected files under examples/")
	}

	for _, src := range sources {
		want := strings.TrimSuffix(src, ".cw") + ".out"
		if _, err := os.Stat(want); err != nil {
			t.Errorf("%s has no expected-output file %s", src, want)
		}
	}

	expected, err := filepath.Glob("examples/*.out")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}

	for _, want := range expected {
		src := strings.TrimSuffix(want, ".out") + ".cw"
		if _, err := os.Stat(src); err != nil {
			t.Errorf("%s has no program file %s", want, src)
		}
	}
}

// TestExamples runs every example and compares its output with the recorded
// expected output.
func TestExamples(t *testing.T) {
	sources, err := filepath.Glob("examples/*.cw")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}

	for _, src := range sources {
		name := strings.TrimSuffix(filepath.Base(src), ".cw")

		t.Run(name, func(t *testing.T) {
			wantPath := strings.TrimSuffix(src, ".cw") + ".out"

			wantBytes, err := os.ReadFile(wantPath)
			if err != nil {
				t.Fatalf("could not read %s: %v", wantPath, err)
			}

			got := runFile(t, src)
			want := string(wantBytes)

			if got != want {
				t.Errorf("output mismatch for %s\n--- want ---\n%s\n--- got ---\n%s", src, want, got)
			}
		})
	}
}

// TestErrorPrograms checks that deliberately broken programs produce the
// errors they are supposed to. These are the examples of what goes wrong and
// why, so they double as a test of the error messages.
func TestErrorPrograms(t *testing.T) {
	tests := []struct {
		file string
		want string
	}{
		{"undefined-variable.cw", "identifier not found: width"},
		{"type-mismatch.cw", "type mismatch: INTEGER + STRING"},
		{"bad-index.cw", "index must be an integer, got STRING"},
		{"wrong-arity.cw", "wrong number of arguments: want 2, got 1"},
		{"division-by-zero.cw", "division by zero"},
		{"not-a-function.cw", "not a function: INTEGER"},
		{"syntax-error.cw", "expected an expression"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			path := filepath.Join("examples", "errors", tt.file)

			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("could not read %s: %v", path, err)
			}

			var out bytes.Buffer
			err = interp.New(&out).Run(string(src), path)
			if err == nil {
				t.Fatalf("%s was expected to fail, but it ran successfully", path)
			}

			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("%s: expected the error to mention %q, got %q", path, tt.want, err.Error())
			}

			// The message must name the file and the line.
			if !strings.Contains(err.Error(), path+":") {
				t.Errorf("%s: expected the error to name the file and line, got %q", path, err.Error())
			}
		})
	}
}

// clearSequence is what `clear()` writes: erase the screen, then home the
// cursor. It is spelled out here rather than borrowed from the interpreter so
// that the animation test checks the sequence the language documents, and would
// notice if the builtin started writing something else.
const clearSequence = "\x1b[2J\x1b[H"

// TestAnimatedExample runs examples/animate/maze.cw and checks it really does
// animate.
//
// That program pauses between frames, which is the whole point of it and also
// what would make a test of it take eight seconds. FRAME_MS drives every pause
// in it, so the test rewrites that one line to zero and the animation then runs
// as fast as it can be drawn. If the constant ever disappears the test fails,
// rather than quietly going back to taking eight seconds.
func TestAnimatedExample(t *testing.T) {
	const path = "examples/animate/maze.cw"

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	frameMS := regexp.MustCompile(`(?m)^let FRAME_MS = \d+;$`)
	if !frameMS.Match(src) {
		t.Fatalf("%s no longer declares `let FRAME_MS = <number>;`, so this test cannot switch its pacing off", path)
	}
	instant := frameMS.ReplaceAll(src, []byte("let FRAME_MS = 0;"))

	run := func() string {
		t.Helper()

		var out bytes.Buffer
		if err := interp.New(&out).Run(string(instant), path); err != nil {
			t.Fatalf("%s failed: %v", path, err)
		}
		return out.String()
	}

	got := run()
	frames := animationFrames(got)

	// Drawing one frame and stopping is what the non-animated example does, so
	// too few frames would mean this program had stopped doing its own job.
	if len(frames) < 100 {
		t.Errorf("expected at least 100 frames from %s, got %d", path, len(frames))
	}

	// Every frame but the last is redrawn where the previous one was, so a frame
	// taller than the screen would scroll and leave the animation smeared down
	// the terminal instead of playing in place.
	for i, frame := range frames {
		if h := frameHeight(frame); h > 24 {
			t.Errorf("frame %d of %s is %d lines tall, which does not fit an ordinary terminal", i, path, h)
		}
	}

	// The show is meant to run through its phases in order, so each one has to
	// appear, and later than the one before it.
	phases := []string{
		"carving the maze",
		"breadth first search",
		"breadth first: retracing",
		"depth first search",
		"depth first: retracing",
	}
	next := 0
	for _, frame := range frames {
		if next < len(phases) && strings.Contains(frame, phases[next]) {
			next++
		}
	}
	if next < len(phases) {
		t.Errorf("%s never reached its %q phase", path, phases[next])
	}

	// The program checks both routes itself as it finishes; a route drawn on a
	// maze looks convincing whether or not it is a real one.
	last := frames[len(frames)-1]
	if n := strings.Count(last, "ok - every step is between neighbouring cells"); n != 2 {
		t.Errorf("%s reported %d of its 2 routes as checked in the final frame:\n%s", path, n, last)
	}

	// A fixed seed and a generator that carries its own state, so the same show
	// every time.
	if second := run(); second != got {
		t.Errorf("%s is not deterministic: two runs produced different output", path)
	}
}

// animationFrames splits animated output into the frames drawn between one
// screen clear and the next.
func animationFrames(out string) []string {
	var frames []string
	for _, frame := range strings.Split(out, clearSequence) {
		if strings.TrimSpace(frame) != "" {
			frames = append(frames, frame)
		}
	}
	return frames
}

// frameHeight counts the lines a frame occupies on screen, ignoring the newline
// that ends the last one.
func frameHeight(frame string) int {
	return strings.Count(strings.TrimRight(frame, "\n"), "\n") + 1
}

// TestHugeExample runs examples/huge/maze.cw - the maze grown until it is no
// longer something to trace with a finger - and checks the claims it makes
// about itself.
//
// It deliberately has no recorded output. SIZE in that program is the one
// number it invites the reader to change, and a checked-in .out would turn
// every change to it into a test failure. What is checked instead is the
// structure, which has to hold at every size the interpreter will run at all.
func TestHugeExample(t *testing.T) {
	const path = "examples/huge/maze.cw"

	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not read %s: %v", path, err)
	}

	// The grid is carved two cells at a time from an odd-sized lattice, so an
	// even SIZE would leave the last row and column unreachable.
	declared := regexp.MustCompile(`(?m)^let SIZE = (\d+);`).FindSubmatch(src)
	if declared == nil {
		t.Fatalf("%s no longer declares `let SIZE = <number>;`, so this test cannot tell how big the maze is meant to be", path)
	}
	size, err := strconv.Atoi(string(declared[1]))
	if err != nil {
		t.Fatalf("%s declares a SIZE of %q, which is not a number", path, declared[1])
	}
	if size%2 == 0 {
		t.Errorf("%s uses SIZE = %d, but the maze needs an odd size", path, size)
	}

	got := runFile(t, path)
	lines := strings.Split(got, "\n")

	// Three pictures: the maze itself, then the maze with each route drawn on
	// it. Each one must be exactly the size of the maze, or the grid being
	// drawn is not the grid being searched.
	for _, marker := range []string{"--- the maze ---", "--- breadth first ---", "--- depth first ---"} {
		picture := pictureAfter(t, lines, marker, size)
		for i, row := range picture {
			if len(row) != size {
				t.Errorf("%s: row %d of the picture after %q is %d characters wide, expected %d", path, i, marker, len(row), size)
				break
			}
		}

		// The outer wall is never opened, so every drawn row begins and ends
		// with one - and the start and the exit each appear exactly once.
		for i, row := range picture {
			if !strings.HasPrefix(row, "#") || !strings.HasSuffix(row, "#") {
				t.Errorf("%s: row %d of the picture after %q is not walled in: %q", path, i, marker, row)
				break
			}
		}
		if n := strings.Count(strings.Join(picture, ""), "S"); n != 1 {
			t.Errorf("%s: the picture after %q has %d starts, expected 1", path, marker, n)
		}
		if n := strings.Count(strings.Join(picture, ""), "E"); n != 1 {
			t.Errorf("%s: the picture after %q has %d exits, expected 1", path, marker, n)
		}
	}

	// The carver's central claim, which nobody can check by eye at this size and
	// which the program therefore counts instead: every cell reachable and
	// joined by exactly one passage fewer than there are cells, which is what
	// makes a maze a maze. The line below only confirms the program still makes
	// that claim; confirming the claim is true is the oracle's job, since a
	// program's own report of its own work is the one thing that cannot be
	// trusted to be right.
	//
	// What can be measured from the page is that the report agrees with the
	// picture. The maze is drawn after the braiding, so the open positions on
	// it are the passages of the tree plus the loops knocked through afterwards,
	// and the program prints how many loops those were.
	if !strings.Contains(got, "a tree, as a maze must be") {
		t.Errorf("%s no longer reports its carving as a tree", path)
	}
	if strings.Contains(got, "FAILED") {
		t.Errorf("%s reported a FAILED check in its own output", path)
	}

	carved := pictureAfter(t, lines, "--- the maze ---", size)
	open, cells := openAndCells(carved)
	if cells != ((size-1)/2)*((size-1)/2) {
		t.Errorf("%s: the drawn maze has %d cells open, but %d fit inside its walls", path, cells, ((size-1)/2)*((size-1)/2))
	}
	if loops := loopsReported(t, got); open-cells != (cells-1)+loops {
		t.Errorf("%s: the drawn maze has %d passages, but a tree on %d cells plus the %d loops it reports needs %d",
			path, open-cells, cells, loops, (cells-1)+loops)
	}

	// Both routes must pass the program's own check. A route drawn on a maze
	// looks convincing whether or not it is real, and at this size nobody is
	// going to trace it.
	if n := strings.Count(got, "check: ok - every step is between neighbouring cells"); n != 2 {
		t.Errorf("%s reported %d of its 2 routes as checked, expected 2", path, n)
	}

	// Breadth first promises the shortest route; depth first promises only
	// some route. So whatever the maze looks like, BFS's route can never be
	// the longer of the two.
	bfs := routeSteps(t, got, "--- breadth first ---")
	dfs := routeSteps(t, got, "--- depth first ---")
	if bfs > dfs {
		t.Errorf("%s: breadth first found a route of %d steps but depth first one of %d; breadth first searches by distance and cannot be longer", path, bfs, dfs)
	}

	// A fixed seed, so the same maze every run.
	if second := runFile(t, path); second != got {
		t.Errorf("%s is not deterministic: two runs produced different output", path)
	}
}

// pictureAfter returns the rows of the picture that follows a section marker.
// A picture is the run of lines that begins at the marker and is the height of
// the maze, so a short one means the program printed less than it claimed.
func pictureAfter(t *testing.T, lines []string, marker string, size int) []string {
	t.Helper()

	at := -1
	for i, line := range lines {
		if line == marker {
			at = i + 1
			break
		}
	}
	if at < 0 {
		t.Fatalf("the output has no %q section", marker)
	}

	// The breadth first and depth first sections report their statistics
	// before they draw, so skip to the first walled row.
	for at < len(lines) && !strings.HasPrefix(lines[at], "#") {
		at++
	}
	if at+size > len(lines) {
		t.Fatalf("the picture after %q is shorter than the %d rows it should be", marker, size)
	}

	return lines[at : at+size]
}

// routeSteps reads the number of steps out of a "route: N steps" line in the
// given section.
func routeSteps(t *testing.T, out, marker string) int {
	t.Helper()

	at := strings.Index(out, marker)
	if at < 0 {
		t.Fatalf("the output has no %q section", marker)
	}

	m := regexp.MustCompile(`route: ([\d,]+) steps`).FindStringSubmatch(out[at:])
	if m == nil {
		t.Fatalf("the %q section reports no route length", marker)
	}
	return atoi(t, m[1])
}

// loopsReported reads the number of loops the braiding knocked through.
func loopsReported(t *testing.T, out string) int {
	t.Helper()

	m := regexp.MustCompile(`braided: ([\d,]+) further walls`).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the output does not report how many walls the braiding knocked out")
	}
	return atoi(t, m[1])
}

// openAndCells counts, on a drawn maze, the positions a search can stand on and
// the cells among them.
//
// Cells sit on odd rows and odd columns; a wall between two cells is never on an
// odd row and an odd column at once, so the two counts come apart cleanly. The
// start and the exit are drawn as their own characters, and both are cells.
func openAndCells(picture []string) (open, cells int) {
	for r, row := range picture {
		for c, ch := range row {
			if ch == '#' {
				continue
			}
			open++
			if r%2 == 1 && c%2 == 1 {
				cells++
			}
		}
	}
	return open, cells
}

// atoi reads a number out of the output, which is written with thousands
// separators.
func atoi(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(strings.ReplaceAll(s, ",", ""))
	if err != nil {
		t.Fatalf("%q is not a number", s)
	}
	return n
}
