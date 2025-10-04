// ---------------------------------------------------------------
// A tour of CCWIG. Every feature of the language appears here.
// ---------------------------------------------------------------

/* Comments come in two forms: a // line comment,
   and a block comment like this one. */

print("--- literals ---");
print(42);        // integer
print(-7);
print(3.5);       // float
print("text");    // string
print(true);      // boolean
print(null);      // null

print("--- strings ---");
print("a quote: \"");
print("a backslash: \\");
print("two lines:\nsecond line");
print("join" + "ed" + " " + "up");

print("--- arithmetic ---");
print(1 + 2 * 3 - 4 / 2);   // precedence and integer division
print(7 / 2);              // integer division truncates: 3
print(7 % 2);              // modulo
print(1.5 + 2.5);          // floats print without a trailing .0
print(2 < 3 && 3 <= 3);    // comparison and short-circuit and
print("a" < "b" || false); // strings compare lexicographically
print(!false);
print(-(3 + 4));

print("--- variables and scope ---");
let a = 1;
a = a + 1;                 // assignment updates the enclosing scope
print(a);
if true {
	let a = 100;           // let is block scoped: this is a new variable
	print(a);
}
print(a);

print("--- conditionals ---");
fn classify(n) {
	if n < 0 {
		return "negative";
	} else if n == 0 {
		return "zero";
	} else {
		return "positive";
	}
}
print(classify(-5));
print(classify(0));
print(classify(5));

print("--- loops ---");
let total = 0;
for let i = 1; i <= 5; i = i + 1 {
	total = total + i;
}
print(total);

let countdown = 3;
while countdown > 0 {
	print(countdown);
	countdown = countdown - 1;
}

print("--- functions ---");
fn add(x, y) { return x + y; }
print(add(2, 3));
print(add(add(1, 1), add(2, 2)));

let square = fn(n) { n * n; };   // anonymous function, implicit return
print(square(7));

fn fact(n) {
	if n <= 1 { return 1; }
	return n * fact(n - 1);
}
print(fact(6));

print("--- closures ---");
fn makeAdder(x) {
	return fn(y) { x + y; };
}
let addTen = makeAdder(10);
print(addTen(5));

print("--- arrays ---");
let xs = [1, 2, 3];
print(xs);
print(xs[0]);
print(xs[len(xs) - 1]);
print(xs[99]);            // reading past the end yields null, not a crash
xs[1] = 20;
print(xs);
print(push(xs, 4));       // push returns a copy
print(xs);                // so xs is unchanged
print(first(xs));
print(last(xs));
print(rest(xs));
print(len("hello"));

print("--- types ---");
print(type(1));
print(type(1.5));
print(type("s"));
print(type(true));
print(type(null));
print(type([]));
print(type(add));
print(type(len));

print("--- truthiness ---");
// Only false, null and zero are falsy. An empty string and an
// empty array are both truthy, as they are in C.
if "" { print("an empty string is truthy"); }
if [] { print("an empty array is truthy"); }
if 0 { print("never printed"); } else { print("zero is falsy"); }
