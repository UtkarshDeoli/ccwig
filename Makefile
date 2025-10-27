# CCWIG build and test targets.

BINARY  := ccwig
PKG     := ./cmd/ccwig
BIN_DIR := bin
VERSION := 1.0.0

# -s -w drop the symbol table and DWARF data, which keeps the binary small
# enough to be comfortable on a Raspberry Pi Zero.
LDFLAGS := -s -w

.PHONY: all build install test test-race fuzz bench fmt vet check examples animate huge run clean pi help

all: check build  ## Format, vet, test and build

build:  ## Build bin/ccwig for the host
	@mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY) $(PKG)
	@echo "built $(BIN_DIR)/$(BINARY) (version $(VERSION))"

install:  ## Install ccwig into GOPATH/bin
	go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

test:  ## Run the test suite
	go test ./...

test-race:  ## Run the test suite under the race detector
	go test -race ./...

fuzz:  ## Run every fuzz target for 30s each
	go test ./internal/lexer/ -run '^$$' -fuzz FuzzNextToken -fuzztime 30s
	go test ./internal/lexer/ -run '^$$' -fuzz FuzzErrorsArePositioned -fuzztime 30s
	go test ./internal/parser/ -run '^$$' -fuzz FuzzParseProgram -fuzztime 30s
	go test ./internal/parser/ -run '^$$' -fuzz FuzzParserTerminates -fuzztime 30s
	go test ./internal/evaluator/ -run '^$$' -fuzz '^FuzzEval$$' -fuzztime 30s
	go test ./internal/evaluator/ -run '^$$' -fuzz FuzzEvalIsDeterministic -fuzztime 30s
	go test ./internal/evaluator/ -run '^$$' -fuzz FuzzRenderingPreservesMeaning -fuzztime 30s

bench:  ## Run every benchmark
	go test -run '^$$' -bench . -benchmem ./internal/...

fmt:  ## Format all Go source
	gofmt -l -w .

vet:  ## Run go vet
	go vet ./...

check: fmt vet test  ## Format, vet and test

examples: build  ## Run every example program
	@for f in examples/*.cw; do \
		echo "=== $$f ==="; \
		./$(BIN_DIR)/$(BINARY) $$f || exit 1; \
	done

animate: build  ## Play the animated maze example
	./$(BIN_DIR)/$(BINARY) examples/animate/maze.cw

huge: build  ## Run the very large maze example
	./$(BIN_DIR)/$(BINARY) examples/huge/maze.cw

run: build  ## Build and start the REPL
	./$(BIN_DIR)/$(BINARY)

pi:  ## Cross-compile for Raspberry Pi (Zero 2 W is arm64; Zero W is armv6)
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-linux-arm64 $(PKG)
	GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY)-linux-armv6 $(PKG)
	@echo "built:"
	@ls -lh $(BIN_DIR)/$(BINARY)-linux-* | awk '{print "  " $$9 "  " $$5}'

clean:  ## Remove build output
	rm -rf $(BIN_DIR)

help:  ## List the available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}'
