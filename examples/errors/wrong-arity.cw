// Calling a function with the wrong number of arguments is caught before the
// body runs, rather than leaving the missing parameter as null.
fn add(a, b) { return a + b; }
print(add(1));
