// hashtable: an open-addressing hash table, and the probing that makes one
// work.
//
// The table is two parallel arrays and no buckets: a key hashes to a slot, and
// if that slot is taken the search walks forward one slot at a time until it
// finds the key or an empty slot. Nothing is chained, nothing is allocated,
// and the whole thing is two `fill` calls.
//
// The hash is FNV-1a, which is four lines and is what most hash tables in the
// wild actually use. It is built out of the operators a hash needs - xor,
// multiply, and a mask - and the mask is there because a 32-bit hash computed
// in 64-bit integers has to be cut back down to 32 bits after every multiply,
// or the arithmetic stops being the hash.
//
// Deletion is the interesting omission. An empty slot is what ends a probe, so
// clearing a slot would break the search for every key that probed past it.
// Real tables solve this with tombstones, which is a story for another
// example; this one fills the table and never takes anything out.
//
// The keys here are `key0` through `key199`, which is not what a real workload
// looks like: they differ in one or two characters, so half of them want a
// slot another key already took, and linear probing then piles the overflow
// into runs. The long probes the report prints are that pile-up, not a broken
// hash - running the same arithmetic over the same keys in another language
// gives the same average to three decimal places, which is the only way to
// tell the two apart.

let SIZE = 256;
let KEYS = 200;

fn hash(s) {
	let h = 2166136261;

	for let i = 0; i < len(s); i = i + 1 {
		h = h ^ ord(s[i]);
		h = h * 16777619;
		// Cut back to 32 bits. Without this the value grows a word per
		// character, and the low bits that pick the slot stop being a hash.
		h = h & 0xFFFFFFFF;
	}

	return h;
}

// insert returns how many slots it had to look at, or -1 if the table was
// full. The count is the cost of the insert, and averaging it is how a hash
// table's behaviour is measured.
fn insert(keys, values, key, value) {
	let start = hash(key) % len(keys);
	let probes = 0;

	while probes < len(keys) {
		let slot = (start + probes) % len(keys);

		if keys[slot] == null {
			keys[slot] = key;
			values[slot] = value;
			return probes + 1;
		}

		// The same key twice replaces rather than inserting a second copy,
		// which is what makes a table a mapping and not a list.
		if keys[slot] == key {
			values[slot] = value;
			return probes + 1;
		}

		probes = probes + 1;
	}

	return -1;
}

// lookup stops at the first empty slot, which is sound only because nothing
// is ever deleted: a key that probed past a slot means that slot was occupied
// when it was inserted.
fn lookup(keys, values, key) {
	let start = hash(key) % len(keys);
	let probes = 0;

	while probes < len(keys) {
		let slot = (start + probes) % len(keys);

		if keys[slot] == null {
			return null;
		}
		if keys[slot] == key {
			return values[slot];
		}

		probes = probes + 1;
	}

	return null;
}

// occupied counts the slots holding a key, which is the table's own account of
// how full it is.
fn occupied(keys) {
	let count = 0;

	for let i = 0; i < len(keys); i = i + 1 {
		if keys[i] != null {
			count = count + 1;
		}
	}

	return count;
}

let keys = fill(SIZE, null);
let values = fill(SIZE, 0);

let totalProbes = 0;
let longestProbe = 0;
let collisions = 0;

for let i = 0; i < KEYS; i = i + 1 {
	let taken = insert(keys, values, "key" + str(i), i * i);

	if taken < 0 {
		print("the table filled up at key " + str(i));
		exit(1);
	}

	totalProbes = totalProbes + taken;

	// A probe past the first slot is a collision: another key already wanted
	// that slot. Counting them says how much the hash is doing its job.
	if taken > 1 {
		collisions = collisions + 1;
	}
	if taken > longestProbe {
		longestProbe = taken;
	}
}

print("an open-addressing table of " + str(SIZE) + " slots holding " + str(KEYS) + " keys");
print("");
print("  slots occupied       " + str(occupied(keys)));
print("  load factor          " + str(occupied(keys) * 100 / SIZE) + "%");
print("  keys that collided   " + str(collisions));
print("  longest probe        " + str(longestProbe) + " slots");
print("  average probe        " + str(totalProbes * 1.0 / KEYS) + " slots");
print("");

// The table has to give back what was put in. The value inserted for key i is
// i * i, which is worked out here rather than read from the table, so an
// answer that survives the round trip through hashing and probing is the
// answer that went in.
let wrong = 0;

for let i = 0; i < KEYS; i = i + 1 {
	if lookup(keys, values, "key" + str(i)) != i * i {
		wrong = wrong + 1;
	}
}

if wrong > 0 {
	print("  check: FAILED - " + str(wrong) + " keys did not come back with the value they went in with");
	exit(1);
}

print("  check: ok - every key came back with the value it went in with");

// A key that was never inserted must not be found, and a hundred of them are
// worth trying: a hash table that answers yes to everything would pass the
// check above.
let found = 0;

for let i = KEYS; i < KEYS * 2; i = i + 1 {
	if lookup(keys, values, "key" + str(i)) != null {
		found = found + 1;
	}
}

if found > 0 {
	print("  check: FAILED - " + str(found) + " keys that were never inserted were found anyway");
	exit(1);
}

print("  check: ok - " + str(KEYS) + " keys that were never inserted were not found");

// Inserting the same key twice must replace, not duplicate, or a table used as
// a map would grow without bound as it was corrected.
let before = occupied(keys);
insert(keys, values, "key0", 12345);

if occupied(keys) != before || lookup(keys, values, "key0") != 12345 {
	print("  check: FAILED - re-inserting a key did not replace its value");
	exit(1);
}

print("  check: ok - re-inserting a key replaced it in place");
