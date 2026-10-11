// vm: a stack machine, an assembler for it, and four programs to run on it.
//
// The machine is the one every dynamic language is compiled to: a stack, a
// program counter, and a little memory. Instructions are pairs of integers -
// an opcode and an operand - which is what an instruction is on real hardware,
// and the assembler is what turns the readable text into those pairs.
//
// The state is global, and it is global on purpose. A machine has one stack
// and one program counter, and passing them into every operation would pretend
// otherwise. This is also the shape in which the state would move into a
// structure if the language had one.
//
//     push 5      [5]                the operand goes on the stack
//     store 0     []                 and comes off into memory
//     load 0      [5]                memory[0]
//     add         [8]                pops two, pushes their sum
//     jz 4        []                 pops, and jumps if it was zero
//
// A stack machine has no registers, so every value lives either on the stack,
// where the last one in is the first one out, or in memory, addressed by a
// number. Two places to keep things, and every instruction above is one of the
// ways to move a value between them.
//
// The whole instruction set is fifteen instructions:
//
//     push n   put the number n on the stack
//     dup      copy the top of the stack
//     swap     exchange the top two
//     drop     throw the top away
//     add      sub   mul   div   mod        pop two, push a op b
//     load n   push memory[n]
//     store n  pop into memory[n]
//     jmp n    continue at instruction n
//     jz n     pop, and continue at instruction n if it was zero
//     print    pop and write the value out
//     halt     stop
//
// That is more than the four programs below use. `dup`, `swap`, `drop`, `div`
// and `print` are what the next program anyone writes would want, and an
// instruction set with holes in it is a machine you have to extend before you
// can use it.

let OP_PUSH = 1;
let OP_DUP = 2;
let OP_SWAP = 3;
let OP_DROP = 4;
let OP_ADD = 5;
let OP_SUB = 6;
let OP_MUL = 7;
let OP_DIV = 8;
let OP_MOD = 9;
let OP_LOAD = 10;
let OP_STORE = 11;
let OP_JMP = 12;
let OP_JZ = 13;
let OP_PRINT = 14;
let OP_HALT = 15;

let STACK_MAX = 64;
let MEMORY_MAX = 16;

// The machine's state: a stack of fixed size with a stack pointer, memory, and
// the program counter. The stack pointer is one past the top, so an empty
// stack is a pointer of zero and the top is always `sp - 1`.
let stack = fill(STACK_MAX, 0);
let memory = fill(MEMORY_MAX, 0);
let sp = 0;
let pc = 0;
let steps = 0;

// The stack operations are named for what they do rather than for the
// instruction that uses them, because a function named `push` would hide the
// language's own `push` for arrays - a builtin is shadowed by any function
// that shares its name, which is worth knowing before it surprises you.
fn pushValue(value) {
	if sp >= STACK_MAX {
		print("stack overflow: the stack holds " + str(STACK_MAX) + " values");
		exit(1);
	}

	stack[sp] = value;
	sp = sp + 1;
}

fn popValue() {
	if sp == 0 {
		print("stack underflow: there was nothing to pop");
		exit(1);
	}

	sp = sp - 1;
	return stack[sp];
}

fn stackText() {
	let shown = [];

	for let i = 0; i < sp; i = i + 1 {
		shown = push(shown, stack[i]);
	}

	return "[" + join(shown, " ") + "]";
}

fn toInt(s) {
	let value = 0;

	for let i = 0; i < len(s); i = i + 1 {
		let ch = s[i];

		if ch < "0" || ch > "9" {
			return -1;
		}

		value = value * 10 + (ord(ch) - ord("0"));
	}

	return value;
}

fn opcodeOf(name) {
	if name == "push" {
		return OP_PUSH;
	}
	if name == "dup" {
		return OP_DUP;
	}
	if name == "swap" {
		return OP_SWAP;
	}
	if name == "drop" {
		return OP_DROP;
	}
	if name == "add" {
		return OP_ADD;
	}
	if name == "sub" {
		return OP_SUB;
	}
	if name == "mul" {
		return OP_MUL;
	}
	if name == "div" {
		return OP_DIV;
	}
	if name == "mod" {
		return OP_MOD;
	}
	if name == "load" {
		return OP_LOAD;
	}
	if name == "store" {
		return OP_STORE;
	}
	if name == "jmp" {
		return OP_JMP;
	}
	if name == "jz" {
		return OP_JZ;
	}
	if name == "print" {
		return OP_PRINT;
	}
	if name == "halt" {
		return OP_HALT;
	}

	return -1;
}

// takesOperand tells the assembler which instructions the words after them
// belong to. It is the same list the disassembler would need, which is the
// price of a variable-length instruction.
fn takesOperand(op) {
	return op == OP_PUSH || op == OP_LOAD || op == OP_STORE || op == OP_JMP || op == OP_JZ;
}

// assemble turns lines of assembly into instructions, each one a pair of
// [opcode, operand]. The operand of an instruction that has none is a zero
// that nothing reads, which keeps every instruction the same width - the
// bargain a real instruction set makes when it pads rather than varies.
fn assemble(lines) {
	let program = [];

	for let i = 0; i < len(lines); i = i + 1 {
		let words = split(lines[i], " ");
		let op = opcodeOf(words[0]);

		if op < 0 {
			print("line " + str(i) + ": unknown instruction: " + lines[i]);
			exit(1);
		}

		let operand = 0;

		if takesOperand(op) {
			if len(words) != 2 {
				print("line " + str(i) + ": " + words[0] + " needs an operand");
				exit(1);
			}

			operand = toInt(words[1]);

			if operand < 0 {
				print("line " + str(i) + ": not a number: " + words[1]);
				exit(1);
			}
		}

		program = push(program, [op, operand]);
	}

	return program;
}

fn listProgram(lines) {
	for let i = 0; i < len(lines); i = i + 1 {
		let label = str(i);

		while len(label) < 3 {
			label = " " + label;
		}

		print("   " + label + "  " + lines[i]);
	}
}

fn binaryOp(op, a, b) {
	if op == OP_ADD {
		return a + b;
	}
	if op == OP_SUB {
		return a - b;
	}
	if op == OP_MUL {
		return a * b;
	}
	if op == OP_DIV {
		if b == 0 {
			print("division by zero at instruction " + str(pc - 1));
			exit(1);
		}
		return a / b;
	}
	return a % b;
}

// execute runs one instruction and leaves the program counter where the next
// one starts. It returns false when the machine has halted.
//
// The stack machine's one rule is on show here: an operation pops what it
// needs and pushes what it makes, so the stack between two instructions is the
// whole of the machine's intermediate state. The trace prints that state after
// the instruction rather than before, which is the reading that makes each
// line a step rather than a preview.
fn execute(program, lines, trace) {
	let instruction = program[pc];
	let op = instruction[0];
	let operand = instruction[1];
	let text = lines[pc];

	steps = steps + 1;
	pc = pc + 1;

	if op == OP_PUSH {
		pushValue(operand);
	}
	if op == OP_DUP {
		pushValue(stack[sp - 1]);
	}
	if op == OP_SWAP {
		let top = popValue();
		let below = popValue();
		pushValue(top);
		pushValue(below);
	}
	if op == OP_DROP {
		popValue();
	}
	if op == OP_ADD || op == OP_SUB || op == OP_MUL || op == OP_DIV || op == OP_MOD {
		let b = popValue();
		let a = popValue();
		pushValue(binaryOp(op, a, b));
	}
	if op == OP_LOAD {
		pushValue(memory[operand]);
	}
	if op == OP_STORE {
		memory[operand] = popValue();
	}
	if op == OP_JMP {
		pc = operand;
	}
	if op == OP_JZ {
		if popValue() == 0 {
			pc = operand;
		}
	}
	if op == OP_PRINT {
		print("   " + str(popValue()));
	}
	if op == OP_HALT {
		return false;
	}

	if trace && steps <= 16 {
		let label = str(steps);

		while len(label) < 3 {
			label = " " + label;
		}
		print("   step " + label + "  " + text + "  ->  " + stackText());
	}

	return true;
}

// run resets the machine, assembles the program, runs it to the halt, prints
// what it left on the stack, and hands the same value back to its caller.
//
// Leaving the answer on the stack is the convention of a stack machine, and
// it is why the four programs below end with a load and a halt rather than
// with a print: what a program computes and what it does with the answer are
// two different questions, and this machine keeps them apart.
fn run(title, lines, trace) {
	sp = 0;
	pc = 0;
	steps = 0;

	let program = assemble(lines);

	print("--- " + title + " ---");
	print("");
	listProgram(lines);
	print("");

	let running = true;

	while running {
		if pc < 0 || pc >= len(program) {
			print("   the program counter ran off the end at " + str(pc));
			exit(1);
		}
		running = execute(program, lines, trace);
	}

	print("");
	print("   result " + str(stack[sp - 1]) + " after " + str(steps) + " steps");
	print("");

	return stack[sp - 1];
}

// The answers the machine gives are checked below against the same numbers
// worked out here, without the machine. A virtual machine that is wrong about
// arithmetic is wrong everywhere at once, and a program that only compares its
// own output with itself would never find out.

fn factorial(n) {
	let product = 1;

	for let i = 2; i <= n; i = i + 1 {
		product = product * i;
	}

	return product;
}

fn fibonacci(n) {
	let previous = 0;
	let current = 1;

	for let i = 0; i < n; i = i + 1 {
		let next = previous + current;
		previous = current;
		current = next;
	}

	return previous;
}

fn gcdOf(a, b) {
	while b != 0 {
		let next = a % b;
		a = b;
		b = next;
	}

	return a;
}

let fromMachine = [];

fromMachine = push(fromMachine, run("factorial of five", [
	"push 5",
	"store 0",
	"push 1",
	"store 1",
	"load 0",
	"jz 15",
	"load 1",
	"load 0",
	"mul",
	"store 1",
	"load 0",
	"push 1",
	"sub",
	"store 0",
	"jmp 4",
	"load 1",
	"halt"
], true));

fromMachine = push(fromMachine, run("the tenth fibonacci number", [
	"push 0",
	"store 0",
	"push 1",
	"store 1",
	"push 10",
	"store 2",
	"load 2",
	"jz 21",
	"load 0",
	"load 1",
	"add",
	"store 3",
	"load 1",
	"store 0",
	"load 3",
	"store 1",
	"load 2",
	"push 1",
	"sub",
	"store 2",
	"jmp 6",
	"load 0",
	"halt"
], false));

fromMachine = push(fromMachine, run("the sum of one to a hundred", [
	"push 0",
	"store 0",
	"push 100",
	"store 1",
	"load 1",
	"jz 15",
	"load 0",
	"load 1",
	"add",
	"store 0",
	"load 1",
	"push 1",
	"sub",
	"store 1",
	"jmp 4",
	"load 0",
	"halt"
], false));

fromMachine = push(fromMachine, run("the greatest common divisor of 1071 and 462", [
	"push 1071",
	"store 0",
	"push 462",
	"store 1",
	"load 1",
	"jz 15",
	"load 0",
	"load 1",
	"mod",
	"store 2",
	"load 1",
	"store 0",
	"load 2",
	"store 1",
	"jmp 4",
	"load 0",
	"halt"
], false));

let withoutTheMachine = [
	factorial(5),
	fibonacci(10),
	100 * 101 / 2,
	gcdOf(1071, 462)
];

let disagreements = 0;

for let i = 0; i < len(withoutTheMachine); i = i + 1 {
	if fromMachine[i] != withoutTheMachine[i] {
		disagreements = disagreements + 1;
	}
}

if disagreements > 0 {
	print("check: FAILED - the machine and the same computations done without it disagree about " + str(disagreements) + " of " + str(len(withoutTheMachine)) + " answers");
	exit(1);
}

print("check: ok - the machine's four answers are the four the same computations give without it");
