// Recursion, string building with str(), and a counting for loop.
fn fib(n) {
	if n < 2 { return n; }
	return fib(n - 1) + fib(n - 2);
}

for let i = 0; i < 12; i = i + 1 {
	print("fib(" + str(i) + ") = " + str(fib(i)));
}
