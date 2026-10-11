# CCWIG network benchmark

How fast can a CCWIG program move bytes over a loopback TCP connection?

Each row is the same exchange - send a payload of *message size* bytes, wait for
the same number of bytes to come back - made *round trips* times, over the same
socket to the same echo server. The `go` rows are that same exchange written in
Go and run as a separate process. They are the control: without them a latency
figure says how fast the machine is, and with them it says how much of the time
is the language.

| client | message size | round trips | wall ms | µs per round trip | round trips/s | MB/s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| ccwig | 64 | 20,000 | 334 | 16.7 | 59,922 | 7.3 |
| go | 64 | 20,000 | 280 | 14.0 | 71,494 | 8.7 |
| ccwig | 256 | 20,000 | 331 | 16.6 | 60,383 | 29.5 |
| go | 256 | 20,000 | 285 | 14.2 | 70,265 | 34.3 |
| ccwig | 1,024 | 15,000 | 277 | 18.5 | 54,161 | 105.8 |
| go | 1,024 | 15,000 | 225 | 15.0 | 66,613 | 130.1 |
| ccwig | 4,096 | 10,000 | 215 | 21.5 | 46,412 | 362.6 |
| go | 4,096 | 10,000 | 152 | 15.2 | 65,657 | 512.9 |
| ccwig | 16,384 | 5,000 | 170 | 34.0 | 29,452 | 920.4 |
| go | 16,384 | 5,000 | 91 | 18.1 | 55,150 | 1723.5 |

At 16,384-byte messages a round trip costs the CCWIG client **1.87× what it costs
the Go one**. The difference is not the socket: both clients make the same system
calls, in the same order, to the same server. It is what happens between them -
a tree-walking interpreter calling two builtins per round trip, and building and
comparing payloads a byte at a time.

## What is measured

- **wall ms** is the whole client process, from `fork` to exit: reading the source,
  parsing it, connecting, and doing the work.
- **µs per round trip** is `wall ms` divided by the number of round trips. It is the
  number a person means by latency.
- **MB/s** counts both directions, because every byte is sent and comes back.
- **loop_us** in the CSV is what the client itself timed with its own clock, between
  the first send and the last receive. It is lower than `wall ms` by the cost of
  starting up, and the two agreeing closely is what says nothing outside the loop
  is taking the time. It is in microseconds rather than milliseconds because the
  [latency sweep](latency.md) reads its first row off this column, and that row is
  a few microseconds wide.
- Every figure is the **median of the runs**, not the fastest: a loopback benchmark on
  a machine doing anything else has occasional slow runs and no occasional fast ones,
  so the middle of the distribution is the honest number and the best one is not.

## What is not measured

This is not a measurement of the kernel's TCP. Loopback is the fastest and least
interesting network there is, and the echo server is a copy in both directions with
nothing in between. What the numbers are about is the interpreter: how much work
stands between a program saying `net_send` and the bytes leaving the process.

Nothing here is a comparison with another language's TCP library, and the Go rows
are not a claim that Go is faster than CCWIG in general. They are the control: the
same work, same machine, no interpreter.

## How to reproduce

```sh
make report
```

or, without make:

```sh
go build -o bin/ccwig ./cmd/ccwig
go run ./cmd/netbench -out reports
```

## The machine this ran on

    go 1.24.4 · darwin/arm64 · 10 CPUs · commit 57d5dc9 · 2026-10-11 23:31 UTC, median of 3 runs · run by hand with make report

## Mismatches

Every reply matched its request byte for byte, in every run of every row. A
benchmark of a client that is not moving the right bytes is a benchmark of the
wrong program.
