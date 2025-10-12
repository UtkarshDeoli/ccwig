# The CCWIG language

CCWIG is a small C-like language. This document is the reference for
everything it can do; `examples/lang.cw` is a runnable tour of the same
material.

## Running programs

```sh
ccwig program.cw            # run a file
ccwig                       # interactive session
ccwig -e 'print("hi");'     # run source from the command line
ccwig -tokens -e '1 + 2;'   # show the token stream
ccwig -ast -e '1 + 2 * 3;'  # show the parsed syntax tree
```

A program is a sequence of statements. Statements are separated by
newlines; semicolons are allowed and ignored, so C-style source reads
naturally.

## Lexical structure

| Kind | Examples |
| --- | --- |
| Integer | `42`, `0`, `-7` |
| Float | `3.5`, `0.25` |
| String | `"hello"`, `""` |
| Boolean | `true`, `false` |
| Null | `null` |
| Identifier | `x`, `total`, `foo_bar1` |
| Keywords | `let`, `fn`, `if`, `else`, `while`, `for`, `return`, `true`, `false`, `null` |

Identifiers start with a letter or `_` and continue with letters, digits or
`_`. Comments are `// to end of line` and `/* block comments */`, which do
not nest.

A string may contain these escapes: `\n`, `\t`, `\r`, `\"`, `\\` and `\0`.
A string may not span a line; a newline inside one is an error.

## Types

| Type | Notes |
| --- | --- |
| `INTEGER` | 64-bit signed |
| `FLOAT` | 64-bit IEEE-754 |
| `STRING` | A sequence of characters, counted by character and not by byte |
| `BOOLEAN` | `true` or `false` |
| `NULL` | The absence of a value |
| `ARRAY` | An ordered, mutable sequence |
| `FUNCTION` | A user-defined function, including its captured scope |
| `BUILTIN` | A function provided by the interpreter |

`type(x)` returns the name of a value's type as a string.

CCWIG is dynamically typed and does not convert silently. `1 + "2"` is an
error, not `"12"`; use `str()` to convert deliberately.

Numbers print in full decimal notation, never with an exponent: `1000000.0`
prints as `1000000`, and `0.0000001` as `0.0000001`. What `print` writes can
therefore be read back as the number it names, though a float near the bottom
of the range prints as a long run of zeros. A whole `FLOAT` is written without
a fractional part, so `1.0 + 3.0` prints as `4`; use `type(v)` if you need to
tell `4` from `4.0`.

## Operators

From tightest to loosest binding:

| Precedence | Operators | Associativity |
| --- | --- | --- |
| 1 (tightest) | `xs[i]` | left |
| 2 | `f(x)` | left |
| 3 | `-x`, `!x` | right |
| 4 | `*` `/` `%` | left |
| 5 | `+` `-` | left |
| 6 | `<` `>` `<=` `>=` | left |
| 7 | `==` `!=` | left |
| 8 | `&&` | left |
| 9 | `\|\|` | left |
| 10 (loosest) | `=` | right |

Notes:

- `+` adds two numbers or concatenates two strings.
- `/` between two integers truncates towards zero: `7 / 2` is `3`, and
  `-7 / 2` is `-3`.
- `%` is integer-only; `5.0 % 2` is an error.
- If either operand is a `FLOAT`, arithmetic promotes both to float:
  `1 + 2.5` is `3.5`.
- `<` `>` `<=` `>=` also work on strings, comparing lexicographically.
- `==` and `!=` are defined between every pair of types and never fail.
  Comparing different types yields `false`, so `x == null` is always safe.
  Numbers compare across `INTEGER` and `FLOAT`, so `1 == 1.0` is `true`.
  Strings compare by their text. Arrays and functions are compared by identity,
  not by contents, so two separately written `[1, 2]` arrays are not equal —
  `xs == xs` is true while `[1, 2] == [1, 2]` is false.
- `&&` and `||` short-circuit and yield a boolean. A guard such as
  `i < len(xs) && xs[i] != null` is therefore safe.

## Variables and scope

```c
let x = 5;      // declares x in the current scope
x = x + 1;      // assigns to the nearest existing x
```

`let` creates a binding in the current block. `=` updates the nearest
existing binding, searching outwards through enclosing blocks and functions;
if no binding exists it creates one in the current scope.

This distinction matters. Blocks (`if`, `while`, `for` bodies and bare `{}`)
each introduce a new scope, so:

```c
let a = 1;
if true {
    let a = 2;   // a new variable; the outer a is untouched
    print(a);    // 2
}
print(a);        // 1
```

while assignment reaches outwards, which is what makes loops work:

```c
let i = 0;
while i < 5 {
    i = i + 1;   // updates the i declared outside the loop
}
print(i);        // 5
```

## Truthiness

A condition may be any value. Following C, these are falsy:

- `false`
- `null`
- integer `0`
- float `0.0`

Everything else is truthy, **including** the empty string `""` and the empty
array `[]`.

## Conditionals

```c
if x > 5 {
    print("greater");
} else if x == 5 {
    print("equal");
} else {
    print("less");
}
```

An `if` is an expression: it produces the value of the branch that ran, or
`null` if no branch did.

```c
let label = if n < 0 { "negative" } else { "non-negative" };
```

## Loops

`while` re-checks its condition before each iteration:

```c
let i = 0;
while i < 10 {
    print(i);
    i = i + 1;
}
```

`for` has the C form. Every clause may be omitted; an omitted condition means
"loop forever".

```c
for let i = 0; i < 10; i = i + 1 {
    print(i);
}

for ; i < 10; i = i + 1 { }   // no init clause
for let i = 0; ; i = i + 1 { } // no condition
for let i = 0; i < 10; { i = i + 1; } // no post clause
```

A `let` written inside a loop body is re-created on every iteration, so it
does not accumulate.

## Functions

```c
fn add(a, b) {          // a named declaration
    return a + b;
}

let square = fn(n) { n * n; };   // an anonymous function value
```

A `fn name(...)` declaration is sugar for `let name = fn(...) {...};`. Either
form may be recursive.

Without an explicit `return`, a function returns the value of the last
expression in its body, or `null` for an empty body:

```c
fn square(n) { n * n; }
print(square(7));    // 49
```

Passing the wrong number of arguments is an error. Functions are values, so
they can be stored, passed and returned:

```c
let apply = fn(f, x) { f(x); };
print(apply(fn(n) { n * 2; }, 21));   // 42
```

### Closures

A function captures the scope it was defined in, and shares it with every
other function created there. That scope lives as long as something refers to
it:

```c
fn makeCounter() {
    let n = 0;                       // private to each counter
    return fn() { n = n + 1; return n; };
}

let a = makeCounter();
let b = makeCounter();
a(); a();
print(b());      // 1 — b does not share a's n
```

## Arrays

```c
let xs = [1, 2, 3];
print(xs[0]);          // 1
print(xs[99]);         // null — reading outside the array is not an error
xs[1] = 20;            // element assignment
print(len(xs));        // 3
```

Arrays are passed by reference, so a function that writes to an element
changes the caller's array. `push` returns a copy instead of mutating, which
makes it safe to build new arrays inside loops.

An index must be an integer. Negative or too-large indices read as `null`;
writing outside the array is an error, because there is nothing to write to.

## Built-in functions

| Call | Result |
| --- | --- |
| `print(a, b, ...)` | Writes its arguments, separated by spaces, followed by a newline. Returns `null`. |
| `len(s)` / `len(xs)` | Number of characters in a string, or elements in an array. |
| `push(xs, v)` | A new array with `v` appended. `xs` is not modified. |
| `first(xs)` | The first element, or `null` if empty. |
| `last(xs)` | The last element, or `null` if empty. |
| `rest(xs)` | A new array without the first element, or `null` if empty. |
| `type(v)` | The name of `v`'s type, as a string. |
| `str(v)` | `v` rendered as a string. |

Built-ins are ordinary values, so `len` can be stored and passed around.

## Errors

A program that cannot be parsed reports every syntax error it finds, with
positions, and never runs. A program that fails while running stops at the
first error and reports the file, line and column:

```
examples/errors/type-mismatch.cw:3:9: type mismatch: INTEGER + STRING
```

Runtime errors are values internally, but they halt the program: an error is
never silently ignored to let a later statement carry on. Output written
before the failure has already been delivered.

## Limits

A program that will not finish is stopped rather than left to hang or to take
the process down with it. Three limits do that, and each reports an ordinary
error naming the problem:

| Limit | Value | Catches |
| --- | --- | --- |
| Steps | 50,000,000 | `while true {}` and every other loop that never ends |
| Call depth | 10,000 | `fn f(n) { return f(n + 1); }` and other runaway recursion |
| Expression nesting | 1,000 | Source such as two million nested parentheses |

The three exist for different reasons, so none of them covers the others.

A loop is metered per iteration, not per node visited, because a loop can spin
without evaluating anything at all — `for ;; { }` has no condition to evaluate
and a body that evaluates nothing.

The call depth limit is what makes infinite recursion diagnosable. Without it
the interpreter exhausts the Go stack, and a stack overflow is a fatal error
that cannot be recovered from: the process dies with a runtime dump instead of
printing a message.

Expression nesting is bounded while parsing because parsing an expression
recurses once per level. A left-associative chain such as `1 + 1 + 1 + ...` is
unaffected however long it is, since it iterates rather than recurses.

The step and call depth limits are generous, so reaching either one almost
always means the program is wrong rather than merely large.

## The REPL

Running `ccwig` with no arguments starts an interactive session. Statements
are evaluated as soon as they are complete, and the value of an expression is
printed (statements that produce nothing, such as `let` and `print`, are not
echoed). If brackets are still open, input continues on the next line, so
functions and blocks can be typed over several lines. Errors are reported
without ending the session. `quit` or `exit` leaves.

```
>> fn add(a, b) {
..     return a + b;
.. }
>> add(1, 2);
3
```

## Differences from C

Worth knowing if you are coming from C:

- Semicolons are optional.
- Only `if`, `while` and `for` have the familiar brace syntax; there is no
  `switch`, `do`, `break`, `continue`, `goto`, `struct`, `typedef`, pointers,
  or integer types other than 64-bit.
- Numbers are not implicitly converted to each other on assignment, but
  `INTEGER` and `FLOAT` mix freely inside arithmetic.
- Strings are a built-in type, not `char *`.
- There is no `main` function; a program is just a list of statements.
- `%` does not accept floats, and division by zero is an error rather than
  undefined behaviour.
- Reading past the end of an array yields `null` instead of being undefined.

## Grammar

```
program        = { statement } ;
statement      = letStatement | returnStatement | whileStatement
               | forStatement | functionDeclaration | block
               | expressionStatement ;
letStatement   = "let" IDENT "=" expression ";" ;
returnStatement= "return" [ expression ] ";" ;
whileStatement = "while" expression block ;
forStatement   = "for" [ simpleStatement ] ";" [ expression ] ";"
                 [ simpleStatement ] block ;
functionDecl   = "fn" IDENT "(" [ params ] ")" block ;
block          = "{" { statement } "}" ;
expressionStmt = expression ";" ;

expression     = assignment ;
assignment     = IDENT | index "=" assignment | logicOr ;
logicOr        = logicAnd { "||" logicAnd } ;
logicAnd       = equality { "&&" equality } ;
equality       = comparison { ( "==" | "!=" ) comparison } ;
comparison     = sum { ( "<" | ">" | "<=" | ">=" ) sum } ;
sum            = product { ( "+" | "-" ) product } ;
product        = prefix { ( "*" | "/" | "%" ) prefix } ;
prefix         = ( "-" | "!" ) prefix | call ;
call           = index { "(" [ arguments ] ")" } ;
index          = primary { "[" expression "]" } ;
primary        = INT | FLOAT | STRING | "true" | "false" | "null"
               | IDENT | "(" expression ")" | functionLiteral
               | arrayLiteral | ifExpression ;
functionLiteral= "fn" "(" [ params ] ")" block ;
arrayLiteral   = "[" [ expression { "," expression } ] "]" ;
ifExpression   = "if" expression block
                 [ "else" ( block | ifExpression ) ] ;
```

Semicolons are shown above for clarity; they are optional in practice.
