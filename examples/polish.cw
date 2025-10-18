// polish.cw — a calculator for prefix arithmetic.
//
// The operator comes first and the expressions it works on follow it:
//
//     * + 3 4 2      is (3 + 4) * 2
//     / - 10 2 3     is (10 - 2) / 3
//
// A number is an expression all by itself. An operator consumes the two
// expressions that follow it, which makes reading a program a recursive
// walk over the token list — exactly the shape `first` gives you the front
// of, and `rest` gives you everything but the front.

let OPERATORS = ["+", "-", "*", "/"];

fn isOperator(v) {
    if type(v) != "STRING" { return false; }

    for let i = 0; i < len(OPERATORS); i = i + 1 {
        if OPERATORS[i] == v { return true; }
    }

    return false;
}

fn apply(op, a, b) {
    if op == "+" { return a + b; }
    if op == "-" { return a - b; }
    if op == "*" { return a * b; }
    if op == "/" { return a / b; }
    return null;
}

// step() reads one whole expression off the front of `tokens` and returns a
// triple: whether it worked, what it came to, and the tokens it did not use.
//
// `note` is called once for every operation. Passing a function in as an
// argument is how the caller decides whether the working is worth showing.
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

// evaluate() walks the program and then checks that nothing was left over.
// It answers with a pair: [true, value] or [false, message].
fn evaluate(program, note) {
    let result = step(program, note);
    if !result[0] { return [false, result[1]]; }

    if len(result[2]) != 0 {
        return [false, "tokens left over: " + str(result[2])];
    }

    return [true, result[1]];
}

// trace() is a note that prints each operation as it happens. Passing it to
// evaluate() is what makes the working visible; passing a function that does
// nothing would run the same calculation in silence.
fn trace(op, a, b, value) {
    print("    " + str(a) + " " + op + " " + str(b) + " = " + str(value));
}

fn show(program) {
    print("--- " + str(program) + " ---");

    let result = evaluate(program, trace);

    if result[0] {
        print("  = " + str(result[1]));
    } else {
        print("  error: " + result[1]);
    }
}

show(["+", 3, 4]);            // 3 + 4
show(["*", "+", 3, 4, 2]);   // (3 + 4) * 2
show(["/", "-", 10, 2, 3]);  // (10 - 2) / 3
show(["+", 1, "*", 2, 3]);   // 1 + (2 * 3)
show(["/", 7, 2]);           // integer division truncates: 3
show(["/", 7.5, 2]);         // a float anywhere makes the arithmetic float
show(["-", 5]);              // an operand is missing
show(["+", 1, 2, 3]);        // a token goes unused
show(["/", 1, 0]);           // division by zero is reported, not fatal
