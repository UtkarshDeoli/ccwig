// malloc: a first-fit heap allocator, and the fragmentation it cannot
// prevent.
//
// The heap is an array of integers and every cell holds one of three things:
//
//     a positive number   a block header, whose value is how many payload
//                         cells follow it
//     0                   a free cell
//     -1                  a payload cell, belonging to the header before it
//
// That is enough to walk the heap from one end to the other and know what
// every cell is, which is what makes the check at the end possible: a heap you
// cannot walk is a heap you cannot prove anything about.
//
// The point of the example is not the allocator. It is what happens after
// `free`. The heap below ends up with plenty of free cells and nowhere to put
// a request that would fit in them, which is the bug every program that lives
// long enough eventually has, and which no amount of total-free-space counting
// can see.

let EMPTY = 0;
let PAYLOAD = -1;

// alloc finds the first run of `size + 1` free cells - one for the header and
// `size` for the payload - and takes it. It returns the index of the first
// payload cell, or -1 if no run is long enough.
fn alloc(heap, size) {
	let cells = len(heap);

	if size < 1 {
		return -1;
	}

	for let start = 0; start + size + 1 <= cells; start = start + 1 {
		let run = 0;

		// A run is only good if it is long enough, so the scan stops the
		// moment it is, rather than looking for the end of the whole free
		// region. First fit is not looking for the best run, only the first
		// one that works.
		while run < size + 1 && start + run < cells && heap[start + run] == EMPTY {
			run = run + 1;
		}

		if run == size + 1 {
			heap[start] = size;

			for let i = start + 1; i <= start + size; i = i + 1 {
				heap[i] = PAYLOAD;
			}

			return start + 1;
		}
	}

	return -1;
}

// free gives a block back. It is handed the pointer alloc returned, so it can
// find the header by stepping back one cell.
fn free(heap, pointer) {
	let header = pointer - 1;

	if header < 0 || heap[header] < 1 {
		return false;
	}

	let size = heap[header];
	heap[header] = EMPTY;

	for let i = header + 1; i <= header + size; i = i + 1 {
		heap[i] = EMPTY;
	}

	return true;
}

fn used(heap) {
	let count = 0;

	for let i = 0; i < len(heap); i = i + 1 {
		if heap[i] != EMPTY {
			count = count + 1;
		}
	}

	return count;
}

// largestFreeRun is the number a caller actually needs and the total free
// count does not give: the longest single piece of the heap still in one
// piece.
fn largestFreeRun(heap) {
	let best = 0;
	let run = 0;

	for let i = 0; i < len(heap); i = i + 1 {
		if heap[i] == EMPTY {
			run = run + 1;
			if run > best {
				best = run;
			}
		} else {
			run = 0;
		}
	}

	return best;
}

// check walks the heap and reports how many cells are not what they should be.
//
// Every cell is one of three things, and only a header may begin a block, so a
// heap with no cells left over is a heap where no two allocations overlap.
fn check(heap) {
	let wrong = 0;
	let i = 0;

	while i < len(heap) {
		if heap[i] == PAYLOAD {
			// A payload cell with no header in front of it. Either a block was
			// freed and its payload was not, or something wrote where it had
			// no business writing.
			wrong = wrong + 1;
			i = i + 1;
		} else {
			if heap[i] == EMPTY {
				i = i + 1;
			} else {
				let size = heap[i];

				for let j = i + 1; j <= i + size; j = j + 1 {
					if j >= len(heap) || heap[j] != PAYLOAD {
						wrong = wrong + 1;
					}
				}

				i = i + size + 1;
			}
		}
	}

	return wrong;
}

fn report(heap, note) {
	print("      " + note);
	print("      used " + str(used(heap)) + " of " + str(len(heap)) + " cells, largest free run " + str(largestFreeRun(heap)));

	let wrong = check(heap);

	if wrong != 0 {
		print("      check: FAILED - " + str(wrong) + " cells are not what they should be");
		exit(1);
	}
}

let CELLS = 16;
let heap = fill(CELLS, EMPTY);

print("a heap of " + str(CELLS) + " cells, each holding a header, a payload cell, or nothing");

let a = alloc(heap, 4);
print("");
print("  a = alloc(4)   returns cell " + str(a));
report(heap, "four cells and a header");

let b = alloc(heap, 4);
print("");
print("  b = alloc(4)   returns cell " + str(b));
report(heap, "the next four cells and a header");

let c = alloc(heap, 4);
print("");
print("  c = alloc(4)   returns cell " + str(c));
report(heap, "the heap is nearly full");

print("");
print("  free(b)        b was in the middle, so the hole it leaves is in the middle");
free(heap, b);
report(heap, "four cells came back");

// This is the whole point of the example. The heap below has six free cells,
// which is more than the six a request for five needs - and the request still
// cannot be met, because the free cells are not in one piece.
let big = alloc(heap, 5);
print("");
print("  alloc(5)       needs six cells together, returns " + str(big));
print("      " + str(CELLS - used(heap)) + " cells are free and the largest run among them is " + str(largestFreeRun(heap)));
print("      so the request fails with memory to spare");

let small = alloc(heap, 3);
print("");
print("  alloc(3)       needs four cells together, returns cell " + str(small));
report(heap, "the same hole, cut into a smaller piece");

print("");
print("  check: ok - every block is intact and no two overlap");
