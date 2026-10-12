// bitset: keep half a million flags in a fraction of the memory, and sieve
// the primes with them.
//
// The idea is the one every systems programmer reaches for: a flag needs one
// bit, and a machine hands out memory in 64-bit words, so sixty-four flags
// share a word and an index tells you which word and which bit within it.
//
//     word = index / 64
//     bit  = index % 64
//
// That is all a bitset is. What makes it worth writing out is that the
// arithmetic is the interesting part and the storage is an array of plain
// integers - no library, no structure, nothing this language has to provide
// beyond `&`, `|`, `<<` and `>>`.

// WORD_BITS is how many flags fit in one integer. This language's integers are
// 64 bits wide, so a shift of up to 63 is well defined and 64 is not.
let WORD_BITS = 64;

// Half a million is as far as a tree-walking interpreter gets through a sieve
// inside its step limit. The arithmetic below is the same at any size, and the
// report prints what a million flags would cost.
let LIMIT = 500000;

fn wordsNeeded(flags) {
	// Round up to a whole number of words. `flags / 64` alone would round down
	// and lose the last word whenever the count is not a multiple of 64.
	return (flags + WORD_BITS - 1) / WORD_BITS;
}

fn setBit(bits, index) {
	bits[index / WORD_BITS] = bits[index / WORD_BITS] | (1 << (index % WORD_BITS));
}

fn testBit(bits, index) {
	return (bits[index / WORD_BITS] & (1 << (index % WORD_BITS))) != 0;
}

// popcount counts the set bits in one word with the trick that `x & (x - 1)`
// clears the lowest set bit: the loop runs once per set bit rather than once
// per bit, which matters when most of them are clear.
fn popcount(x) {
	let count = 0;

	while x != 0 {
		count = count + 1;
		x = x & (x - 1);
	}

	return count;
}

fn countSet(bits) {
	let total = 0;

	for let i = 0; i < len(bits); i = i + 1 {
		total = total + popcount(bits[i]);
	}

	return total;
}

// isPrimeByTrialDivision is the slow, obvious way to decide the same question,
// and the reason it is here is that it shares no working with the sieve. Two
// programs that disagree are worth more than one that agrees with itself.
fn isPrimeByTrialDivision(n) {
	if n < 2 {
		return false;
	}

	let divisor = 2;

	while divisor * divisor <= n {
		if n % divisor == 0 {
			return false;
		}
		divisor = divisor + 1;
	}

	return true;
}

let considered = LIMIT + 1;
let bits = fill(wordsNeeded(considered), 0);
let words = len(bits);

// The sieve: a number is prime until a smaller prime says otherwise, and the
// smallest multiple of p that is not already struck out is p * p.
setBit(bits, 0);
setBit(bits, 1);

for let p = 2; p * p <= LIMIT; p = p + 1 {
	if !testBit(bits, p) {
		for let multiple = p * p; multiple <= LIMIT; multiple = multiple + p {
			// This line is setBit written out. The call costs more than the bit
			// arithmetic it wraps, and this loop runs a few million times.
			bits[multiple / WORD_BITS] = bits[multiple / WORD_BITS] | (1 << (multiple % WORD_BITS));
		}
	}
}

// Counting the struck-out flags counts the composites, and everything else is
// prime - which is a fact about the bitset rather than about arithmetic, so
// the real check is further down.
let marked = countSet(bits);
let primes = considered - marked;

// The largest prime is found from the top, which is where the one a reader
// cannot guess lives.
let largest = LIMIT;
while testBit(bits, largest) {
	largest = largest - 1;
}

// The first ten primes are the ones that fit on one line and can be checked
// by eye, which the rest of these numbers cannot.
let first = [];
let candidate = 0;

while len(first) < 10 {
	if candidate < considered && !testBit(bits, candidate) {
		first = push(first, candidate);
	}
	candidate = candidate + 1;
}

print("sieve of eratosthenes to " + str(LIMIT));
print("");
print("  numbers considered     " + str(considered));
print("  primes                 " + str(primes));
print("  struck out             " + str(marked));
print("  largest prime          " + str(largest));
print("  first ten primes       " + str(first));
print("");
print("  a bitset stores " + str(WORD_BITS) + " flags in an integer");
print("  this sieve uses        " + str(words) + " integers (" + str(words * 8) + " bytes)");
print("  one flag per number    " + str(considered) + " values, a byte each");
print("  a million flags would  " + str(wordsNeeded(1000001) * 8) + " bytes");
print("");

// The check that matters: decide primality 500 times the slow way and see
// whether the bitset told the same story.
let disagreements = 0;

for let n = 0; n < 500; n = n + 1 {
	if testBit(bits, n) == isPrimeByTrialDivision(n) {
		disagreements = disagreements + 1;
	}
}

if disagreements > 0 {
	print("  check: FAILED - the sieve and trial division disagree about " + str(disagreements) + " of the first 500 numbers");
	exit(1);
}

print("  check: ok - the sieve and trial division agree on the first 500 numbers");
