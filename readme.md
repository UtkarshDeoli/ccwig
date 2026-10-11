# CCWIG

CCWIG is a small programming language with a C accent, and this repository is
the whole of it: a hand-written lexer, a recursive-descent parser and a
tree-walking evaluator, with a command-line interface and a REPL on top.

It is about 5,800 lines of Go behind 7,700 lines of tests, and it has no
dependencies at all — the module imports nothing beyond the standard library.

**On the name.** CCWIG stands for "C-like Compiler Written in Go", but what is
here is an *interpreter*: the syntax tree is walked directly, and there is no
code generation, no bytecode and no `.c` input. Compiling real C, or adding a
bytecode backend to this language, is future work.

**On the machine.** The project was started on 2024-08-28, and the code is
written on a Raspberry Pi Zero 2W — a machine that cost about 1.5k rupees
(roughly 15 dollars) in India at the time. The point was to find out whether a
project of this size can be built happily on the simplest computer around. So
far it can.

## Quick start

```sh
git clone https://github.com/UtkarshDeoli/ccwig
cd ccwig
make build

./bin/ccwig examples/fizzbuzz.cw   # run a program
./bin/ccwig                        # interactive REPL
./bin/ccwig -e 'print(1 + 2);'     # run a one-liner

./bin/ccwig apps/todo/todo.cw add buy milk   # a program that keeps a file
./bin/ccwig apps/todo/todo.cw list
```

Nothing needs installing and there is nothing to fetch. `go run ./cmd/ccwig`
works anywhere `./bin/ccwig` does.

## Your first program

Here is a complete CCWIG program. It prints the five times table.

```c
// times.cw — the five times table, five rows tall

// row(n) builds one line of the table: the products of n, space separated.
fn row(n) {
    let line = "";
    for let i = 1; i <= 5; i = i + 1 {
        line = line + " " + str(n * i);
    }
    return line;
}

for let n = 1; n <= 5; n = n + 1 {
    print(row(n));
}
```

Running it (`./bin/ccwig times.cw`) prints:

```
 1 2 3 4 5
 2 4 6 8 10
 3 6 9 12 15
 4 8 12 16 20
 5 10 15 20 25
```

The pieces, one at a time:

- `fn row(n) { ... }` defines a function. `row` takes one parameter and
  `return`s its result. Functions are ordinary values, so they can be stored,
  passed and returned — which is how closures work.
- `let line = ""` declares a variable. `let` is block-scoped: a `let` inside
  braces belongs to those braces and disappears outside them.
- The `for` loop reads like C, minus the types: `for let i = 1; i <= 5; i = i + 1`
  is initialise, test, step. There is a `while` too.
- `line = line + " " + str(n * i)` builds the line by joining strings. CCWIG
  never converts silently — `1 + "2"` is an error, not `"12"` — so a number
  joins a string only once `str()` has made that conversion deliberate.
- `print` writes a line to the terminal. It is how every program in
  `examples/` shows its work.

The same code works in the REPL. Run `./bin/ccwig`, paste the `fn` block, then
call `row(3)`. Or poke at the language in one line:

```sh
./bin/ccwig -e 'let greet = fn(name) { return "hello, " + name; }; print(greet("world"));'
# hello, world
```

If you would rather be walked through the language a step at a time,
[docs/GUIDE.md](docs/GUIDE.md) does that, and finishes by building a small
calculator.

## What the language can do

`while` and `for` loops; functions and closures; arrays with a small library of
builtins (`len`, `push`, `fill`, `first`, `last`, `rest`, `type`, `str`);
integers, floats, strings and booleans; `&&` and `||` that short-circuit; both
kinds of C-style comment; and `clear` and `sleep`, which let a program redraw
the terminal frame by frame. Strings are counted by character rather than by
byte, so multi-byte text lands where it should.

A program can also talk to the machine it is on. `args()` is the command line,
`readLine()` and `readAll()` are standard input, `readFile`, `writeFile` and
`exists` are the filesystem, `exit(code)` is the exit status, and
`net_listen`, `net_accept`, `net_connect`, `net_send` and `net_recv` are real
TCP sockets with real file descriptors — numbered from 3 the way a C program
numbers them, `-1` on failure, and `net_error()` for the reason. There are no
threads, so a server watching several sockets gives each one a deadline with
`net_settimeout` and polls, which is what `select()` does in C.
[docs/LANGUAGE.md](docs/LANGUAGE.md#networking) covers that in full.

Errors are meant to be readable rather than survivable. Syntax errors are all
reported at once, with positions, and the program does not run. A failure while
running stops at the first error and reports the file, line and column:

```
examples/errors/type-mismatch.cw:3:9: type mismatch: INTEGER + STRING
```

A program that will not finish is stopped rather than left to hang. Runaway
loops, runaway recursion and pathologically nested source each hit a limit that
reports an ordinary error, so `while true {}` prints a message instead of
taking the machine with it. The three limits are described under
[Limits](docs/LANGUAGE.md#limits).

The full reference is [docs/LANGUAGE.md](docs/LANGUAGE.md).

## Examples

[`examples/`](examples/) holds eight runnable programs, each with its output
checked in beside it. `make examples` runs them all.

| Program | What it shows |
| --- | --- |
| [`hello.cw`](examples/hello.cw) | The smallest thing that runs |
| [`lang.cw`](examples/lang.cw) | A tour of every feature, in one file |
| [`fizzbuzz.cw`](examples/fizzbuzz.cw) | The classic, with `%` |
| [`fib.cw`](examples/fib.cw) | Recursion and string building |
| [`sorting.cw`](examples/sorting.cw) | Nested loops, and arrays changing under you |
| [`closures.cw`](examples/closures.cw) | Counters, callbacks, state that stays put |
| [`polish.cw`](examples/polish.cw) | A prefix-notation calculator, built up in the guide |
| [`maze.cw`](examples/maze.cw) | A recursive maze carver and two solvers |

The largest of them, [`maze.cw`](examples/maze.cw), carves a maze with a
recursive backtracker, solves it twice — breadth first and depth first — draws
both routes, and then checks in the language that each route really is a walk
from the start to the exit.

### Programs that do a job

[`examples/real/`](examples/real/) is six programs of the kind you would
actually reach for, each named on the command line the way the tool it imitates
is, and each reading a file rather than a variable:

| Program | What it shows |
| --- | --- |
| [`wc.cw`](examples/real/wc.cw) | Lines, words and characters in one pass, no lookahead |
| [`head.cw`](examples/real/head.cw) | An argument that arrives as text and has to become a number |
| [`grep.cw`](examples/real/grep.cw) | A pattern from the command line, not from the source |
| [`csvreport.cw`](examples/real/csvreport.cw) | Reading a data file and totalling it by field |
| [`expr.cw`](examples/real/expr.cw) | Recursive descent — sum of terms, term of factors |
| [`json.cw`](examples/real/json.cw) | Re-indenting a JSON document by walking it |

### Programs that do the hard part

[`examples/systems/`](examples/systems/) is five programs written the way a C
programmer writes them: no library doing the interesting work, and a check at
the end that recomputes the answer some other way.

| Program | What it shows |
| --- | --- |
| [`bitset.cw`](examples/systems/bitset.cw) | Sixty-four flags to a word, and a prime sieve on top |
| [`crc32.cw`](examples/systems/crc32.cw) | The checksum in every zip file, table-driven, verified bit by bit |
| [`hashtable.cw`](examples/systems/hashtable.cw) | Open addressing and the probing that makes it work |
| [`malloc.cw`](examples/systems/malloc.cw) | A first-fit heap allocator, and the fragmentation it cannot prevent |
| [`vm.cw`](examples/systems/vm.cw) | A stack machine, an assembler, and four programs to run |

### Programs that talk to other machines

[`examples/net/`](examples/net/) is four programs that open real sockets. Their
output cannot be a checked-in file — it depends on who answers — so
`net_examples_test.go` drives each one against a peer written in Go, and
`make net` runs that.

| Program | What it shows |
| --- | --- |
| [`echo-server.cw`](examples/net/echo-server.cw) | A server that answers every line, numbered |
| [`http-get.cw`](examples/net/http-get.cw) | Speaking HTTP/1.0 by hand and reporting the status line |
| [`chat-server.cw`](examples/net/chat-server.cw) | Many clients, one thread, a deadline per socket |
| [`netbench.cw`](examples/net/netbench.cw) | The CCWIG half of the benchmark below |

[`apps/todo/todo.cw`](apps/todo/todo.cw) is the other kind of program: a tool
you run, with subcommands (`add`, `list`, `done`, `remove`, `prune`), a `-f`
option naming its file, and an exit status. It has no recorded output either;
[`todo_app_test.go`](todo_app_test.go) checks every command by running it and
reading the file it leaves behind. `make todo` runs it.

[`examples/animate/maze.cw`](examples/animate/maze.cw) is the same maze animated
in the terminal, frame by frame (`make animate` plays it), and
[`examples/huge/maze.cw`](examples/huge/maze.cw) is the same maze grown to a
hundred and one cells square — ten thousand positions, routes of a few hundred
steps, and a picture you take in rather than trace (`make huge` runs it). Its
`SIZE` is the one number it invites you to change, so it deliberately has no
recorded output to break; what holds it together instead is that it counts what
it has built, and checks the count.

[`examples/errors/`](examples/errors/) is seven short programs that each fail
in a specific way, so you can see what the messages look like before you meet
them for real.

## The network benchmark

[`reports/`](reports/) holds a measurement of the one thing a language like this
is usually assumed to be bad at: moving bytes over a socket. Two clients make
the same exchange over loopback TCP — send a payload, wait for the same number
of bytes back — against the same echo server. One client is written in CCWIG
and run by the interpreter; the other is the same exchange written in Go and
run as a separate process. The Go rows are the control, so the difference
between the two columns is the interpreter and not the machine.

| message size | CCWIG µs/round trip | Go µs/round trip | CCWIG MB/s | Go MB/s |
| ---: | ---: | ---: | ---: | ---: |
| 64 B | 16.2 | 13.5 | 7.6 | 9.0 |
| 1,024 B | 18.0 | 14.6 | 108.4 | 133.7 |
| 4,096 B | 21.3 | 15.1 | 366.1 | 517.7 |
| 16,384 B | 33.7 | 17.9 | 928.1 | 1,745.1 |

At 16 KiB messages a round trip costs the CCWIG client **1.88× what it costs
the Go one**. The gap widens with the message because the part that grows is
the part the interpreter does: building, sending, receiving and comparing a
string a byte at a time, with two builtin calls per round trip. That is the
price of no compiler, measured rather than asserted.

Every figure is the median of three runs, after a discarded warm-up, on one
machine, and the report says which machine:

    go 1.24.4 · darwin/arm64 · 10 CPUs · commit f802d75, median of 3 runs

Three files come out of one run of `make report`:
[`netbench.md`](reports/netbench.md) is the readable version,
[`netbench.csv`](reports/netbench.csv) is the data behind it, and
[`reports/index.html`](reports/index.html) is a self-contained page with the
throughput drawn to scale. Nothing in the report is a number the program
chose — the client counts the bytes it moved, and the harness fails the run if
a single reply did not match its request byte for byte.

`.github/workflows/bench.yml` runs the same thing on a GitHub runner and
publishes the page, so the numbers can be re-measured somewhere other than the
computer they were taken on. Once it has run, the page is at
**<https://utkarshdeoli.github.io/ccwig/>**, and the run carries the three files
away as a downloadable artifact.

## Project layout

| Path | Contents |
| --- | --- |
| `internal/token` | Token kinds and source positions |
| `internal/lexer` | Source text to tokens |
| `internal/ast` | Syntax tree nodes |
| `internal/parser` | Tokens to syntax tree |
| `internal/object` | Runtime values and scopes |
| `internal/evaluator` | Tree-walking evaluation and builtins |
| `internal/interp` | Lex, parse and evaluate together, plus the REPL |
| `internal/sockets` | The descriptor table behind the `net_*` builtins |
| `cmd/ccwig` | Command-line entry point |
| `cmd/netbench` | The benchmark harness that writes `reports/` |

## Make targets

| Target | Does |
| --- | --- |
| `make` | `check` then `build` |
| `make build` | Write `bin/ccwig` |
| `make test` | `go test ./...` |
| `make check` | `gofmt`, `go vet`, `go test` |
| `make run` | Build and start the REPL |
| `make examples` | Run every program in `examples/` |
| `make animate` | Play the animated maze example |
| `make huge` | Run the very large maze example |
| `make bench` | Run the benchmarks |
| `make report` | Run the network benchmark and write `reports/` |
| `make report-quick` | The same, short enough to check the harness itself |
| `make net` | Run the networking examples against real peers |
| `make todo` | Run the to-do list application |
| `make fuzz` | Run every fuzz target for 30s each |
| `make pi` | Cross-compile for the Raspberry Pi |
| `make help` | List every target |

## Tests and benchmarks

`go test ./...` runs 286 test functions, which come to 862 cases counting
subtests. `integration_test.go` runs the nineteen programs in `examples/`,
`examples/real/` and `examples/systems/` and compares what each prints against
its checked-in `.out` file — with the arguments in its `.args` file and the
input in its `.in` file where it has them — and checks that each program in
`examples/errors/` fails with the message it is supposed to and names the file
it came from.

The programs whose answer does not fit in a file are held to something better
than a golden file: `net_examples_test.go` runs the four `examples/net/`
programs against peers written in Go, `todo_app_test.go` runs the to-do tool
and reads the file it leaves behind, and `real` and `systems` tests recompute
what can be recomputed independently, in Go, rather than trusting a program's
own report of its own work.

Two programs in subdirectories are deliberately not among those nineteen,
because there is no single output to compare: the animation draws hundreds of
frames, and the huge maze is meant to be resized. `TestAnimatedExample` and
`TestHugeExample` check what those two are supposed to do instead.

Seven fuzz targets cover the three phases: that the lexer always advances and
always gives every token a position, that the parser always terminates, that
evaluation is deterministic and always returns a value, and that a program and
its rendering behave identically. `make fuzz` runs them.

The parser is also held to a round-trip property — re-parsing a rendered syntax
tree must produce that same tree again. Both the REPL and `-ast` show a
rendering of the tree, so a rendering the parser cannot read back is a real
bug, and this one property found most of the defects fixed along the way.

`make bench` measures the hot paths. Against the interpreter as first written,
evaluation is 23–36% faster and allocates about a fifth fewer objects, mostly
from three changes: a scope keeps its bindings in parallel slices instead of a
map, operators dispatch on the concrete type of their operands rather than on
type-name strings, and `true`, `false` and `null` are shared instances rather
than a fresh allocation each time.

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

Parsing is about 2% slower for the same input than it was, because the parser
now counts how deeply an expression nests, so that pathological source reports
an error instead of exhausting the stack and killing the process.

## License

Apache 2.0 — see [LICENSE](LICENSE).
