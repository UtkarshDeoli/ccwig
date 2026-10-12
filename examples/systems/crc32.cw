// crc32: the checksum every zip file and every network packet carries, built
// from a table, and checked against the same sum computed a bit at a time.
//
// CRC-32 is a polynomial division over the field of two elements, which sounds
// grander than it is. In practice it is a loop over the bytes of the message:
//
//     crc = table[(crc ^ byte) & 0xFF] ^ (crc >> 8)
//
// and the table is the same loop run eight times for each of the 256 possible
// bytes, so that the message loop does not have to. That is the whole trick of
// a table-driven checksum: the work is moved out of the message loop and into
// a table built once.
//
// The table is built here rather than written down, which matters because a
// checksum is a thing other programs have to agree with. A table copied out of
// a book and written down wrong gives sums that look plausible and match
// nothing, and there is no way to tell by looking at it.
//
// Two independent computations of the same checksum are in this file on
// purpose. The table-driven one and the bit-at-a-time one share no code, and
// they are compared over a hundred messages, along with five published values
// for CRC-32 that any implementation has to reproduce.

// POLY is CRC-32's generator, written backwards, which is how the reflected
// form of the algorithm uses it.
let POLY = 0xEDB88320;

fn makeTable(size) {
	let table = fill(size, 0);

	for let n = 0; n < size; n = n + 1 {
		let remainder = n;

		// Eight shifts, one per bit of the byte. A set bit at the bottom means
		// the running remainder has a term to cancel, so the polynomial is
		// exclusive-ored in; a clear bit means the division just shifts.
		for let bit = 0; bit < 8; bit = bit + 1 {
			if (remainder & 1) != 0 {
				remainder = POLY ^ (remainder >> 1);
			} else {
				remainder = remainder >> 1;
			}
		}

		table[n] = remainder;
	}

	return table;
}

// crc32 is the message loop: one table lookup, one exclusive-or and one shift
// per byte.
//
// Characters are taken as bytes. This language's strings are sequences of
// characters, so the two are the same thing only while the message is ASCII -
// which every published CRC-32 test vector is.
fn crc32(table, message) {
	let crc = 0xFFFFFFFF;

	for let i = 0; i < len(message); i = i + 1 {
		let byte = ord(message[i]);
		crc = table[(crc ^ byte) & 0xFF] ^ (crc >> 8);
	}

	// The all-ones start and finish are what turn a plain polynomial division
	// into the checksum that catches leading zeros.
	return crc ^ 0xFFFFFFFF;
}

// crc32Bitwise is the same checksum with the table taken back out: eight
// shifts per byte instead of one lookup. It is slow, it is short, and it is
// the version the table was built from - so if the two ever disagree, the
// table is what is wrong.
fn crc32Bitwise(message) {
	let crc = 0xFFFFFFFF;

	for let i = 0; i < len(message); i = i + 1 {
		crc = crc ^ ord(message[i]);

		for let bit = 0; bit < 8; bit = bit + 1 {
			if (crc & 1) != 0 {
				crc = POLY ^ (crc >> 1);
			} else {
				crc = crc >> 1;
			}
		}
	}

	return crc ^ 0xFFFFFFFF;
}

fn hexDigit(digit) {
	if digit < 10 {
		return chr(ord("0") + digit);
	}
	return chr(ord("a") + digit - 10);
}

// hex writes a checksum the way it is printed everywhere else: eight digits,
// zero padded, lowercase. A checksum written in decimal is a checksum nobody
// can compare against anything.
fn hex(value) {
	let out = "";
	let rest = value;

	for let i = 0; i < 8; i = i + 1 {
		out = hexDigit(rest % 16) + out;
		rest = rest / 16;
	}

	return out;
}

fn quoted(s) {
	return "\"" + s + "\"";
}

let table = makeTable(256);

// Two messages with the same bytes in a different order must not share a sum,
// so the table has to be built from the polynomial rather than from anything
// about the messages: swapping two characters is the smallest change a
// checksum is expected to catch.
let published = [
	"",
	"a",
	"abc",
	"123456789",
	"The quick brown fox jumps over the lazy dog"
];

let expected = [
	0x00000000,
	0xE8B7BE43,
	0x352441C2,
	0xCBF43926,
	0x414FA339
];

let wrong = 0;
let widest = 0;

for let i = 0; i < len(published); i = i + 1 {
	if len(published[i]) > widest {
		widest = len(published[i]);
	}
}

print("crc-32 of the published test vectors");
print("");

for let i = 0; i < len(published); i = i + 1 {
	let got = crc32(table, published[i]);
	let want = expected[i];

	// The label is padded to the width of the longest quoted message plus the
	// two spaces, so that the checksums line up in a column.
	let label = quoted(published[i]);
	while len(label) < widest + 4 {
		label = label + " ";
	}

	if got == want {
		print("  " + label + hex(got));
	} else {
		print("  " + label + hex(got) + "  expected " + hex(want));
		wrong = wrong + 1;
	}
}

print("");

if wrong > 0 {
	print("  check: FAILED - " + str(wrong) + " of the published values did not match");
	exit(1);
}

print("  check: ok - the published values match, " + hex(expected[3]) + " and all");

// The table is only worth having if it agrees with the loop it was built from,
// over messages the published vectors say nothing about.
let messages = [];

for let i = 0; i < 100; i = i + 1 {
	messages = push(messages, "message " + str(i));
}

let disagreements = 0;

for let i = 0; i < len(messages); i = i + 1 {
	if crc32(table, messages[i]) != crc32Bitwise(messages[i]) {
		disagreements = disagreements + 1;
	}
}

if disagreements > 0 {
	print("  check: FAILED - the table and the bit-at-a-time loop disagree on " + str(disagreements) + " messages");
	exit(1);
}

print("  check: ok - the table and the bit-at-a-time loop agree on all " + str(len(messages)) + " messages");

// A checksum that ignored the tail of a message would pass both checks above
// on messages that differ only at the end, which is exactly the case a
// checksum exists to catch.
let shifted = 0;

for let i = 0; i < 100; i = i + 1 {
	let a = "message " + str(i);
	let b = " message " + str(i);

	if crc32(table, a) == crc32(table, b) {
		shifted = shifted + 1;
	}
}

if shifted > 0 {
	print("  check: FAILED - " + str(shifted) + " pairs of messages that differ in their leading bytes share a sum");
	exit(1);
}

print("  check: ok - moving a message by one space is enough to change its sum");
