package ccwig_test

import (
	"hash/crc32"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The examples under examples/systems make claims about themselves. A claim a
// program checks itself is worth less than a claim checked by something that
// shares no code with it, so the ones that can be recomputed here in Go are.

// TestSystemExamplesPassTheirOwnChecks is the floor, not the ceiling: these
// programs check their own work as they run, and a check that failed would be
// printed as FAILED rather than thrown as an error.
func TestSystemExamplesPassTheirOwnChecks(t *testing.T) {
	for _, src := range examplePrograms(t) {
		if filepath.Base(filepath.Dir(src)) != "systems" {
			continue
		}

		t.Run(filepath.Base(src), func(t *testing.T) {
			got := runFile(t, src)

			if strings.Contains(got, "FAILED") {
				t.Errorf("%s reported a failed check:\n%s", src, got)
			}
			if !strings.Contains(got, "check: ok") {
				t.Errorf("%s reports no passing check at all:\n%s", src, got)
			}
		})
	}
}

// TestBitsetPrimesAgreeWithGoSieve counts the primes below the limit with an
// ordinary sieve here, and compares the answer with the one the bitset
// example reports.
func TestBitsetPrimesAgreeWithGoSieve(t *testing.T) {
	got := runFile(t, "examples/systems/bitset.cw")

	limit := numberAfter(t, got, `sieve of eratosthenes to (\d+)`)
	reported := numberAfter(t, got, `primes +(\d+)`)

	if want := countPrimesBelow(limit); reported != want {
		t.Errorf("bitset.cw reports %d primes up to %d, but a plain sieve here finds %d", reported, limit, want)
	}
}

// TestCRC32AgreesWithGo recomputes every published vector with Go's own CRC-32
// and compares it with the hex the program printed. A checksum is only worth
// anything if someone else's implementation agrees with it, and this is the
// closest thing to someone else available here.
func TestCRC32AgreesWithGo(t *testing.T) {
	got := runFile(t, "examples/systems/crc32.cw")

	hexes := regexp.MustCompile(`\b[0-9a-f]{8}\b`).FindAllString(got, -1)
	vectors := []string{
		"",
		"a",
		"abc",
		"123456789",
		"The quick brown fox jumps over the lazy dog",
	}

	// The output holds the five printed checksums plus the one the check line
	// quotes at the end.
	if len(hexes) != len(vectors)+1 {
		t.Fatalf("expected %d checksums in the output, found %d:\n%s", len(vectors)+1, len(hexes), got)
	}

	for i, message := range vectors {
		want := strconv.FormatUint(uint64(crc32.ChecksumIEEE([]byte(message))), 16)
		// Eight digits, the same as the program writes them.
		for len(want) < 8 {
			want = "0" + want
		}

		if hexes[i] != want {
			t.Errorf("crc32.cw gives %q for %q, but Go's crc32 gives %q", hexes[i], message, want)
		}
	}
}

// TestVMResultsAgreeWithGo recomputes the four answers here and compares them
// with what the machine printed. A virtual machine whose arithmetic cannot be
// checked against anything else is a machine that could be wrong about all of
// it and look convincing.
func TestVMResultsAgreeWithGo(t *testing.T) {
	got := runFile(t, "examples/systems/vm.cw")

	results := regexp.MustCompile(`result (\d+) after (\d+) steps`).FindAllStringSubmatch(got, -1)

	factorial := 1
	for i := 2; i <= 5; i++ {
		factorial = factorial * i
	}

	fibPrevious, fibCurrent := 0, 1
	for i := 0; i < 10; i++ {
		fibPrevious, fibCurrent = fibCurrent, fibPrevious+fibCurrent
	}

	want := []int{factorial, fibPrevious, 100 * 101 / 2, gcd(1071, 462)}

	if len(results) != len(want) {
		t.Fatalf("expected %d results from vm.cw, found %d:\n%s", len(want), len(results), got)
	}

	for i, m := range results {
		value, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("could not read the result %q: %v", m[1], err)
		}

		if value != want[i] {
			t.Errorf("result %d of vm.cw is %d, but the same computation here gives %d", i+1, value, want[i])
		}

		if steps, err := strconv.Atoi(m[2]); err != nil || steps == 0 {
			t.Errorf("result %d reports %q steps, which is not a count of steps", i+1, m[2])
		}
	}
}

// TestMallocShowsWhatFragmentationCosts pins the thing the allocator example
// exists to show: a request that fails while enough cells are free, because
// they are not in one piece.
func TestMallocShowsWhatFragmentationCosts(t *testing.T) {
	got := runFile(t, "examples/systems/malloc.cw")

	// The failing request, and the two numbers that explain it.
	if !strings.Contains(got, "alloc(5)       needs six cells together, returns -1") {
		t.Errorf("malloc.cw no longer reports the request that fails:\n%s", got)
	}
	if !strings.Contains(got, "6 cells are free and the largest run among them is 5") {
		t.Errorf("malloc.cw no longer explains why the request failed:\n%s", got)
	}

	// And the smaller request that fits in what is left: a heap that refused
	// everything would make the same point and be much less useful.
	if !strings.Contains(got, "alloc(3)       needs four cells together, returns cell 6") {
		t.Errorf("malloc.cw no longer shows a smaller request fitting in the same hole:\n%s", got)
	}
}

// numberAfter reads the first number that follows a pattern in the output.
func numberAfter(t *testing.T, out, pattern string) int {
	t.Helper()

	m := regexp.MustCompile(pattern).FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("the output has nothing matching %q:\n%s", pattern, out)
	}

	n, err := strconv.Atoi(m[1])
	if err != nil {
		t.Fatalf("%q is not a number: %v", m[1], err)
	}
	return n
}

// countPrimesBelow sieves the primes up to limit, which is the plainest
// implementation of the same answer.
func countPrimesBelow(limit int) int {
	composite := make([]bool, limit+1)
	primes := 0

	for n := 2; n <= limit; n++ {
		if composite[n] {
			continue
		}

		primes++
		for multiple := n * n; multiple <= limit; multiple += n {
			composite[multiple] = true
		}
	}

	return primes
}

// gcd is Euclid's algorithm, spelled out rather than called from a library.
func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

// TestEverySystemExampleHasItsOwnFile keeps the systems directory from
// collecting a program nobody runs.
func TestEverySystemExampleHasItsOwnFile(t *testing.T) {
	found, err := filepath.Glob("examples/systems/*.cw")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(found) == 0 {
		t.Fatal("no systems examples found under examples/systems/")
	}

	for _, src := range found {
		if _, err := os.Stat(strings.TrimSuffix(src, ".cw") + ".out"); err != nil {
			t.Errorf("%s has no recorded output", src)
		}
	}
}
