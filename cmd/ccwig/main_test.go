package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// exec runs the CLI with the given arguments and returns its exit code,
// standard output and standard error.
func exec(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr)

	return code, stdout.String(), stderr.String()
}

func TestEvalFlagRunsSource(t *testing.T) {
	code, stdout, stderr := exec(t, "", "-e", `print("hello");`)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if stdout != "hello\n" {
		t.Errorf("expected \"hello\\n\", got %q", stdout)
	}
}

func TestEvalFlagReportsRuntimeErrors(t *testing.T) {
	code, _, stderr := exec(t, "", "-e", "1 + true;")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "type mismatch: INTEGER + BOOLEAN") {
		t.Errorf("expected the error on stderr, got %q", stderr)
	}
}

func TestEvalFlagReportsSyntaxErrors(t *testing.T) {
	code, _, stderr := exec(t, "", "-e", "let = 5;")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "syntax error") {
		t.Errorf("expected a syntax error message, got %q", stderr)
	}
}

func TestEmptySourceIsSuccess(t *testing.T) {
	code, stdout, stderr := exec(t, "", "-e", "")
	if code != 0 || stdout != "" || stderr != "" {
		t.Errorf("expected a silent success, got code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestRunsAFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.cw")

	src := "let name = \"world\";\nprint(\"hello \" + name);\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("could not write the test file: %v", err)
	}

	code, stdout, stderr := exec(t, "", path)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if stdout != "hello world\n" {
		t.Errorf("expected \"hello world\\n\", got %q", stdout)
	}
}

func TestFileErrorsNameTheFileAndLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.cw")

	if err := os.WriteFile(path, []byte("let a = 1;\n5 + true;\n"), 0o644); err != nil {
		t.Fatalf("could not write the test file: %v", err)
	}

	code, _, stderr := exec(t, "", path)
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "broken.cw:2:") {
		t.Errorf("expected the file and line in the message, got %q", stderr)
	}
}

func TestMissingFileIsReported(t *testing.T) {
	code, _, stderr := exec(t, "", filepath.Join(t.TempDir(), "nope.cw"))

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "nope.cw") {
		t.Errorf("expected the filename in the message, got %q", stderr)
	}
}

func TestNoArgsRunsTheREPL(t *testing.T) {
	code, stdout, stderr := exec(t, "let x = 2;\nx * 21;\nquit\n")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "42") {
		t.Errorf("expected the REPL to print 42, got %q", stdout)
	}
}

func TestREPLPromptIsShown(t *testing.T) {
	_, stdout, _ := exec(t, "1 + 1;\nexit\n")
	if !strings.Contains(stdout, ">> ") {
		t.Errorf("expected a prompt, got %q", stdout)
	}
}

func TestVersionFlag(t *testing.T) {
	code, stdout, _ := exec(t, "", "-version")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, Version) {
		t.Errorf("expected the version %q in the output, got %q", Version, stdout)
	}
}

func TestHelpFlag(t *testing.T) {
	code, stdout, _ := exec(t, "", "-h")

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if !strings.Contains(stdout, "Usage") {
		t.Errorf("expected usage information, got %q", stdout)
	}
}

func TestUnknownFlagIsAUsageError(t *testing.T) {
	code, _, stderr := exec(t, "", "-bogus")

	if code != 2 {
		t.Errorf("expected exit code 2 for a usage error, got %d", code)
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("expected the bad flag to be named, got %q", stderr)
	}
}

func TestBothASourceAndAFileIsAUsageError(t *testing.T) {
	code, _, stderr := exec(t, "", "-e", "print(1);", "file.cw")

	if code != 2 {
		t.Errorf("expected exit code 2, got %d", code)
	}
	if !strings.Contains(stderr, "not both") {
		t.Errorf("expected an explanatory message, got %q", stderr)
	}
}

func TestTokensFlagDumpsTheTokenStream(t *testing.T) {
	code, stdout, stderr := exec(t, "", "-tokens", "-e", "let x = 1;")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	for _, want := range []string{`LET("let")`, `IDENT("x")`, `ASSIGN("=")`, `INT("1")`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("expected %s in the token dump, got:\n%s", want, stdout)
		}
	}
}

func TestTokensFlagReportsLexicalErrors(t *testing.T) {
	code, _, stderr := exec(t, "", "-tokens", "-e", "a @ b")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "unexpected character") {
		t.Errorf("expected a lexical error, got %q", stderr)
	}
}

func TestASTFlagPrintsTheTree(t *testing.T) {
	code, stdout, stderr := exec(t, "", "-ast", "-e", "1 + 2 * 3;")

	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "(1 + (2 * 3))") {
		t.Errorf("expected the grouped expression, got %q", stdout)
	}
}

func TestASTFlagReportsSyntaxErrors(t *testing.T) {
	code, _, stderr := exec(t, "", "-ast", "-e", "let = 1;")

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr, "syntax error") {
		t.Errorf("expected a syntax error, got %q", stderr)
	}
}

func TestFileArgumentRunsTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "args.cw")

	if err := os.WriteFile(path, []byte("print(1 + 2);\n"), 0o644); err != nil {
		t.Fatalf("could not write the test file: %v", err)
	}

	code, stdout, stderr := exec(t, "", "-tokens", path)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, `IDENT("print")`) {
		t.Errorf("expected a token dump, got %q", stdout)
	}
}
