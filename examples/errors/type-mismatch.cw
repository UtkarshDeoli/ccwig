// Adding a number to a string is an error; CCWIG never converts silently.
// Use str() to make the intent explicit.
let n = 1;
print(n + "2");
