package interp

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func run(t *testing.T, src string) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	err := New(&buf).Run(src, "test.cw")

	return buf.String(), err
}

func mustRun(t *testing.T, src string) string {
	t.Helper()

	out, err := run(t, src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return out
}

func TestRunAProgram(t *testing.T) {
	out := mustRun(t, `
let greeting = "hello";
print(greeting + " world");
fn add(a, b) { return a + b; }
print(add(2, 3));
`)

	want := "hello world\n5\n"
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}

func TestRunEmptyProgram(t *testing.T) {
	out := mustRun(t, "")
	if out != "" {
		t.Errorf("expected no output, got %q", out)
	}

	out = mustRun(t, "// just a comment\n")
	if out != "" {
		t.Errorf("expected no output, got %q", out)
	}
}

func TestRunAPureExpressionProducesNoOutput(t *testing.T) {
	out := mustRun(t, "1 + 2;")
	if out != "" {
		t.Errorf("expected no output from a bare expression, got %q", out)
	}
}

func TestSyntaxErrorReportsEveryProblem(t *testing.T) {
	_, err := run(t, "let = 5;\nlet y = ;\n")
	if err == nil {
		t.Fatal("expected a syntax error")
	}

	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected *SyntaxError, got %T (%v)", err, err)
	}

	if len(syntaxErr.Problems) != 2 {
		t.Errorf("expected 2 problems, got %d: %v", len(syntaxErr.Problems), syntaxErr.Problems)
	}

	// Both problems should be reported, and the message should name the file.
	msg := err.Error()
	if !strings.Contains(msg, "test.cw") {
		t.Errorf("expected the filename in the message, got %q", msg)
	}
	if !strings.Contains(msg, "1:") || !strings.Contains(msg, "2:") {
		t.Errorf("expected errors on lines 1 and 2, got %q", msg)
	}
}

func TestLexicalErrorIsReportedAsASyntaxError(t *testing.T) {
	_, err := run(t, `print("unterminated`)
	if err == nil {
		t.Fatal("expected an error for an unterminated string")
	}

	var syntaxErr *SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Fatalf("expected *SyntaxError, got %T (%v)", err, err)
	}
	if !strings.Contains(err.Error(), "unterminated") {
		t.Errorf("expected the message to mention the unterminated string, got %q", err.Error())
	}
}

func TestRuntimeErrorCarriesFileLineAndColumn(t *testing.T) {
	out, err := run(t, "let a = 1;\nprint(\"before\");\n5 + true;\nprint(\"after\");\n")
	if err == nil {
		t.Fatal("expected a runtime error")
	}

	// Output written before the failure is still delivered.
	if out != "before\n" {
		t.Errorf("expected output before the error, got %q", out)
	}

	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected *RuntimeError, got %T (%v)", err, err)
	}

	if runtimeErr.Pos.Line != 3 {
		t.Errorf("expected the error on line 3, got %s", runtimeErr.Pos)
	}
	if runtimeErr.Message != "type mismatch: INTEGER + BOOLEAN" {
		t.Errorf("unexpected message: %q", runtimeErr.Message)
	}

	msg := err.Error()
	if !strings.Contains(msg, "test.cw:3:") {
		t.Errorf("expected the message to start with test.cw:3:, got %q", msg)
	}
}

func TestRuntimeErrorFromABuiltinHasAPosition(t *testing.T) {
	_, err := run(t, "print(\"ok\");\nlen(1);\n")
	if err == nil {
		t.Fatal("expected a runtime error")
	}

	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) {
		t.Fatalf("expected *RuntimeError, got %T (%v)", err, err)
	}
	if runtimeErr.Pos.Line != 2 {
		t.Errorf("expected the builtin error on line 2, got %s", runtimeErr.Pos)
	}
}

func TestEvalReturnsTheValueOfTheLastStatement(t *testing.T) {
	in := New(new(bytes.Buffer))

	obj, err := in.Eval("let x = 5;\nx * 2;", "test.cw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj == nil {
		t.Fatal("expected a value")
	}
	if got := obj.Inspect(); got != "10" {
		t.Errorf("expected 10, got %q", got)
	}
}

func TestEnvironmentPersistsAcrossEvalCalls(t *testing.T) {
	in := New(new(bytes.Buffer))

	if _, err := in.Eval("let x = 41;", "test.cw"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := in.Eval("x + 1;", "test.cw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := obj.Inspect(); got != "42" {
		t.Errorf("expected 42, got %q", got)
	}
}

// --------------------------------------------------------------------- REPL

func repl(t *testing.T, input string) string {
	t.Helper()

	var out bytes.Buffer
	in := New(&out)
	in.Prompt = "" // keep the expected output free of prompts

	if err := in.RunREPL(strings.NewReader(input)); err != nil {
		t.Fatalf("REPL returned an error: %v", err)
	}

	return out.String()
}

func TestREPLPrintsExpressionResults(t *testing.T) {
	got := repl(t, "1 + 2;\n")
	if got != "3\n" {
		t.Errorf("expected \"3\\n\", got %q", got)
	}
}

func TestREPLIsSilentForBindings(t *testing.T) {
	got := repl(t, "let x = 5;\n")
	if got != "" {
		t.Errorf("expected no output for a let, got %q", got)
	}
}

func TestREPLKeepsStateBetweenLines(t *testing.T) {
	got := repl(t, "let x = 5;\nx * 2;\nfn double(n) { n * 2; }\ndouble(x);\n")
	if got != "10\n10\n" {
		t.Errorf("expected \"10\\n10\\n\", got %q", got)
	}
}

func TestREPLSurvivesErrors(t *testing.T) {
	got := repl(t, "1 + true;\n1 + 2;\n")

	if !strings.Contains(got, "type mismatch: INTEGER + BOOLEAN") {
		t.Errorf("expected the error to be reported, got %q", got)
	}
	if !strings.Contains(got, "3") {
		t.Errorf("expected the REPL to keep going and print 3, got %q", got)
	}
}

func TestREPLSurvivesSyntaxErrors(t *testing.T) {
	got := repl(t, "let = 5;\n1 + 2;\n")

	if !strings.Contains(got, "expected") {
		t.Errorf("expected a syntax error to be reported, got %q", got)
	}
	if !strings.Contains(got, "3") {
		t.Errorf("expected the REPL to keep going, got %q", got)
	}
}

func TestREPLIgnoresBlankLines(t *testing.T) {
	got := repl(t, "\n\n   \n1 + 1;\n")
	if got != "2\n" {
		t.Errorf("expected \"2\\n\", got %q", got)
	}
}

func TestREPLExitsOnQuitAndExit(t *testing.T) {
	for _, cmd := range []string{"quit", "exit"} {
		got := repl(t, cmd+"\n1 + 1;\n")
		if got != "" {
			t.Errorf("%q: expected the REPL to stop, got %q", cmd, got)
		}
	}
}

func TestREPLReadsMultiLineInput(t *testing.T) {
	got := repl(t, "fn add(a, b) {\n  return a + b;\n}\nadd(1, 2);\n")
	if got != "3\n" {
		t.Errorf("expected \"3\\n\", got %q", got)
	}
}

func TestREPLReportsErrorsMultiLineInputFailsToClose(t *testing.T) {
	// An unclosed brace at end of input must not hang; it is reported.
	got := repl(t, "if true {\nprint(1);\n")
	if !strings.Contains(got, "never closed") && !strings.Contains(got, "expected") {
		t.Errorf("expected an unclosed-block error, got %q", got)
	}
}

func TestREPLShowsFunctions(t *testing.T) {
	got := repl(t, "fn add(a, b) { return a + b; }\nadd\n")
	if !strings.HasPrefix(got, "fn(a, b)") {
		t.Errorf("expected the function to be echoed, got %q", got)
	}
}

func TestREPLWritesAPrompt(t *testing.T) {
	var out bytes.Buffer
	in := New(&out)
	in.Prompt = ">> "

	if err := in.RunREPL(strings.NewReader("1 + 1;\n")); err != nil {
		t.Fatalf("REPL returned an error: %v", err)
	}

	if !strings.Contains(out.String(), ">> ") {
		t.Errorf("expected a prompt, got %q", out.String())
	}
}

func TestREPLPrintsNullishResults(t *testing.T) {
	// A statement whose value is null prints nothing: `print` already
	// returned its own value.
	got := repl(t, "print(1);\n")
	if got != "1\n" {
		t.Errorf("expected just \"1\\n\", got %q", got)
	}
}

func TestREPLClosuresPersist(t *testing.T) {
	got := repl(t, "fn counter() { let n = 0; return fn() { n = n + 1; return n; }; }\nlet c = counter();\nc();\nc();\nc();\n")
	if got != "1\n2\n3\n" {
		t.Errorf("expected \"1\\n2\\n3\\n\", got %q", got)
	}
}
