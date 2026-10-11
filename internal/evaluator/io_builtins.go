package evaluator

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// The input, output and argument builtins. Without these a CCWIG program could
// compute but not interact: it could not see the file it was pointed at, the
// line it was piped, or the arguments it was given, so it could only ever be a
// demonstration rather than a tool.
const (
	builtinArgs      = "args"
	builtinReadLine  = "readLine"
	builtinReadAll   = "readAll"
	builtinReadFile  = "readFile"
	builtinWriteFile = "writeFile"
	builtinExists    = "exists"
	builtinTime      = "time"
	builtinExit      = "exit"
)

// maxExitCode is the largest status `exit` accepts. A process status is a byte
// on Unix, and a program that asked for 300 got 44 without being told.
const maxExitCode = 255

// ExitError carries a status code out of a running program. It is panicked by
// `exit` and recovered at the interpreter boundary.
//
// A panic is the right shape here and the only place in CCWIG where one is
// used deliberately. Every other failure is a value that propagates through
// `isError` checks, but an exit is not a failure — it is the program saying it
// is finished — and threading a second kind of "stop everything" sentinel
// through every one of those checks would cost the common path for the sake of
// the rare one. The boundary that recovers it is the one place with a
// well-defined idea of "the program is over".
type ExitError struct {
	Code int
}

func (e ExitError) Error() string { return fmt.Sprintf("exit with status %d", e.Code) }

func (e *Evaluator) ioBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		builtinArgs: {
			Name: builtinArgs,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinArgs, args, 0, token.Position{}); err != nil {
					return err
				}
				elements := make([]object.Object, len(e.Args))
				for i, a := range e.Args {
					elements[i] = &object.String{Value: a}
				}
				return &object.Array{Elements: elements}
			},
		},

		builtinReadLine: {
			Name: builtinReadLine,
			// One line, without its terminator, or null at the end of input.
			// A trailing carriage return is dropped too, so a file with CRLF
			// endings reads the same as one without.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinReadLine, args, 0, token.Position{}); err != nil {
					return err
				}
				in, errObj := e.input()
				if errObj != nil {
					return errObj
				}

				line, err := in.ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return object.NewError(token.Position{}, "reading standard input: %v", err)
				}
				if line == "" && errors.Is(err, io.EOF) {
					return object.NullValue()
				}

				return &object.String{Value: trimLineEnding(line)}
			},
		},

		builtinReadAll: {
			Name: builtinReadAll,
			// Everything left on the input, as it is. This is what a filter
			// that works a whole document at once — a JSON pretty-printer, say
			// — reads instead of looping over lines.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinReadAll, args, 0, token.Position{}); err != nil {
					return err
				}
				in, errObj := e.input()
				if errObj != nil {
					return errObj
				}

				data, err := io.ReadAll(in)
				if err != nil {
					return object.NewError(token.Position{}, "reading standard input: %v", err)
				}
				return &object.String{Value: string(data)}
			},
		},

		builtinReadFile: {
			Name: builtinReadFile,
			// A missing file is an error rather than null: a program that
			// cannot find its input is broken, and finding that out at the
			// point of the read is more useful than a null that travels.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinReadFile, args, 1, token.Position{}); err != nil {
					return err
				}
				path, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "argument to `readFile` must be a string, got %s", args[0].Type())
				}

				data, err := os.ReadFile(path.Value)
				if err != nil {
					return object.NewError(token.Position{}, "cannot read %s: %v", path.Value, err)
				}
				return &object.String{Value: string(data)}
			},
		},

		builtinWriteFile: {
			Name: builtinWriteFile,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinWriteFile, args, 2, token.Position{}); err != nil {
					return err
				}
				path, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "first argument to `writeFile` must be a string, got %s", args[0].Type())
				}
				contents, ok := args[1].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "second argument to `writeFile` must be a string, got %s", args[1].Type())
				}

				if err := os.WriteFile(path.Value, []byte(contents.Value), 0o644); err != nil {
					return object.NewError(token.Position{}, "cannot write %s: %v", path.Value, err)
				}
				return object.NullValue()
			},
		},

		builtinExists: {
			Name: builtinExists,
			// Whether there is anything at this path.
			//
			// `readFile` on a missing file is an error that stops the program,
			// which is right for a program that was pointed at a file it needs.
			// But a program whose file is *allowed* to be missing - the store
			// behind a command-line tool, on its first run - cannot start at
			// all without asking first, and there is nothing to catch an error
			// with. This is that question.
			//
			// It answers whether something is there, not whether it can be
			// read: a directory exists, and reading it is still an error.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinExists, args, 1, token.Position{}); err != nil {
					return err
				}
				path, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "argument to `exists` must be a string, got %s", args[0].Type())
				}

				_, err := os.Stat(path.Value)
				return object.BooleanOf(err == nil)
			},
		},

		builtinTime: {
			Name: builtinTime,
			// Milliseconds since the Unix epoch. It is the clock a CCWIG
			// program needs to time itself, which is what the networking
			// benchmarks use it for.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinTime, args, 0, token.Position{}); err != nil {
					return err
				}
				return &object.Integer{Value: time.Now().UnixMilli()}
			},
		},

		builtinExit: {
			Name: builtinExit,
			// exit(code) stops the program with that status; exit() stops it
			// with zero. Anything outside a byte is refused rather than
			// truncated.
			Fn: func(args []object.Object) object.Object {
				code := int64(0)

				switch len(args) {
				case 0:
				case 1:
					n, ok := args[0].(*object.Integer)
					if !ok {
						return object.NewError(token.Position{}, "argument to `exit` must be an integer, got %s", args[0].Type())
					}
					code = n.Value
				default:
					return object.NewError(token.Position{}, "wrong number of arguments to `exit`: want at most 1, got %d", len(args))
				}

				if code < 0 || code > maxExitCode {
					return object.NewError(token.Position{}, "argument to `exit` must be between 0 and %d, got %d", maxExitCode, code)
				}

				panic(ExitError{Code: int(code)})
			},
		},
	}
}

// input returns the buffered reader over standard input, creating it on first
// use so that `readLine` and `readAll` share one position.
//
// A nil In means there is nothing to read, which is the case in the REPL: the
// session already owns standard input, and a builtin that quietly consumed it
// from underneath the prompt would do more harm than one that says so.
func (e *Evaluator) input() (*bufio.Reader, *object.Error) {
	if e.In == nil {
		return nil, object.NewError(token.Position{}, "no standard input available to read")
	}
	if e.stdin == nil || e.stdinFor != e.In {
		e.stdin = bufio.NewReader(e.In)
		e.stdinFor = e.In
	}
	return e.stdin, nil
}

// trimLineEnding removes a trailing newline, and a carriage return before it.
func trimLineEnding(line string) string {
	line = trimSuffixByte(line, '\n')
	return trimSuffixByte(line, '\r')
}

func trimSuffixByte(s string, b byte) string {
	if len(s) > 0 && s[len(s)-1] == b {
		return s[:len(s)-1]
	}
	return s
}
