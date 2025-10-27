// A maze too big to follow by eye: 101 by 101, ten thousand and one positions.
//
// This is examples/maze.cw scaled up until the maze stops being a picture you
// trace with a finger and becomes one you take in at a glance. At this size the
// two searches look properly different: breadth first has to cross thousands of
// cells before its wavefront arrives at the exit, and depth first commits to a
// single corridor that wanders for hundreds of steps on the way.
//
// SIZE below is the only number that needs changing, and the carving, both
// searches, the routes and every statistic follow it. This program deliberately
// has no recorded output, so go ahead and change it - examples/maze.cw is the
// one pinned to an expected-output file, and it is the readable introduction.
//
// What it cannot do is grow without limit. Size is quadratic in the cells, and
// the interpreter stops a program that has evaluated fifty million steps, so the
// whole program runs out of budget somewhere around SIZE = 300. Measured on the
// machine this was written on: 101 takes a tenth of a second, 201 three quarters
// of a second, 301 three seconds, and 351 stops with the step limit exceeded
// partway through the breadth first search. The search that costs the most is
// the depth first one, which can put a cell on its stack once for each of its
// neighbours rather than once in total.
//
// What makes a maze this size possible at all is the `fill` built-in. An array
// cannot be written past its end, so every grid has to be built at full size
// before anything is stored in it, and building one a `push` at a time copies
// the whole array on every push - some fifty million element copies for a
// single grid at this size, and this program builds about a dozen. `fill`
// allocates a grid once instead.
//
// Run it with:
//
//     ./bin/ccwig examples/huge/maze.cw
//
// The maze is drawn one character per position, so it is 101 characters wide. A
// narrower terminal will wrap it and scramble the picture; widen the window, or
// turn SIZE down until it fits.

// ------------------------------------------------------------------ constants

let SIZE = 101;                      // must be odd; 201 and 301 also work
let CELLS = SIZE * SIZE;
let START = 1 * SIZE + 1;            // top-left inner cell
let GOAL = (SIZE - 2) * SIZE + (SIZE - 2);    // bottom-right inner cell
let MAZE_SEED = 20261012;
let BRAID_PERCENT = 18;             // how eagerly to open up loops

// ------------------------------------------------------------ random numbers

// makeRandom returns a generator producing numbers in [0, bound). Seeded, so
// the maze is the same every run.
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

// commas writes a number with thousands separators. Five figures is where a
// count stops being readable at a glance, and this maze's counts all run to
// five figures.
//
// Strings cannot be indexed, so this peels digits off numerically: the last
// three are one group, and whatever is left in front of them is the same
// problem again, which is the recursion.
fn commas(n) {
	if n < 1000 {
		return str(n);
	}

	let group = str(n % 1000);
	while len(group) < 3 {
		group = "0" + group;
	}
	return commas(n / 1000) + "," + group;
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

// ------------------------------------------------------------------- carving

// carve digs a maze out of a grid of walls.
//
// A recursive backtracker, with an explicit stack in place of the recursion so
// that a maze far larger than the call depth limit still works. Every step
// moves two cells and knocks down the wall between them, which leaves every
// cell reachable from every other.
//
// It stops when the grid is a spanning tree: every inner cell is open, joined
// by exactly one passage fewer than there are cells, and no loops at all. The
// caller checks that, because at this size it cannot be seen.
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

// braid opens up some of the walls the carve left standing, so the maze has
// loops and therefore more than one route through it.
//
// Without loops a maze has exactly one route between any two cells, so both
// searches would find that route and arrive at the same answer by different
// means - there would be nothing to compare. Only walls strictly inside the
// border are considered, so the maze never opens to the outside.
fn braid(maze, rng) {
	let r = 2;
	while r < SIZE - 1 {
		let c = 1;
		while c < SIZE - 1 {
			if maze[r * SIZE + c] == 1 && rng(100) < BRAID_PERCENT {
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
			if maze[r2 * SIZE + c2] == 1 && rng(100) < BRAID_PERCENT {
				maze[r2 * SIZE + c2] = 0;
			}
			c2 = c2 + 2;
		}
		r2 = r2 + 2;
	}

	return maze;
}

// ------------------------------------------------------------------ counting

// countOpen counts every open position in the grid - the cells a search can
// stand on, plus the wall positions knocked out to join them up.
fn countOpen(maze) {
	let n = 0;
	let i = 0;
	while i < CELLS {
		if maze[i] == 0 { n = n + 1; }
		i = i + 1;
	}
	return n;
}

// countCells counts the cells themselves: the open positions on odd rows and
// odd columns. A wall between two cells is never on an odd row and an odd
// column at once, so this counts cells and nothing else.
//
// Open minus cells is therefore the number of passages, and on a freshly carved
// maze that must be exactly one less than the number of cells. Counting it is
// how a maze this size is checked: 2,500 cells and 2,499 passages is a tree, and
// a tree is a maze with exactly one route between any two points.
fn countCells(maze) {
	let n = 0;
	let r = 1;
	while r < SIZE {
		let c = 1;
		while c < SIZE {
			if maze[r * SIZE + c] == 0 { n = n + 1; }
			c = c + 2;
		}
		r = r + 2;
	}
	return n;
}

// ------------------------------------------------------------------ printing

// render prints the maze, one row per line.
//
// Cells are drawn with single characters so that no line ends in a space, which
// would be invisible in the output and easy to lose. A cell in mark is drawn
// with glyph.
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

// ------------------------------------------------------------ the two searches

// solveBFS searches breadth first: it takes the next cell from the front of a
// queue, so cells are explored in order of how many steps they are from the
// start. The wavefront moves outwards one ring at a time, and the first time
// the exit is reached it must have been reached by a shortest route.
//
// Every cell is found by way of one other cell, recorded in parent; following
// parent back from the exit is what turns "found it" into an actual route.
fn solveBFS(maze) {
	let visited = fill(CELLS, 0);
	let parent = fill(CELLS, -1);
	let queue = fill(CELLS, 0);

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

	return [parent, explored, found];
}

// solveDFS searches depth first: nearly the same loop, taking the next cell
// from the back of a stack instead of the front of a queue.
//
// That one change is most of the difference between them. The most recently
// reached cell is always the one explored next, so the search commits to a
// single corridor and follows it until it dead-ends, then backs up to the last
// junction with a turning left untried. It stops at the first exit it reaches
// rather than the nearest one, so its route wanders - which at this size means
// a route many times longer than the shortest one.
//
// A cell is marked when it is taken off the stack rather than when it is put
// on, which is what makes the search go deep: a cell reached early but pushed
// under others waits its turn, and can sit on the stack several times over. The
// extra copies cost stack, which is why it is sized for the worst case of every
// cell being pushed by each of its four neighbours.
fn solveDFS(maze) {
	let visited = fill(CELLS, 0);
	let parent = fill(CELLS, -1);
	let stack = fill(CELLS * 4, 0);

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
			explored = explored + 1;

			if cell == GOAL {
				found = true;
			} else {
				let r = cell / SIZE;
				let c = cell % SIZE;

				// Pushed so that the first direction tried is up, then left,
				// then right, then down - away from the exit, which is what
				// sends the search off into the far corners.
				if r < SIZE - 1 { let n = cell + SIZE; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
				if c < SIZE - 1 { let n = cell + 1; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
				if c > 0 { let n = cell - 1; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
				if r > 0 { let n = cell - SIZE; if maze[n] == 0 && visited[n] == 0 { if parent[n] == -1 { parent[n] = cell; } stack[top] = n; top = top + 1; } }
			}
		}
	}

	return [parent, explored, found];
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

// markPath returns a grid with the cells of a route marked.
fn markPath(path) {
	let marks = fill(CELLS, 0);
	let i = 0;
	while i < len(path) {
		marks[path[i]] = 1;
		i = i + 1;
	}
	return marks;
}

// countShared counts how many cells of path are marked in marks.
//
// Two routes through the same maze start together and finish together, in
// between they may agree or go their own ways. How much of the maze they have
// in common is the plainest measure of how differently the two searches
// behaved.
fn countShared(path, marks) {
	let n = 0;
	let i = 0;
	while i < len(path) {
		if marks[path[i]] == 1 { n = n + 1; }
		i = i + 1;
	}
	return n;
}

// verifyPath walks a route and checks it is one: every step moves to a
// neighbouring cell, and the walk begins at the start and ends at the exit.
//
// A search that returns a plausible-looking list of cells is not evidence that
// the search is right, and at this size a route drawn on the maze looks
// convincing whether or not it is real. This is what makes the drawn route
// trustworthy.
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

// ------------------------------------------------------------------ the race

print("=== a maze " + str(SIZE) + " x " + str(SIZE) + " ===");
print(commas(CELLS) + " positions, of which " + commas((SIZE - 2) * (SIZE - 2)) +
	" are inside the outer wall.");

let rng = makeRandom(MAZE_SEED);
let maze = carve(fill(CELLS, 0), rng);

// The carver claims to leave a tree behind: every inner cell open and joined by
// exactly one passage fewer than there are cells. At twenty cells that is plain
// to see; at two and a half thousand it has to be counted.
let cells = countCells(maze);
let carved = countOpen(maze);
let passages = carved - cells;

print("");
print("carved: " + commas(cells) + " cells joined by " + commas(passages) + " passages");
if passages == cells - 1 {
	print("  a tree, as a maze must be - exactly one route between any two cells");
} else {
	print("  FAILED - a tree needs exactly " + commas(cells - 1) + " passages");
}

braid(maze, rng);
let braided = countOpen(maze) - carved;
print("braided: " + commas(braided) + " further walls knocked out, so routes are now choices");

// What a search has to work with. The searches are timed against it rather than
// against the size of the grid, since the walls between cells are most of it.
let openCells = carved + braided;

print("");
print("--- the maze ---");
render(maze, fill(CELLS, 0), ".");

print("");
print("--- breadth first ---");
let bfs = solveBFS(maze);
let bfsExplored = bfs[1];
let bfsPath = reconstruct(bfs[0]);
print("explored " + commas(bfsExplored) + " of the " + commas(openCells) +
	" open cells (" + str(bfsExplored * 100 / openCells) + "%)");
print("route: " + commas(len(bfsPath) - 1) + " steps");
render(maze, markPath(bfsPath), "o");
print("check: " + verifyPath(bfsPath));

print("");
print("--- depth first ---");
let dfs = solveDFS(maze);
let dfsExplored = dfs[1];
let dfsPath = reconstruct(dfs[0]);
print("explored " + commas(dfsExplored) + " of the " + commas(openCells) +
	" open cells (" + str(dfsExplored * 100 / openCells) + "%)");
print("route: " + commas(len(dfsPath) - 1) + " steps");
render(maze, markPath(dfsPath), "o");
print("check: " + verifyPath(dfsPath));

print("");
print("--- scoreboard ---");
print(pad("", 16) + padLeft("explored", 10) + padLeft("route", 7));
print(pad("BFS", 16) + padLeft(commas(bfsExplored), 10) + padLeft(commas(len(bfsPath) - 1), 7));
print(pad("DFS", 16) + padLeft(commas(dfsExplored), 10) + padLeft(commas(len(dfsPath) - 1), 7));
print("");

let shared = countShared(dfsPath, markPath(bfsPath));
print("the two routes have " + commas(shared) + " of their cells in common");
if shared < 3 {
	print("  they agree only at the start and the exit");
}
print("");

if len(bfsPath) < len(dfsPath) {
	print("BFS found the shortest route - " + commas(len(dfsPath) - len(bfsPath)) +
		" steps shorter than DFS's.");
} else if len(bfsPath) == len(dfsPath) {
	print("Both found a route of the same length.");
} else {
	print("DFS happened to find the shorter route this time.");
}
