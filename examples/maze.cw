// DFS versus BFS: two searches race through the same maze.
//
// The maze is carved with a recursive backtracker - which is itself a depth
// first search - and then solved twice from the same starting point, once
// breadth first and once depth first. The two solvers are nearly the same loop
// written twice; the main difference is which end of the waiting list they take
// the next cell from. Everything else about how differently they behave follows
// from that.
//
// A few snapshots of each search are printed as it goes, so the contrast is
// visible: breadth first spreads outwards in a wave, depth first commits to one
// corridor at a time. Then both routes are drawn and compared.

// ------------------------------------------------------------------ constants

let SIZE = 21;                        // must be odd
let CELLS = SIZE * SIZE;
let START = 1 * SIZE + 1;             // top-left inner cell
let GOAL = (SIZE - 2) * SIZE + (SIZE - 2);   // bottom-right inner cell
let MAZE_SEED = 20261012;
let SNAPSHOTS = 3;

// ------------------------------------------------------------ random numbers

// makeRandom returns a generator producing numbers in [0, bound).
//
// There is no random built-in, so this is a linear congruential generator: the
// next number is the previous one times a constant plus another, taken modulo
// 2^31. It is seeded, which makes the whole program reproducible - the same
// maze every run.
//
// The high bits are returned rather than the low ones, because the low bits of
// this kind of generator repeat far too regularly to pick directions with.
fn makeRandom(seed) {
	let state = seed;
	return fn(bound) {
		state = (state * 1103515245 + 12345) % 2147483648;
		return (state / 65536) % bound;
	};
}

// ------------------------------------------------------------------- helpers

// reverse returns xs back to front.
fn reverse(xs) {
	let out = [];
	let i = len(xs) - 1;
	while i >= 0 {
		out = push(out, xs[i]);
		i = i - 1;
	}
	return out;
}

// carve digs a maze out of a grid of walls.
//
// This is a recursive backtracker, but the "recursion" is an explicit stack so
// that a maze far larger than the call depth limit would still work. Starting
// from one cell, it repeatedly steps two cells into the grid - over the wall
// between - knocking that wall down as it goes. When it reaches a cell with
// nowhere left to go it backtracks to the most recent cell that did, which is
// what the stack is for.
//
// Because every step moves two cells and knocks down one wall between them,
// every cell ends up reachable from every other. It also leaves the maze
// perfect: one winding route between any two points, and no loops at all.
fn carve(maze, rng) {
	let i = 0;
	while i < CELLS {
		maze[i] = 1;
		i = i + 1;
	}

	let stack = fill(CELLS, 0);
	let top = 0;
	maze[START] = 0;
	stack[top] = START;
	top = top + 1;

	while top > 0 {
		let cell = stack[top - 1];       // look at the top, do not take it yet
		let r = cell / SIZE;
		let c = cell % SIZE;

		// The neighbours two cells away that are still solid wall.
		let moves = [];
		if r > 2 { let n = cell - 2 * SIZE; if maze[n] == 1 { moves = push(moves, n); } }
		if r < SIZE - 3 { let n = cell + 2 * SIZE; if maze[n] == 1 { moves = push(moves, n); } }
		if c > 2 { let n = cell - 2; if maze[n] == 1 { moves = push(moves, n); } }
		if c < SIZE - 3 { let n = cell + 2; if maze[n] == 1 { moves = push(moves, n); } }

		if len(moves) == 0 {
			top = top - 1;               // dead end: back up
		} else {
			let pick = moves[rng(len(moves))];
			maze[(cell + pick) / 2] = 0; // the wall between the two cells
			maze[pick] = 0;
			stack[top] = pick;
			top = top + 1;
		}
	}

	return maze;
}

// braid opens up some of the walls the carve left standing, so that the maze
// has loops and therefore more than one route through it.
//
// Without this there would be nothing to compare. A perfect maze has exactly
// one route between any two cells, so both searches would find that route and
// arrive at the same answer by different means. Loops are what make the choice
// of route a real choice: breadth first finds the shortest one, and depth first
// finds whichever one it happens to walk into.
//
// Only walls strictly inside the border are considered, and only those with a
// passage on both sides, which every remaining wall between two odd cells has.
// The outer wall is left alone, so the maze never opens to the outside.
fn braid(maze, rng) {
	let r = 2;
	while r < SIZE - 1 {
		let c = 1;
		while c < SIZE - 1 {
			if maze[r * SIZE + c] == 1 && rng(100) < 18 {
				maze[r * SIZE + c] = 0;
			}
			c = c + 2;
		}
		r = r + 2;
	}

	let r2 = 1;
	while r2 < SIZE - 1 {
		let c2 = 2;
		while c2 < SIZE - 1 {
			if maze[r2 * SIZE + c2] == 1 && rng(100) < 18 {
				maze[r2 * SIZE + c2] = 0;
			}
			c2 = c2 + 2;
		}
		r2 = r2 + 2;
	}

	return maze;
}

// ------------------------------------------------------------------ printing

// render prints the maze, one row per line.
//
// Cells are drawn with single characters so that no line ends in a space,
// which would be invisible in the output and easy to lose. A cell in mark is
// drawn with glyph: "*" while a search is running, "o" once a path is known.
fn render(maze, mark, glyph) {
	let r = 0;
	while r < SIZE {
		let row = "";
		let c = 0;
		while c < SIZE {
			let i = r * SIZE + c;
			if i == START {
				row = row + "S";
			} else if i == GOAL {
				row = row + "E";
			} else if mark[i] == 1 {
				row = row + glyph;
			} else if maze[i] == 1 {
				row = row + "#";
			} else {
				row = row + ".";
			}
			c = c + 1;
		}
		print(row);
		r = r + 1;
	}
}

// snapshotAt prints the maze as it looked after the first count cells were
// explored.
//
// The search records the cells in the order it reached them, so a snapshot is
// just the first count of those marked on the grid. Doing it this way means
// snapshots can be taken at any point of the search, including the very end,
// without having had to guess in advance how long the search would run.
fn snapshotAt(maze, order, count, glyph) {
	let mark = fill(CELLS, 0);
	let i = 0;
	while i < count {
		mark[order[i]] = 1;
		i = i + 1;
	}
	render(maze, mark, glyph);
}

// pad and padLeft pad a string out to a width, for lining up columns.
fn pad(s, width) {
	let out = s;
	while len(out) < width {
		out = out + " ";
	}
	return out;
}

fn padLeft(s, width) {
	let out = s;
	while len(out) < width {
		out = " " + out;
	}
	return out;
}

// ------------------------------------------------------------ the two searches

// solveBFS searches breadth first: it takes the next cell from the front of a
// queue, so cells are explored in order of how many steps they are from the
// start. The wavefront therefore moves outwards one ring at a time, and the
// first time the exit is reached it must have been reached by a shortest
// route.
//
// Every cell is found by way of one other cell, and that cell is recorded in
// parent. Following parent back from the exit is what turns "found it" into an
// actual route.
fn solveBFS(maze) {
	let visited = fill(CELLS, 0);
	let parent = fill(CELLS, -1);
	let queue = fill(CELLS, 0);
	let order = fill(CELLS, 0);

	let head = 0;
	let tail = 0;
	queue[tail] = START;
	tail = tail + 1;
	visited[START] = 1;

	let explored = 0;
	let found = false;

	while head < tail && found == false {
		let cell = queue[head];         // take from the front
		head = head + 1;
		order[explored] = cell;
		explored = explored + 1;

		if cell == GOAL {
			found = true;
		} else {
			let r = cell / SIZE;
			let c = cell % SIZE;

			if r > 0 { let n = cell - SIZE; if maze[n] == 0 && visited[n] == 0 { visited[n] = 1; parent[n] = cell; queue[tail] = n; tail = tail + 1; } }
			if r < SIZE - 1 { let n = cell + SIZE; if maze[n] == 0 && visited[n] == 0 { visited[n] = 1; parent[n] = cell; queue[tail] = n; tail = tail + 1; } }
			if c > 0 { let n = cell - 1; if maze[n] == 0 && visited[n] == 0 { visited[n] = 1; parent[n] = cell; queue[tail] = n; tail = tail + 1; } }
			if c < SIZE - 1 { let n = cell + 1; if maze[n] == 0 && visited[n] == 0 { visited[n] = 1; parent[n] = cell; queue[tail] = n; tail = tail + 1; } }
		}
	}

	return [order, parent, explored, found];
}

// solveDFS searches depth first: nearly the same loop, taking the next cell
// from the back of a stack instead of the front of a queue.
//
// That one change is most of the difference between them. The most recently
// reached cell is always the one explored next, so the search commits to a
// single corridor and follows it until it dead-ends, then backs up to the last
// junction with a turning left untried. Where breadth first spreads out in a
// ring, depth first leaves a single tendril winding through the maze.
//
// It stops at the first exit it reaches rather than the nearest one, so its
// route wanders. On a maze with loops, that route is usually longer than
// breadth first's - though there is no guarantee either way, and on a maze
// without loops the two must agree, there being only one route to find.
//
// A cell is marked when it is taken off the stack rather than when it is put
// on, which is what makes the search go deep: a cell reached early but pushed
// under others waits its turn, and a cell can sit on the stack several times
// over. The extra copies cost stack, which is why it is sized for the worst
// case of every cell being pushed by each of its four neighbours.
fn solveDFS(maze) {
	let visited = fill(CELLS, 0);
	let parent = fill(CELLS, -1);
	let stack = fill(CELLS * 4, 0);
	let order = fill(CELLS, 0);

	let top = 0;
	stack[top] = START;
	top = top + 1;

	let explored = 0;
	let found = false;

	while top > 0 && found == false {
		let cell = stack[top - 1];      // take from the back
		top = top - 1;

		// A cell can be stacked more than once; only the first arrival counts.
		if visited[cell] == 0 {
			visited[cell] = 1;
			order[explored] = cell;
			explored = explored + 1;

			if cell == GOAL {
				found = true;
			} else {
				let r = cell / SIZE;
				let c = cell % SIZE;

				// Pushed in this order so that the first direction tried is up,
				// then left, then right, then down - away from the exit, which
				// is what sends the search off into the far corners. Which
				// order they go in does not affect correctness.
				if r < SIZE - 1 { let n = cell + SIZE; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
				if c < SIZE - 1 { let n = cell + 1; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
				if c > 0 { let n = cell - 1; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
				if r > 0 { let n = cell - SIZE; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
			}
		}
	}

	return [order, parent, explored, found];
}

// reconstruct follows parent back from the exit to the start and returns the
// cells along the way, in order. The start is the only cell with no parent, so
// that is where the walk stops.
fn reconstruct(parent) {
	let path = [];
	let cur = GOAL;
	while cur != -1 {
		path = push(path, cur);
		cur = parent[cur];
	}
	return reverse(path);
}

// verifyPath walks a route and checks it is one: every step moves to a
// neighbouring cell, and the walk begins at the start and ends at the exit.
//
// A search that returns a plausible-looking list of cells is not evidence that
// the search is right, and a path drawn on the maze looks convincing whether or
// not it is real. This is what makes the drawn route trustworthy.
fn verifyPath(path) {
	if len(path) == 0 {
		return "FAILED - empty";
	}
	if path[0] != START {
		return "FAILED - does not begin at the start";
	}
	if path[len(path) - 1] != GOAL {
		return "FAILED - does not end at the exit";
	}

	let i = 1;
	while i < len(path) {
		let a = path[i - 1];
		let b = path[i];
		let d = b - a;

		// Sideways steps must not wrap around the edge of the grid, which is
		// why a difference of one is only a step if both cells share a row.
		let adjacent = false;
		if d == 1 { if a / SIZE == b / SIZE { adjacent = true; } }
		if d == -1 { if a / SIZE == b / SIZE { adjacent = true; } }
		if d == SIZE { adjacent = true; }
		if d == -SIZE { adjacent = true; }

		if adjacent == false {
			return "FAILED - step " + str(i) + " is not between neighbouring cells";
		}
		i = i + 1;
	}

	return "ok - every step is between neighbouring cells";
}

// report prints the snapshots, the route and the check for one search, and
// returns how many cells it explored and how long its route was.
fn report(label, result, maze) {
	let order = result[0];
	let parent = result[1];
	let explored = result[2];
	let found = result[3];

	if found == false {
		print(label + ": the exit was never reached");
		return [explored, 0];
	}

	// Snapshots spread evenly through the search.
	let k = 1;
	while k <= SNAPSHOTS {
		let at = explored * k / (SNAPSHOTS + 1);
		print(label + " after " + str(at) + " of " + str(explored) + " cells:");
		snapshotAt(maze, order, at, "*");
		k = k + 1;
	}

	let path = reconstruct(parent);
	print(label + " route: " + str(len(path) - 1) + " steps");
	let mark = fill(CELLS, 0);
	let i = 0;
	while i < len(path) {
		mark[path[i]] = 1;
		i = i + 1;
	}
	render(maze, mark, "o");
	print("check: " + verifyPath(path));

	return [explored, len(path) - 1];
}

// ------------------------------------------------------------------ the race

print("=== a maze, searched two ways ===");

let rng = makeRandom(MAZE_SEED);
let maze = braid(carve(fill(CELLS, 0), rng), rng);

print("");
print("--- the maze ---");
render(maze, fill(CELLS, 0), ".");

print("");
print("--- breadth first ---");
let bfs = report("BFS", solveBFS(maze), maze);

print("");
print("--- depth first ---");
let dfs = report("DFS", solveDFS(maze), maze);

print("");
print("--- scoreboard ---");
print(pad("", 16) + padLeft("explored", 10) + padLeft("route", 7));
print(pad("BFS", 16) + padLeft(str(bfs[0]), 10) + padLeft(str(bfs[1]), 7));
print(pad("DFS", 16) + padLeft(str(dfs[0]), 10) + padLeft(str(dfs[1]), 7));
print("");

if bfs[1] < dfs[1] {
	print("BFS found the shortest route - " + str(dfs[1] - bfs[1]) + " steps shorter than DFS's.");
} else if bfs[1] == dfs[1] {
	print("Both found a route of the same length.");
} else {
	print("DFS happened to find the shorter route this time.");
}
