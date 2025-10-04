package ccwig_test

import (
	"bytes"
	"os"
	"path/filepath"
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
