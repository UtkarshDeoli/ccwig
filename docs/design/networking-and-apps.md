# Design: growing CCWIG from a language into a toolkit

Date: 2026-10-12
Status: approved and in progress

## Why

CCWIG today is a complete small language with eight example programs, a REPL,
and a test suite. It can loop, recurse, close over state and draw a maze. What
it cannot do is *touch anything outside itself*: it cannot read its arguments,
read a line of input, open a socket, or twiddle bits. Every example is
therefore a demonstration rather than a tool.

This design closes that gap in four movements, which is the shape of the
request that prompted it:

1. Real-world usable examples.
2. Real networking, performance-tested, with results published as reports.
3. The things C and C++ people actually do — bits, memory, systems.
4. A command-line application.

The first three need capability the language does not have, so the work starts
there.

## What is being added

### 1. Bitwise operators

`&`, `|`, `^`, `<<`, `>>` and unary `~`, on `INTEGER` only, with C semantics
over 64-bit signed values. Floats are an error, as they already are for `%`.

Precedence follows C exactly, which is the only defensible choice for a
language with a C accent:

| Level | Operators |
| --- | --- |
| loosest | `=` |
| | `\|\|` |
| | `&&` |
| | `\|` |
| | `^` |
| | `&` |
| | `==` `!=` |
| | `<` `>` `<=` `>=` |
| | `<<` `>>` |
| | `+` `-` |
| tightest of these | `*` `/` `%` |

The lexer currently reports a bare `&` or `|` as an error suggesting `&&` or
`||`. That error goes away; the tokens become real. `<<` and `>>` are
recognised before `<=`/`>=` and `<`/`>`.

A shift by a negative amount is an error rather than a panic. Go's shift
semantics already define a shift at or beyond the operand width as zero (or
sign-fill), which is the behaviour C leaves undefined and we may as well fix.

### 2. Strings you can actually process

`len` and `+` are not enough to write a tokeniser. Added:

| Call | Result |
| --- | --- |
| `s[i]` | the i-th character as a one-character string, or `null` out of range |
| `ord(s)` | the code point of the first character |
| `chr(n)` | the one-character string for a code point |
| `substr(s, start, len)` | a slice, clamped rather than failing |
| `split(s, sep)` | array of pieces; `""` separator splits into characters |
| `join(xs, sep)` | a string, calling `str` on each element |
| `find(s, sub)` | index of the first occurrence, or `-1` |
| `upper(s)` / `lower(s)` | case conversion |

Indexing a string is the one grammar change: `s[i]` previously errored with
"index operator not supported: STRING". Strings stay immutable, so `s[i] = c`
remains an error.

### 3. Input, output and arguments

A program that cannot see its arguments or its input cannot be a tool.

| Call | Result |
| --- | --- |
| `args()` | array of the program's arguments, excluding the interpreter's |
| `readLine()` | the next line without its newline, or `null` at end of input |
| `readAll()` | everything left on standard input as one string |
| `readFile(path)` | the whole file, or an error naming the path |
| `writeFile(path, s)` | writes `s`, returns `null` |
| `time()` | milliseconds since the Unix epoch, for timing |
| `exit(code)` | stops the program with that status |

`exit` is the awkward one: CCWIG propagates errors as values, and an exit code
is not an error. It is implemented as a typed panic recovered at the
`interp` boundary — the one place with a well-defined "the program is over"
frame — rather than by threading a new sentinel through every `isError` check.
The evaluator's fuzz targets recover it too, so a fuzzer that stumbles onto
`exit()` does not crash the test binary.

Standard input is read through a single buffered scanner held by the
evaluator, so `readLine` and `readAll` share one position.

### 4. Networking

Real TCP, exposed in the shape C exposes it: **integer file descriptors and
negative return codes**, with the message available from `net_error()`. This
is deliberately unlike the rest of the language, which prefers typed errors,
because the fd-and-errno shape is what makes socket code read like C and what
makes it composable (`if fd < 0 { ... }`).

| Call | Result |
| --- | --- |
| `net_listen(port)` | a listening fd, or `-1` |
| `net_accept(fd)` | a connection fd, or `-1` |
| `net_connect(host, port)` | a connection fd, or `-1` |
| `net_send(fd, s)` | bytes written, or `-1` |
| `net_recv(fd, max)` | up to `max` bytes as a string; `null` at end of stream |
| `net_recvline(fd)` | one line, without its newline; `null` at end of stream |
| `net_close(fd)` | `null` |
| `net_settimeout(fd, ms)` | `null` |
| `net_addr(fd)` | the peer's `host:port`, or `""` |
| `net_error()` | the last socket error, or `""` |

Sockets live in a registry keyed by fd in a new `internal/sockets` package.
Calls block the interpreter, which is correct: the evaluator is single
threaded, so a blocking `accept` behaves the way a one-client-at-a-time C
server does. No goroutines are needed, and none are hidden.

Timeouts are implemented with deadlines on the underlying connection so that a
test — or a benchmark — can never hang.

### 5. The examples

Two new groups under `examples/`, both wired into the existing integration
harness with checked-in `.out` files:

- `examples/real/` — programs that do a job: `wc.cw`, `grep.cw`, `head.cw`,
  `csvreport.cw` (parse a CSV, aggregate, print a table), `json.cw`
  (tokenise and pretty-print a small JSON document), `expr.cw` (a tokeniser
  and recursive-descent evaluator for arithmetic).
- `examples/systems/` — the C-flavoured set: `bitset.cw` (a sieve over a
  bitset), `malloc.cw` (a bump allocator with a free list over a flat integer
  array), `hashtable.cw` (open addressing with string keys), `vm.cw` (a stack
  bytecode machine and a program for it), `crc32.cw`.

- `examples/net/` — `echo-server.cw`, `http-get.cw` (a real HTTP/1.0 request
  and response over a raw socket), `chat-server.cw`, and `netbench.cw`.

### 6. Performance reports, published

A Go harness at `cmd/netbench` measures CCWIG's socket layer from both sides
(a Go client against the CCWIG server, and the CCWIG client against a Go
server) and writes:

- `reports/netbench.csv` — machine-readable, one row per measurement.
- `reports/netbench.md` — a table a person reads.
- `reports/index.html` — the same numbers as a page, for GitHub Pages.

`make report` regenerates them. A GitHub Actions workflow at
`.github/workflows/bench.yml` runs the harness on push to `main` and on
demand, publishes `reports/` to GitHub Pages, and stores the run as an
artifact. The committed `.md` is the snapshot that shows in the repository
without anyone opening a browser.

### 7. The command-line application

`apps/todo/todo.cw` — a task manager with subcommands (`add`, `list`, `done`,
`rm`, `clear`) persisting to a plain-text file, built on `args`, `readFile`
and `writeFile`. It is the flagship demonstration that CCWIG can carry a real
program: argument parsing, a file format, error messages, and an exit code.

`make todo ARGS="add buy milk"` runs it.

## How it is tested

Every capability above lands with unit tests beside the code that implements
it, in the existing style. The examples are covered by extending
`integration_test.go`: programs that read standard input get their stdin
supplied, and their output is compared against a checked-in `.out` file. The
networking examples are tested with a Go-side test that stands up the server
and talks to it, so `go test ./...` never depends on a port being free for
long and never hangs — every socket it opens has a deadline.

`make check` (gofmt, vet, test) must be green at every commit.

## Order of work

1. Bitwise operators — lexer, token, parser, evaluator, docs, tests.
2. String builtins and string indexing.
3. I/O and argument builtins, including `exit` and its plumbing.
4. Networking builtins in `internal/sockets`.
5. `examples/real/` and `examples/systems/` with outputs and integration tests.
6. `examples/net/`, the `cmd/netbench` harness, the reports, and the workflow.
7. `apps/todo/todo.cw`, docs, README.
8. Branch pushed, pull request opened.

## Out of scope

A bytecode backend, compiling C, `struct`, pointers as a language feature,
`break`/`continue`, and maps. Each is real future work; none is needed for
the four movements above, and adding them would turn a sharp design into a
blunt one.
