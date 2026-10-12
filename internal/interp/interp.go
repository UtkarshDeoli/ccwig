// Package interp ties the lexer, parser and evaluator together into a single
// "run this source" entry point, and provides the interactive REPL.
package interp

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/UtkarshDeoli/ccwig/internal/evaluator"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/parser"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// DefaultPrompt is what the REPL prints when waiting for input.
const DefaultPrompt = ">> "

// continuationPrompt is printed while a statement is still being typed.
const continuationPrompt = ".. "

// SyntaxError reports one or more problems found while reading the source.
// Every problem found is listed, not just the first, because a reader fixing
// a file one error per run is a poor experience.
type SyntaxError struct {
	Filename string
	Problems []string
}

func (e *SyntaxError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d syntax error(s)", e.Filename, len(e.Problems))
	for _, p := range e.Problems {
		b.WriteString("\n  ")
		b.WriteString(p)
	}
	return b.String()
}

// RuntimeError reports a failure that happened while evaluating the program.
type RuntimeError struct {
	Filename string
	Pos      token.Position
	Message  string
}

func (e *RuntimeError) Error() string {
	return fmt.Sprintf("%s:%s: %s", e.Filename, e.Pos, e.Message)
}

// ExitError reports that the program called `exit` and stopped on purpose.
// It is not a failure, which is why it is a type of its own: the command line
// turns it into the process status and prints nothing.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("exit status %d", e.Code)
}

// Interpreter evaluates CCWIG source. A single Interpreter keeps one global
// environment, so successive Eval calls share state; that is what lets the
// REPL remember variables between lines.
type Interpreter struct {
	// Out receives program output and REPL results.
	Out io.Writer
	// Prompt is printed by the REPL before reading a statement. Set it to
	// the empty string to run the REPL silently.
	Prompt string
	// Env is the global environment shared by every Eval call.
	Env *object.Environment
	// In is the standard input a program may read with `readLine` or
	// `readAll`. A nil In means there is none, which is the case in the REPL.
	In io.Reader
	// Args is what `args()` returns. It excludes the interpreter and the file
	// being run.
	Args []string

	ev *evaluator.Evaluator
}

// New returns an Interpreter writing output to out.
func New(out io.Writer) *Interpreter {
	return &Interpreter{
		Out:    out,
		Prompt: DefaultPrompt,
		Env:    object.NewEnvironment(),
		ev:     evaluator.New(out),
	}
}

// Eval parses and evaluates src, returning the value of its final statement.
// filename appears in error messages.
//
// This is the boundary that turns an `exit` panic into an ordinary error, so
// it is the outermost frame of a program run: the panic unwinds the evaluator
// to here, where the status code is still available and the interpreter is
// still usable.
func (in *Interpreter) Eval(src, filename string) (result object.Object, err error) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		exit, ok := r.(evaluator.ExitError)
		if !ok {
			// Not ours; let it carry on to whoever can deal with it.
			panic(r)
		}

		// The panic unwound past the call-depth decrements, so the counter is
		// left higher than it should be. Restoring it keeps an interpreter
		// that survives an exit — the REPL, or a caller that catches this —
		// honest about how deep it is.
		in.ev.ResetCallDepth()
		result, err = nil, &ExitError{Code: exit.Code}
	}()

	in.ev.In = in.In
	in.ev.Args = in.Args

	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()

	if problems := p.Errors(); len(problems) > 0 {
		return nil, &SyntaxError{Filename: filename, Problems: problems}
	}

	result = in.ev.Eval(program, in.Env)
	if errObj, ok := result.(*object.Error); ok {
		return nil, &RuntimeError{Filename: filename, Pos: errObj.Pos, Message: errObj.Message}
	}

	return result, nil
}

// Run parses and evaluates src for its side effects, discarding the final
// value.
func (in *Interpreter) Run(src, filename string) error {
	_, err := in.Eval(src, filename)
	return err
}

// RunREPL reads statements from r one line at a time, evaluating each as soon
// as it is complete, and writes results and error messages to Out.
//
// A statement that leaves brackets open keeps reading, so multi-line
// functions and blocks can be typed naturally. Errors are reported without
// ending the session.
func (in *Interpreter) RunREPL(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var pending strings.Builder

	for {
		if pending.Len() == 0 {
			in.writePrompt(in.Prompt)
		} else if in.Prompt != "" {
			in.writePrompt(continuationPrompt)
		}

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()

		if pending.Len() == 0 {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if trimmed == "quit" || trimmed == "exit" {
				return nil
			}
		}

		pending.WriteString(line)
		pending.WriteString("\n")

		src := pending.String()
		if needsMoreInput(src) {
			continue
		}

		// An `exit` ends the session, as typing quit would.
		if err := in.runOne(src); err != nil {
			return nil
		}
		pending.Reset()
	}

	// Report anything left over when input ended mid-statement.
	if pending.Len() > 0 {
		if err := in.runOne(pending.String()); err != nil {
			return nil
		}
	}

	return scanner.Err()
}

// runOne evaluates a single REPL entry and reports the outcome. It returns an
// error only for an `exit`, which ends the session rather than being printed.
func (in *Interpreter) runOne(src string) error {
	result, err := in.Eval(src, "<stdin>")
	if err != nil {
		if exit, ok := err.(*ExitError); ok {
			return exit
		}
		fmt.Fprintln(in.Out, err.Error())
		return nil
	}

	// Statements that produce nothing (a `let`, or `print`, which returns
	// null) are not echoed, so that `print(1)` does not also print "null".
	if result == nil || result.Type() == object.NULL {
		return nil
	}

	fmt.Fprintln(in.Out, result.Inspect())
	return nil
}

func (in *Interpreter) writePrompt(prompt string) {
	if prompt == "" {
		return
	}
	fmt.Fprint(in.Out, prompt)
}

// needsMoreInput reports whether src is an unfinished statement that should
// keep reading input, either because brackets are still open or because a
// string or comment was never closed.
func needsMoreInput(src string) bool {
	l := lexer.New(src)
	depth := 0

	for {
		tok := l.NextToken()

		switch tok.Type {
		case token.LBRACE, token.LPAREN, token.LBRACKET:
			depth++
		case token.RBRACE, token.RPAREN, token.RBRACKET:
			depth--
		case token.EOF:
			for _, err := range l.Errors() {
				if strings.Contains(err.Msg, "unterminated") {
					return true
				}
			}
			return depth > 0
		}
	}
}
