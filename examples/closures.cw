// Closures: a function that returns a function, capturing the scope it was
// defined in. Each counter keeps its own private state.
fn makeCounter(start) {
	let n = start;
	return fn() {
		n = n + 1;
		return n;
	};
}

let next = makeCounter(10);
print(next());
print(next());

// A second counter does not share state with the first.
let other = makeCounter(100);
print(other());
print(next());

// Closures are the usual way to make a partially applied function.
fn makeMultiplier(factor) {
	return fn(x) { x * factor; };
}

let triple = makeMultiplier(3);
let tenTimes = makeMultiplier(10);
print(triple(5));
print(tenTimes(5));
