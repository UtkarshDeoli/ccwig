// An array index has to be an integer. Reading outside the array is fine and
// yields null, but indexing with a non-integer is a mistake worth reporting.
let xs = [1, 2, 3];
print(xs["one"]);
