## CCWIG -> C Compiler Written in Go

> I am starting this project on 2024-08-28 
> The Code will be written on a Raspberry pi zero 2W 
> Just to maybe challange myself and prove that a complex level project can be made in simplest machine (Cost of a raspberry pi zero 2 W at this time in india is around 1.5k rupees or roughly 15 dollars)
---
### 🛠️ To-Do: Build a Go Interpreter/Compiler in 1 Week

## **Day 1: Set Up and Learn Core Concepts**
- [x] Install Go and set up the environment.
- [x] Test installation using `go version`.
- [x] Complete [Go Tour](https://tour.golang.org/) to learn basics:
  - Variables, loops, conditionals.
  - Structs, interfaces, functions.
  - All basic Stuff (Go as deep as you want to)
- [ ] Learn the three main parts of an interpreter:
  - Lexical Analysis (Tokenizer).
  - Parsing (Abstract Syntax Tree).
  - Evaluation (Executing AST).
- [ ] Define the minimal language features(Yet to be finalized):
  - Arithmetic operations (`+`, `-`, `*`, `/`).
  - Variables and assignments.
  - Conditional statements (`if`, `else`).
  - Print statements for output.
## **Day 2: Implement the Lexer**
- [ ] Define token types (`NUMBER`, `IDENTIFIER`, `OPERATOR`, `KEYWORD`, etc.).
- [ ] Write the `Lexer` function to tokenize input.
- [ ] Handle whitespace and invalid input gracefully.
- [ ] Test the lexer with simple examples:
  - Input: `x = 5 + 3`.
  - Output: A list of tokens.

## **Day 3: Implement the Parser**
- [ ] Define AST nodes using structs:
  - Binary expressions.
  - Variable declarations.
  - Literals.
- [ ] Write a recursive descent parser to build an AST:
  - Parse expressions like `1 + 2`.
  - Handle assignments like `x = 5`.
- [ ] Test the parser:
  - Input: `x = 1 + 2`.
  - Output: A tree-like structure (AST).

## **Day 4: Implement the Interpreter**
- [ ] Write an `Eval` function to evaluate the AST.
- [ ] Store variables in a map for assignments.
- [ ] Handle:
  - Arithmetic operations.
  - Variable lookups.
  - Print statements.
- [ ] Add basic error handling for:
  - Invalid syntax.
  - Undefined variables.
- [ ] Test the interpreter:
  - Input: `x = 10; y = x + 5; print(y)`.
  - Output: `15`.

## **Day 5: Add Basic Features**
- [ ] Add support for conditionals:
  ```monkey
  if x > 5 {
      print("greater")
  } else {
      print("less")
  }

(Optional) Add looping constructs like while or for.

(Optional) Implement functions and function calls.

## **Day 6: Polish and Extend**

- [ ] Improve error messages for:
  - Lexing errors.
  - Parsing errors.
  - Runtime errors.

- [ ] Build a simple CLI or REPL:

  - Allow users to run files or interactively input code.

  - Enhance syntax with:
      - Comments.
      - String literals.
      - Additional operators.

## **Day 7: Refine and Test**
  ### Test edge cases:
  - [ ] Invalid input.
  - [ ] Large expressions.
  - [ ] Edge scenarios.

### Optimize code:

  - [ ]  Simplify logic.
  - [ ]  Add comments for readability.

### Document the project:

  - [ ]  Write basic documentation for the language.

### Package the code:
  - [ ] Create a executable
---


### Timeline
Check [Blog](https://www.utkarshdeoli.in/blog) for complete timeline

