// The same maze as examples/maze.cw, carved and searched live on the screen.
//
// This is the animated version. Instead of printing a few snapshots and
// stopping, it draws a frame at a time - the walls falling as the maze is
// carved, the wavefront of a breadth first search spreading outwards, the
// single tendril of a depth first search, and then each route retraced from
// the start to the exit.
//
// Two builtins make it possible. `clear` erases the screen and puts the cursor
// back in the corner, so each frame replaces the last instead of scrolling
// away. `sleep` pauses between frames, so the animation runs at a speed a
// person can watch rather than finishing in the few milliseconds the search
// actually takes.
//
// The maze is smaller than the one in examples/maze.cw so that a frame fits a
// normal terminal window with room to spare.
//
// Run it with:
//
//     ./bin/ccwig examples/animate/maze.cw

// ------------------------------------------------------------------ constants

let SIZE = 15;                       // must be odd
let CELLS = SIZE * SIZE;
let START = 1 * SIZE + 1;            // top-left inner cell
let GOAL = (SIZE - 2) * SIZE + (SIZE - 2);    // bottom-right inner cell
let SEED = 20261012;

// Pacing, all driven by one number.
//
// FRAME_MS is the pause between frames, in milliseconds. Every other wait is
// written as a multiple of it, so raising FRAME_MS slows the whole show down
// and setting it to zero removes every pause at once. The strides say how much
// happens between one frame and the next.
let FRAME_MS = 28;
let HOLD = 12;                      // a pause at a phase boundary
let CARVE_EVERY = 2;                // carve steps per frame
let SEARCH_EVERY = 3;               // search steps per frame
let PATH_EVERY = 1;                 // route steps per frame

// ------------------------------------------------------------ random numbers

// makeRandom returns a generator producing numbers in [0, bound). Seeded, so
// the animation is the same every time it is run.
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

// A grid of nothing, for the frames that mark no cells at all.
let BLANK = fill(CELLS, 0);

// ------------------------------------------------------------------ printing

// draw paints one frame and pauses.
//
// The whole frame is built as a single string and written in one go, rather
// than a line at a time. A frame written line by line would be caught halfway
// through and flicker; written at once it appears complete.
fn draw(title, note, maze, mark, glyph, footer) {
	let out = title + "\n" + note;

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
		out = out + "\n" + row;
		r = r + 1;
	}

	if footer != "" {
		out = out + "\n\n" + footer;
	}

	clear();
	print(out);
	sleep(FRAME_MS);
}

// makeWatcher returns the callback a search calls after every cell it reaches.
//
// The callback decides whether this step is one worth drawing. Handing the
// search a function to call, rather than teaching it about frames, is what
// keeps the search itself identical to the one in examples/maze.cw: it knows
// nothing about drawing.
fn makeWatcher(title, maze, mark, glyph, every) {
	return fn(step) {
		if step % every == 0 {
			draw(title, str(step) + " cells explored", maze, mark, glyph, "");
		}
	};
}

// makeCarveWatcher is the same idea for the carver, which counts cells dug
// rather than cells explored.
fn makeCarveWatcher(maze) {
	return fn(carved) {
		if carved % CARVE_EVERY == 0 {
			draw("carving the maze", str(carved) + " cells dug out", maze, BLANK, ".", "");
		}
	};
}

// ------------------------------------------------------------------- carving

// carve digs the maze, calling onDug after each cell is opened up.
//
// A recursive backtracker - itself a depth first search - with an explicit
// stack in place of the recursion. Each step moves two cells, knocking down
// the wall in between, which leaves every cell reachable from every other.
fn carve(maze, rng, onDug) {
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
	onDug(1);

	while top > 0 {
		let cell = stack[top - 1];
		let r = cell / SIZE;
		let c = cell % SIZE;

		let moves = [];
		if r > 2 { let n = cell - 2 * SIZE; if maze[n] == 1 { moves = push(moves, n); } }
		if r < SIZE - 3 { let n = cell + 2 * SIZE; if maze[n] == 1 { moves = push(moves, n); } }
		if c > 2 { let n = cell - 2; if maze[n] == 1 { moves = push(moves, n); } }
		if c < SIZE - 3 { let n = cell + 2; if maze[n] == 1 { moves = push(moves, n); } }

		if len(moves) == 0 {
			top = top - 1;
		} else {
			let pick = moves[rng(len(moves))];
			maze[(cell + pick) / 2] = 0;
			maze[pick] = 0;
			stack[top] = pick;
			top = top + 1;
			onDug(top);              // the stack depth is the number dug so far
		}
	}

	return maze;
}

// braid knocks out some of the walls that are left, so the maze has loops and
// more than one route through it. Without loops both searches would find the
// same single route and there would be nothing to watch.
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

// ------------------------------------------------------------- the searches

// solveBFS searches breadth first, taking the next cell from the front of a
// queue so the wavefront spreads outwards one ring at a time. The first time
// it reaches the exit, it has reached it by a shortest route.
//
// mark is filled in as cells are reached, which is what the animation draws;
// onFrame is called after each one.
fn solveBFS(maze, mark, onFrame) {
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
		let cell = queue[head];
		head = head + 1;
		mark[cell] = 1;
		explored = explored + 1;
		onFrame(explored);

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

// solveDFS searches depth first - the same loop, taking the next cell from the
// back of a stack instead of the front of a queue.
//
// That one change is most of the difference between them. The most recently
// reached cell is always the one explored next, so the search commits to a
// single corridor and follows it to its end before backing up. It stops at the
// first exit it reaches, not the nearest one, so its route wanders.
//
// A cell is marked when it leaves the stack rather than when it goes on, which
// is what makes the search go deep; a cell can therefore sit on the stack more
// than once, so the stack is sized for the worst case.
fn solveDFS(maze, mark, onFrame) {
	let visited = fill(CELLS, 0);
	let parent = fill(CELLS, -1);
	let stack = fill(CELLS * 4, 0);

	let top = 0;
	stack[top] = START;
	top = top + 1;

	let explored = 0;
	let found = false;

	while top > 0 && found == false {
		let cell = stack[top - 1];
		top = top - 1;

		if visited[cell] == 0 {
			visited[cell] = 1;
			mark[cell] = 1;
			explored = explored + 1;
			onFrame(explored);

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

// reconstruct follows parent back from the exit to the start. The start is the
// only cell with no parent, so that is where the walk stops.
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
// A route drawn on the maze looks convincing whether or not it is real, so
// this is what makes the picture trustworthy.
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

// retrace walks a route from the start to the exit a step at a time, so the
// route is seen being followed rather than appearing all at once. It returns
// the grid it drew, so that the caller can show the finished route again.
fn retrace(title, maze, path) {
	let mark = fill(CELLS, 0);
	let i = 0;
	while i < len(path) {
		mark[path[i]] = 1;
		if i % PATH_EVERY == 0 {
			draw(title, str(i) + " of " + str(len(path) - 1) + " steps", maze, mark, "o", "");
		}
		i = i + 1;
	}
	draw(title, str(len(path) - 1) + " steps", maze, mark, "o", "");
	return mark;
}

// ------------------------------------------------------------------ the show

clear();
print("A maze, carved and searched two ways.");
print("The same maze as examples/maze.cw, one frame at a time.");
sleep(FRAME_MS * HOLD * 3);

// --- carve ------------------------------------------------------------------

let rng = makeRandom(SEED);
let maze = fill(CELLS, 0);
carve(maze, rng, makeCarveWatcher(maze));
braid(maze, rng);
draw("carving the maze", "done - with loops knocked through it", maze, BLANK, ".", "");
sleep(FRAME_MS * HOLD);

// --- breadth first ----------------------------------------------------------

let bfsMark = fill(CELLS, 0);
let bfs = solveBFS(maze, bfsMark, makeWatcher("breadth first search", maze, bfsMark, "*", SEARCH_EVERY));
let bfsParent = bfs[0];
let bfsExplored = bfs[1];
draw("breadth first search", "the wavefront has reached the exit", maze, bfsMark, "*", "");
sleep(FRAME_MS * HOLD);

let bfsPath = reconstruct(bfsParent);
let bfsSteps = len(bfsPath) - 1;
retrace("breadth first: retracing its route", maze, bfsPath);
sleep(FRAME_MS * HOLD);
// --- depth first ------------------------------------------------------------

let dfsMark = fill(CELLS, 0);
let dfs = solveDFS(maze, dfsMark, makeWatcher("depth first search", maze, dfsMark, "*", SEARCH_EVERY));
let dfsParent = dfs[0];
let dfsExplored = dfs[1];
draw("depth first search", "the tendril has reached the exit", maze, dfsMark, "*", "");
sleep(FRAME_MS * HOLD);

let dfsPath = reconstruct(dfsParent);
let dfsSteps = len(dfsPath) - 1;
let dfsRoute = retrace("depth first: retracing its route", maze, dfsPath);

// --- the verdict ------------------------------------------------------------
//
// Six lines, no blank ones, so that the finished maze and the board together
// still fit the twenty-four lines of an ordinary terminal window.

let board = pad("", 16) + padLeft("explored", 10) + padLeft("route", 7) + "\n";
board = board + pad("BFS", 16) + padLeft(str(bfsExplored), 10) + padLeft(str(bfsSteps), 7) + "\n";
board = board + pad("DFS", 16) + padLeft(str(dfsExplored), 10) + padLeft(str(dfsSteps), 7) + "\n";

if bfsSteps < dfsSteps {
	board = board + "BFS found the shortest route - " + str(dfsSteps - bfsSteps) + " steps shorter than DFS's.\n";
} else if bfsSteps == dfsSteps {
	board = board + "Both found a route of the same length.\n";
} else {
	board = board + "DFS happened to find the shorter route this time.\n";
}

board = board + "BFS route: " + verifyPath(bfsPath) + "\n";
board = board + "DFS route: " + verifyPath(dfsPath);

draw("depth first: the route it found", str(dfsSteps) + " steps, drawn on the maze", maze, dfsRoute, "o", board);
