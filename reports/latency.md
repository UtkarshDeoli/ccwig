# What a round trip costs when the network costs something

The [main report](netbench.md) puts two clients against each other on loopback,
where a round trip is about twenty microseconds and a few of them are the
interpreter. Loopback is the one network on which that share is large, and the
one network on which it does not matter to anybody.

These are the same two clients and the same exchange against a server that waits
before it answers, so that a round trip costs what a real one costs. Both clients
meet that server, in the same order, on the same machine, with the same wait in
front of each reply; the only thing that differs between the two rows of a line is
the language the client was written in.

![The interpreter's share of a round trip, against server delay](interpreter-share.svg)

| server delay | CCWIG µs/round trip | Go µs/round trip | difference | CCWIG's share |
| ---: | ---: | ---: | ---: | ---: |
| 0 ms | 17.9 | 14.6 | 3.3 | 18.62% |
| 1 ms | 1,427.5 | 1,400.0 | 27.5 | 1.93% |
| 10 ms | 11,932.3 | 11,884.7 | 47.6 | 0.40% |
| 50 ms | 51,916.7 | 51,809.9 | 106.7 | 0.21% |

The round trip grows from **18 µs** to **51.9 ms** down this table, and the
difference between the two clients never exceeded **107 µs** of it. At the longest
round trip here - 50 ms of delay - 107 µs is **0.21%** of what a person would wait.

That is a bound rather than a measurement, and deliberately. The difference is
divided by the number of round trips, so the fewer of them a row has, the more of
the machine's own jitter is in it: one scheduling hiccup of a millisecond, spread
over a few dozen round trips, is tens of microseconds that have nothing to do with
the language. A row that short states a ceiling the noise decided, which is still
an honest thing to state, and is the only honest thing that length supports.

The one row where the difference is a number rather than a ceiling is the first,
because it is the one row with enough round trips in it: **3.3 µs** of the 17.9 µs
round trip, or **18.6%**. That is the interpreter, measured where it can be: on
loopback, which is the only network where the interpreter is not the small part.

## What is measured

- Every figure is the client's own clock divided by the round trips, median of the
  runs. That clock starts at the first send and stops at the last receive, and is
  the one difference from the [main report](netbench.md), which measures the whole
  process from `fork` to exit.
- The main report charges every round trip for starting the interpreter, which is
  right when a run is twenty thousand round trips and the start-up is spread thin.
  Here a run is as few as a few dozen, so the same charge would put the start-up cost
  into a column about the network - which would look exactly like the interpreter
  getting slower as the server got further away.
- The first row is not measured here. It is the main report's rows for this
  message size, read off their own clocks, because the zero-delay point is the one
  point in this table where the difference between the clients is a number rather
  than a ceiling, and it is worth the many thousand round trips the main report
  spends on it rather than the few hundred a swept row can afford.
- The message is 1,024 bytes at every delay, so the one thing changing down the
  table is the delay.
- The round trip is not the delay. A thread that asks to sleep for a millisecond
  is woken late, on a shared machine by more than a millisecond, so a row's round
  trip is longer than the delay it was asked for - and by an amount that is the
  machine's, not the language's. That is why the difference column compares the two
  clients against each other rather than either one against the delay. Both clients
  meet the same server with the same overrun; the only thing that differs is the
  language the client was written in.

## What this does not say

A `time.Sleep` is not a network. A real link has jitter, bandwidth that falls under
load, and a delay that is not the same in both directions; this has a fixed pause
before each reply and nothing else. What it reproduces is the one property that
decides whether the interpreter matters -- a round trip that costs time the
interpreter is not spending -- and it does not reproduce the rest.

## How to reproduce

```sh
make report
```

## The machine this ran on

    go 1.24.4 · darwin/arm64 · 10 CPUs · commit 9fca157 · 2026-10-12 04:50 UTC, median of 3 runs · run by hand with make report

## Mismatches

Every reply matched its request byte for byte, at every delay, for both clients.
