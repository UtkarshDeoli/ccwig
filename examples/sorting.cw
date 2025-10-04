// Bubble sort. Exercises arrays, nested loops, element assignment and the
// fact that arrays are passed by reference: sort() mutates its argument.
fn sort(xs) {
	let n = len(xs);

	for let i = 0; i < n - 1; i = i + 1 {
		for let j = 0; j < n - 1 - i; j = j + 1 {
			if xs[j] > xs[j + 1] {
				let tmp = xs[j];
				xs[j] = xs[j + 1];
				xs[j + 1] = tmp;
			}
		}
	}

	return xs;
}

let numbers = [5, 3, 8, 1, 9, 2];
print("before: " + str(numbers));
sort(numbers);
print("after:  " + str(numbers));
