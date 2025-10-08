## CCWIG -> C-like Compiler Written in Go

> I am starting this project on 2024-08-28
> The Code will be written on a Raspberry pi zero 2W
> Just to maybe challange myself and prove that a complex level project can be made in simplest machine (Cost of a raspberry pi zero 2 W at this time in india is around 1.5k rupees or roughly 15 dollars)

CCWIG is a from-scratch implementation of a small C-like language: a hand-written
lexer, a recursive-descent parser, and a tree-walking evaluator, with a CLI and a
REPL on top. It is roughly 3,100 lines of Go behind 2,500 lines of tests, and has
no third-party dependencies — the module requires nothing but the standard
library.

**A note on the name.** The title says "compiler", but what is implemented here is
an *interpreter* — the syntax tree is walked directly and there is no code
generation, no bytecode, and no `.c` input. That was the plan in Day 1 (see the
"three main parts of an interpreter" item below). Compiling real C, or adding a
bytecode backend to this language, is future work.

```sh
make build
./bin/ccwig examples/fizzbuzz.cw
./bin/ccwig                      # interactive REPL
./bin/ccwig -e 'print(1 + 2);'
```

See [docs/LANGUAGE.md](docs/LANGUAGE.md) for the full language reference and
[examples/](examples/) for runnable programs.

---
### 🛠️ To-Do: Build a Go Interpreter/Compiler in 1 Week

## **Day 1: Set Up and Learn Core Concepts**
- [x] Install Go and set up the environment.
- [x] Test installation using `go version`.
- [x] Complete [Go Tour](https://tour.golang.org/) to learn basics:
  - Variables, loops, conditionals.
  - Structs, interfaces, functions.
  - All basic Stuff (Go as deep as you want to)
- [x] Learn the three main parts of an interpreter:
  - Lexical Analysis (Tokenizer).
  - Parsing (Abstract Syntax Tree).
  - Evaluation (Executing AST).
- [x] Define the minimal language features:
  - Arithmetic operations (`+`, `-`, `*`, `/`, `%`).
  - Variables and assignments.
  - Conditional statements (`if`, `else`).
  - Print statements for output.

  The language ended up a little larger than the minimum: `while` and `for`
  loops, functions and closures, arrays, floats and strings, `&&` / `||`, and
  the comparison and assignment operators. The feature set is recorded in
  [docs/LANGUAGE.md](docs/LANGUAGE.md).

## **Day 2: Implement the Lexer**
- [x] Define token types (`NUMBER`, `IDENTIFIER`, `OPERATOR`, `KEYWORD`, etc.).
- [x] Write the `Lexer` function to tokenize input.
- [x] Handle whitespace and invalid input gracefully.
- [x] Test the lexer with simple examples:
  - Input: `x = 5 + 3`.
  - Output: A list of tokens.

  Tokens and positions live in `internal/token`, the scanner in `internal/lexer`.
  It tracks line and column by *character*, not by byte, so a multi-byte
  character such as `é` does not shift the columns that follow it.

## **Day 3: Implement the Parser**
- [x] Define AST nodes using structs:
  - Binary expressions.
  - Variable declarations.
  - Literals.
- [x] Write a recursive descent parser to build an AST:
  - Parse expressions like `1 + 2`.
  - Handle assignments like `x = 5`.
- [x] Test the parser:
  - Input: `x = 1 + 2`.
  - Output: A tree-like structure (AST).

  Expressions use precedence climbing, so `1 + 2 * 3` groups correctly; see
  `internal/parser`. `ccwig -ast -e '1 + 2 * 3;'` prints the tree.

## **Day 4: Implement the Interpreter**
- [x] Write an `Eval` function to evaluate the AST.
- [x] Store variables in a map for assignments.
- [x] Handle:
  - Arithmetic operations.
  - Variable lookups.
  - Print statements.
- [x] Add basic error handling for:
  - Invalid syntax.
  - Undefined variables.
- [x] Test the interpreter:
  - Input: `x = 10; y = x + 5; print(y)`.
  - Output: `15`.

  The evaluator is in `internal/evaluator`, runtime values in `internal/object`.

## **Day 5: Add Basic Features**
- [x] Add support for conditionals:

  ```c
  if x > 5 {
      print("greater");
  } else {
      print("less");
  }
  ```

- [x] (Optional) Add looping constructs like while or for.
- [x] (Optional) Implement functions and function calls.

  Functions close over the scope they are defined in, so counters, recursion
  and callbacks all work — see `examples/closures.cw`.

## **Day 6: Polish and Extend**

- [x] Improve error messages for:
  - Lexing errors.
  - Parsing errors.
  - Runtime errors.

- [x] Build a simple CLI or REPL:

  - Allow users to run files or interactively input code.

- [x] Enhance syntax with:
  - Comments.
  - String literals.
  - Additional operators.

  Syntax errors are all reported at once, with positions, and the program does
  not run. Runtime errors report `file:line:col: message`, for example
  `examples/errors/type-mismatch.cw:3:9: type mismatch: INTEGER + STRING`.

## **Day 7: Refine and Test**
  ### Test edge cases:
  - [x] Invalid input.
  - [x] Large expressions.
  - [x] Edge scenarios.

  `go test ./...` runs 178 test functions (710 cases counting subtests) across
  the lexer, parser, evaluator, REPL and CLI. `integration_test.go` runs every
  program in `examples/` and compares its
  output against a checked-in `.out` file, and asserts that each program in
  `examples/errors/` fails with the expected message and names the file.

  Seven fuzz targets cover the three phases — that the lexer always advances and
  always gives a token a position, that the parser always terminates, that
  evaluation is deterministic and always returns a value, and that a program and
  its rendering behave identically. `make fuzz` runs them.

  The parser is also held to a round-trip property: re-parsing a rendered tree
  must produce the same tree again. Both the REPL and `-ast` show a rendering of
  the tree, so a rendering the parser cannot read back is a real bug, and this
  one property found most of the defects listed below. `make bench` runs the
  benchmarks.

### Optimize code:

  - [x]  Simplify logic.
  - [x]  Add comments for readability.
  - [x]  Measure the hot paths and make them faster.

  `make bench` runs the benchmarks. Against the interpreter as first written,
  evaluation is 23–36% faster and allocates roughly a fifth fewer objects,
  mostly from three changes: a scope keeps its bindings in parallel slices
  instead of a map, operators dispatch on the concrete type of their operands
  rather than on type-name strings, and `true`, `false` and `null` are shared
  instances rather than a fresh allocation each time.

  | Benchmark | Before | After | |
  | --- | ---: | ---: | ---: |
  | Loop body | 194,991 ns | 125,480 ns | −36% |
  | While loop | 182,198 ns | 130,593 ns | −28% |
  | Function calls | 2,260,440 ns | 1,734,911 ns | −23% |
  | Array index | 314,663 ns | 231,056 ns | −27% |
  | String concatenation | 43,167 ns | 32,900 ns | −24% |
  | Built-in calls | 123,790 ns | 90,742 ns | −27% |
  | Nested scope lookup | 60,417 ns | 39,299 ns | −35% |
  | Arithmetic | 423 ns | 396 ns | −6% |

  Parsing is about 2% slower for the same input, because the parser now counts
  how deeply an expression nests so that pathological source reports an error
  instead of exhausting the stack and killing the process.

### Document the project:

  - [x]  Write basic documentation for the language.

  See [docs/LANGUAGE.md](docs/LANGUAGE.md). A runaway loop is stopped by a step
  limit rather than hanging, so `while true {}` fails with a diagnosable
  message. Runaway recursion and pathologically nested source are stopped the
  same way; the three limits and why none of them covers the others are
  described under [Limits](docs/LANGUAGE.md#limits).

### Package the code:
  - [x] Create a executable

  `make build` writes `bin/ccwig`. `make pi` cross-compiles for the Raspberry Pi
  Zero 2 W (`linux/arm64`) and the original Zero (`linux/armv6`, GOARM=6).
---

### Project layout

| Path | Contents |
| --- | --- |
| `internal/token` | Token kinds and source positions |
| `internal/lexer` | Source text to tokens |
| `internal/ast` | Syntax tree nodes |
| `internal/parser` | Tokens to syntax tree |
| `internal/object` | Runtime values and the environment |
| `internal/evaluator` | Tree-walking evaluation and built-ins |
| `internal/interp` | Facade over lex/parse/eval, plus the REPL |
| `cmd/ccwig` | Command-line entry point |

### Make targets

| Target | Does |
| --- | --- |
| `make` | `check` then `build` |
| `make test` | `go test ./...` |
| `make check` | `gofmt`, `go vet`, `go test` |
| `make run` | Build and start the REPL |
| `make examples` | Run every program in `examples/` |
| `make pi` | Cross-compile for the Raspberry Pi |
| `make help` | List every target |

### Timeline
Check [Blog](https://www.utkarshdeoli.in/blog) for complete timeline
