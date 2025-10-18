# Getting started with CCWIG

This is a hands-on introduction. It assumes you have programmed a little in
some language and have never seen CCWIG before. By the end you will have run a
dozen small programs and read one long one.

[docs/LANGUAGE.md](LANGUAGE.md) is the reference to keep open beside this. When
the two disagree, the reference is right and this file is a bug.

## Build it

```sh
git clone https://github.com/UtkarshDeoli/ccwig
cd ccwig
make build
```

That leaves a binary at `bin/ccwig`. There is nothing to install and no
dependencies to fetch — the module imports only the Go standard library.

If you would rather not build anything, `go run ./cmd/ccwig` is a drop-in
replacement for `./bin/ccwig` everywhere below.

## The smallest program

Save this as `hello.cw`:

```c
print("Hello, world!");
```

and run it:

```sh
./bin/ccwig hello.cw
```

```
Hello, world!
```

For one-liners, `-e` skips the file entirely:

```sh
./bin/ccwig -e 'print("Hello, world!");'
```

Run `./bin/ccwig` with no arguments at all and you get a prompt instead, where
statements are evaluated as soon as you finish typing them:

```
>> 1 + 2
3
>> let name = "CCWIG";
>> print("hello, " + name);
hello, CCWIG
```

The rest of this guide uses `-e` and short files interchangeably.

## Values

There are eight types, and you can ask any value what it is:

```c
print(type(1));       // INTEGER
print(type(1.5));     // FLOAT
print(type("hi"));    // STRING
print(type(true));    // BOOLEAN
print(type(null));    // NULL
print(type([]));      // ARRAY
print(type(add));     // FUNCTION — a function you wrote
print(type(len));     // BUILTIN — one the interpreter provides
```

Integers are 64 bits wide and signed. Floats are 64-bit IEEE-754. Strings are
counted by character rather than by byte, so text in any alphabet behaves the
way you would expect.

Arithmetic follows C, with one deliberate exception: **nothing is converted
for you.**

```c
print(1 + 2 * 3);      // 7
print(7 / 2);          // 3 — integer division truncates
print(1 + 2.5);        // 3.5 — a float makes the whole expression float
print("one" + " " + "two");   // one two
```

Mixing things that do not mix is an error rather than a silent surprise:

```c
print(1 + "2");
```

```
<command line>:1:9: type mismatch: INTEGER + STRING
```

To join a number to a string, say so with `str()`:

```c
print(str(42) + "!");   // 42!
```

This one rule removes a whole family of bugs. When you see a string built, you
know every conversion in it was intended.

Whole floats print without a fractional part, so `1.0 + 3.0` prints as `4`.
`type()` is how you tell `4` from `4.0`.

## Variables and scope

`let` creates a variable. `=` assigns to the nearest one that already exists.

```c
let x = 5;
x = x + 1;
print(x);      // 6
```

The distinction between the two is what scope means here. Braces make a new
scope, so a `let` inside them belongs to them:

```c
let a = 1;
if true {
    let a = 2;   // a new variable; the outer one is untouched
    print(a);    // 2
}
print(a);        // 1
```

Assignment works the other way — it reaches outward until it finds the name.
That is what makes a loop counter survive its own body:

```c
let i = 0;
while i < 5 {
    i = i + 1;
}
print(i);       // 5
```

## Decisions

`if` looks like C, and `else if` chains the way you would hope:

```c
fn classify(n) {
    if n < 0 {
        return "negative";
    } else if n == 0 {
        return "zero";
    } else {
        return "positive";
    }
}
print(classify(-5));   // negative
print(classify(0));    // zero
print(classify(5));    // positive
```

Any value can be a condition. Following C, only `false`, `null`, `0` and `0.0`
are falsy. The empty string `""` and the empty array `[]` are **truthy** — a
detail worth remembering, because it is the opposite of what Python would do.

`if` is also an expression, so it can produce a value:

```c
fn sign(n) {
    return if n < 0 { "negative" } else { "non-negative" };
}
print(sign(-5));   // negative
print(sign(5));    // non-negative
```

## Loops

`while` checks its condition before each pass. `for` is the C three-clause
form, minus the types:

```c
let total = 0;
for let i = 1; i <= 5; i = i + 1 {
    total = total + i;
}
print(total);   // 15
```

Every clause is optional. `for ;; { }` loops forever, which the interpreter
will stop for you rather than hanging — see [Limits](LANGUAGE.md#limits).

There is no `break` and no `continue`. A loop ends because its condition says
so.

## Functions

```c
fn add(a, b) {
    return a + b;
}
print(add(2, 3));   // 5
```

`fn name(...)` is shorthand for `let name = fn(...) {...};`. Either form may
call itself, so recursion needs no ceremony.

If you leave off `return`, a function gives back the value of its last
expression. Small functions therefore often read as pure expressions:

```c
fn square(n) { n * n; }
print(square(7));   // 49
```

Calling a function with the wrong number of arguments is an error, not a
`null`. Functions are ordinary values, so they can be passed around:

```c
let apply = fn(f, x) { f(x); };
print(apply(fn(n) { n * 2; }, 21));   // 42
```

### Closures

A function keeps the scope it was written in, and shares that scope with any
other function written in the same place. The scope lives as long as something
still refers to it.

```c
fn makeCounter() {
    let n = 0;
    return fn() { n = n + 1; return n; };
}

let a = makeCounter();
let b = makeCounter();

a();
a();
print(b());   // 1 — b has its own n
print(a());   // 3
```

Two calls to `makeCounter()` produce two independent counters. This is the
usual way to keep state private in CCWIG, since there are no objects.

## Arrays

Arrays are ordered and mutable, indexed from zero:

```c
let xs = [1, 2, 3];
print(xs[0]);    // 1
print(xs[99]);   // null — reading past the end is not an error
xs[1] = 20;
print(xs);       // [1, 20, 3]
```

Reading outside an array gives you `null`, which makes a bounds check
unnecessary when all you want is "the value, or nothing". Writing outside one
is an error, because there is no element to write to.

`push` returns a **new** array rather than changing the one you gave it:

```c
print(push(xs, 4));   // [1, 20, 3, 4]
print(xs);            // [1, 20, 3] — unchanged
```

That is what makes it safe to build an array inside a loop. The other way
round, assigning to an element *does* change the caller's array, since arrays
are passed by reference.

Four builtins walk an array from the front. They are the vocabulary you will
reach for most:

```c
print(len(xs));     // 3
print(first(xs));   // 1
print(last(xs));    // 3
print(rest(xs));    // [20, 3] — everything but the first
print(rest([1]));   // []
print(rest([]));    // null
```

`fill(n, v)` makes an array of `n` copies of `v`:

```c
let row = fill(3, 0);   // [0, 0, 0]
```

It repeats the value rather than copying it, and that catches people out the
first time they build a grid. This does **not** make three separate rows:

```c
let grid = fill(3, row);
grid[0][0] = 9;
print(grid);            // [[9, 0, 0], [9, 0, 0], [9, 0, 0]]
```

`row` is one array, and all three slots hold it. To get rows that are really
separate you need a fresh `fill` for each one:

```c
fn grid(rows, cols) {
    let out = [];
    for let i = 0; i < rows; i = i + 1 {
        out = push(out, fill(cols, 0));
    }
    return out;
}

let g = grid(2, 3);
g[0][0] = 9;
print(g);              // [[9, 0, 0], [0, 0, 0]]
```

`fill` exists for the speed: `push` copies the whole array on every call, so
building a large array one element at a time copies quadratically. `fill`
allocates once. For small arrays the difference does not matter, and `push` is
the clearer way to say what you mean.

## A worked example

Let us write something real: a calculator for prefix arithmetic. (Prefix, not
the postfix of an old HP calculator — the operator comes first, and the two
expressions it works on follow it.)

```
* + 3 4 2       is (3 + 4) * 2
/ - 10 2 3      is (10 - 2) / 3
```

A program is an array of tokens. A number is a complete expression all by
itself; an operator swallows the two expressions after it. That definition is
recursive, and so is the code — which is why `first` and `rest` exist.

The finished program is [`examples/polish.cw`](../examples/polish.cw). We will
build it in five steps.

### Step 1: the vocabulary

Start with the operators and a way to recognise one. Since tokens are a mix of
numbers and strings, `type()` tells them apart:

```c
let OPERATORS = ["+", "-", "*", "/"];

fn isOperator(v) {
    if type(v) != "STRING" { return false; }

    for let i = 0; i < len(OPERATORS); i = i + 1 {
        if OPERATORS[i] == v { return true; }
    }

    return false;
}

print(isOperator("+"));   // true
print(isOperator(3));     // false
print(isOperator("("));   // false
```

Doing the arithmetic is a small dispatch on the operator:

```c
fn apply(op, a, b) {
    if op == "+" { return a + b; }
    if op == "-" { return a - b; }
    if op == "*" { return a * b; }
    if op == "/" { return a / b; }
    return null;
}
```

### Step 2: reading one expression

`step` takes a list of tokens and reads a single expression off the front. It
returns three things: whether it worked, what the expression came to, and the
tokens it did not touch. A triple in an array is the idiom — there are no
structs to reach for.

It takes a second argument too, a `note` function, which we will put to use in
Step 4. For now, hand it one that does nothing.

```c
fn step(tokens, note) {
    if len(tokens) == 0 {
        return [false, "the program stops in the middle of an expression", []];
    }

    let token = first(tokens);
    let todo = rest(tokens);

    if !isOperator(token) {
        return [true, token, todo];
    }

    let left = step(todo, note);
    if !left[0] { return left; }

    let right = step(left[2], note);
    if !right[0] { return right; }

    if token == "/" && right[1] == 0 {
        return [false, "division by zero", []];
    }

    let value = apply(token, left[1], right[1]);
    note(token, left[1], right[1], value);

    return [true, value, right[2]];
}

let quiet = fn(op, a, b, value) { null; };

print(step(["+", 3, 4], quiet));         // [true, 7, []]
print(step(["*", "+", 3, 4, 2], quiet)); // [true, 14, []]
```

Read the operator branch twice, because the order is the whole trick.
`right[2]` — the tokens *right* did not use — is what `step` hands back to its
caller. Errors travel the same way as values: `if !left[0] { return left; }`
passes a failure straight up the recursion without a special mechanism.

The check for division by zero is deliberate. Dividing by zero is a runtime
error in CCWIG, and a runtime error stops the program where it stands. That is
the right call in general and the wrong one for a calculator that has been
asked to divide by zero on purpose, so the calculator catches it first and
reports it like any other bad program.

### Step 3: checking there is nothing left over

A program is one expression, so anything still unused afterwards is a mistake:

```c
fn evaluate(program, note) {
    let result = step(program, note);
    if !result[0] { return [false, result[1]]; }

    if len(result[2]) != 0 {
        return [false, "tokens left over: " + str(result[2])];
    }

    return [true, result[1]];
}

print(evaluate(["+", 1, "*", 2, 3], quiet));   // [true, 7]
print(evaluate(["+", 1, 2, 3], quiet));        // [false, tokens left over: [3]]
```

### Step 4: showing the working

`note` has been riding along since Step 2 doing nothing. It is there so that
printing the working stays the caller's decision — a `step` that printed would
be a `step` nobody could use quietly.

Hand it a note that prints, and every operation is announced as it happens:

```c
// trace() is a note that prints each operation as it happens.
fn trace(op, a, b, value) {
    print("    " + str(a) + " " + op + " " + str(b) + " = " + str(value));
}
```

Neither `step` nor `evaluate` changes by a character. That is the argument for
taking the function as an argument rather than deciding for yourself.

### Step 5: running the things

```c
fn show(program) {
    print("--- " + str(program) + " ---");

    let result = evaluate(program, trace);

    if result[0] {
        print("  = " + str(result[1]));
    } else {
        print("  error: " + result[1]);
    }
}

show(["+", 3, 4]);
show(["*", "+", 3, 4, 2]);
show(["/", "-", 10, 2, 3]);
show(["/", 7, 2]);
show(["/", 7.5, 2]);
show(["-", 5]);
show(["+", 1, 2, 3]);
show(["/", 1, 0]);
```

Running it gives:

```
--- [+, 3, 4] ---
    3 + 4 = 7
  = 7
--- [*, +, 3, 4, 2] ---
    3 + 4 = 7
    7 * 2 = 14
  = 14
--- [/, -, 10, 2, 3] ---
    10 - 2 = 8
    8 / 3 = 2
  = 2
--- [/, 7, 2] ---
    7 / 2 = 3
  = 3
--- [/, 7.5, 2] ---
    7.5 / 2 = 3.75
  = 3.75
--- [-, 5] ---
  error: the program stops in the middle of an expression
--- [+, 1, 2, 3] ---
  error: tokens left over: [3]
--- [/, 1, 0] ---
  error: division by zero
```

The last three are the point of the error handling. A malformed program
produces a message, not a crash, and the division by zero is caught before it
happens — the interpreter would have stopped the program there, which is the
right thing for a real error and the wrong thing for a calculator that is
asked to divide by zero on purpose.

## Where to go next

Read [`examples/lang.cw`](../examples/lang.cw) with its output beside it. It is
a tour of every feature in one file, and it doubles as a test.

After that, in rough order of difficulty:

| Program | What it shows |
| --- | --- |
| [`fib.cw`](../examples/fib.cw) | Recursion and string building |
| [`sorting.cw`](../examples/sorting.cw) | Nested loops, and arrays changing under you |
| [`fizzbuzz.cw`](../examples/fizzbuzz.cw) | The classic, with `%` |
| [`closures.cw`](../examples/closures.cw) | Counters, callbacks, state that stays put |
| [`polish.cw`](../examples/polish.cw) | This guide's calculator, complete |
| [`maze.cw`](../examples/maze.cw) | A recursive maze carver and two solvers |

`examples/errors/` is the other half of the story: seven short programs that
each fail in a specific way, so you can see what the messages look like before
you meet them for real.

When you want the details — precedence, the grammar, the exact behaviour of
every builtin — [docs/LANGUAGE.md](LANGUAGE.md) has them.
