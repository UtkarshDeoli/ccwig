// Command ccwig runs programs written in CCWIG, a small C-like language
// implemented as a tree-walking interpreter.
//
//	ccwig program.cw        run a file
//	ccwig                   start an interactive session
//	ccwig -e 'print(1)'     run source given on the command line
//
// Use -tokens or -ast to inspect what the lexer and parser make of the input.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/UtkarshDeoli/ccwig/internal/interp"
	"github.com/UtkarshDeoli/ccwig/internal/lexer"
	"github.com/UtkarshDeoli/ccwig/internal/parser"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// Version is the interpreter's version string, shown by -version.
const Version = "1.0.0"

// Exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the command line and returns the process exit code. It is
// separate from main so that it can be tested without spawning a process.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ccwig", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr) }

	source := fs.String("e", "", "evaluate `source` instead of reading a file")
	showTokens := fs.Bool("tokens", false, "print the token stream instead of running the program")
	showAST := fs.Bool("ast", false, "print the parsed syntax tree instead of running the program")
	showVersion := fs.Bool("version", false, "print the version and exit")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// -h/-help is a successful request for information.
			printUsage(stdout)
			return exitOK
		}
		return exitUsage
	}

	// An explicit `-e ""` means "run nothing successfully", which is
	// different from not passing -e at all (which starts the REPL), so the
	// flag's presence has to be detected rather than its value.
	sourceSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "e" {
			sourceSet = true
		}
	})

	if *showVersion {
		fmt.Fprintf(stdout, "ccwig %s\n", Version)
		return exitOK
	}

	rest := fs.Args()
	if sourceSet && len(rest) > 0 {
		fmt.Fprintln(stderr, "ccwig: give either -e or a file to run, not both")
		return exitUsage
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "ccwig: expected at most one file, got %d\n", len(rest))
		return exitUsage
	}

	// Decide what to run: -e, a file, or the REPL.
	var (
		src      string
		filename string
		interact bool
	)

	switch {
	case sourceSet:
		src, filename = *source, "<command line>"
	case len(rest) == 1:
		contents, err := os.ReadFile(rest[0])
		if err != nil {
			fmt.Fprintf(stderr, "ccwig: %v\n", err)
			return exitError
		}
		src, filename = string(contents), rest[0]
	default:
		interact = true
	}

	if *showTokens {
		return dumpTokens(src, filename, stdout, stderr)
	}
	if *showAST {
		return dumpAST(src, filename, stdout, stderr)
	}

	in := interp.New(stdout)

	if interact {
		if err := in.RunREPL(stdin); err != nil {
			fmt.Fprintf(stderr, "ccwig: %v\n", err)
			return exitError
		}
		return exitOK
	}

	if err := in.Run(src, filename); err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return exitError
	}

	return exitOK
}

// dumpTokens prints the lexer's output, one token per line.
func dumpTokens(src, filename string, stdout, stderr io.Writer) int {
	l := lexer.New(src)

	for {
		tok := l.NextToken()
		fmt.Fprintf(stdout, "%s\t%s\n", tok.Pos, tok)
		if tok.Type == token.EOF {
			break
		}
	}

	if lexErrors := l.Errors(); len(lexErrors) > 0 {
		problems := make([]string, 0, len(lexErrors))
		for _, err := range lexErrors {
			problems = append(problems, err.Error())
		}
		printProblems(filename, problems, stderr)
		return exitError
	}

	return exitOK
}

// dumpAST prints the parsed tree, which shows how expressions were grouped.
func dumpAST(src, filename string, stdout, stderr io.Writer) int {
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()

	if problems := p.Errors(); len(problems) > 0 {
		printProblems(filename, problems, stderr)
		return exitError
	}

	fmt.Fprintln(stdout, program.String())
	return exitOK
}

// printProblems reports problems using the same wording as the interpreter,
// so the CLI has one error format rather than two.
func printProblems(filename string, problems []string, stderr io.Writer) {
	fmt.Fprintf(stderr, "%v\n", &interp.SyntaxError{Filename: filename, Problems: problems})
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `ccwig - a C-like language interpreter

Usage:
  ccwig [options] [file]

With no file and no -e, ccwig starts an interactive session.

Options:
  -e source    evaluate source instead of reading a file
  -tokens      print the token stream instead of running the program
  -ast         print the parsed syntax tree instead of running the program
  -version     print the version and exit
  -h, -help    print this message

Examples:
  ccwig examples/fizzbuzz.cw
  ccwig -e 'print("hello");'
  ccwig -tokens -e 'let x = 1;'
`)
}
