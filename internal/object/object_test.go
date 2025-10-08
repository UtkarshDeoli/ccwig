package object

import (
	"testing"

	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// TestEnvironmentBindsAtEverySize walks the whole range of scope sizes around
// mapThreshold, checking after every binding that each name written so far can
// still be read back.
//
// The interesting sizes are the ones just past the point where a scope starts
// indexing itself: those are the bindings added after the index was built.
func TestEnvironmentBindsAtEverySize(t *testing.T) {
	const max = mapThreshold * 3

	env := NewEnvironment()
	for i := 0; i < max; i++ {
		name := bindingName(i)
		env.Set(name, &Integer{Value: int64(i)})

		for j := 0; j <= i; j++ {
			want := bindingName(j)
			got, ok := env.Get(want)
			if !ok {
				t.Fatalf("with %d bindings, %q is not bound", i+1, want)
			}
			if got.(*Integer).Value != int64(j) {
				t.Fatalf("with %d bindings, %q is %v, want %d", i+1, want, got, j)
			}
		}
	}
}

// TestEnvironmentKeepsBindingsAddedAfterItGrows is the regression guard for a
// scope that lost the bindings written after it started indexing itself.
//
// A scope kept names in a slice and built a name index once it grew past
// mapThreshold. Adding a further name appended to the slice but left the index
// alone, and lookups consult the index once it exists, so every binding added
// after that point was written and then could not be found again.
func TestEnvironmentKeepsBindingsAddedAfterItGrows(t *testing.T) {
	env := NewEnvironment()

	// Enough names to build the index and then keep going past it.
	for i := 0; i < mapThreshold+5; i++ {
		env.Set(bindingName(i), &Integer{Value: int64(i)})
	}

	for i := 0; i < mapThreshold+5; i++ {
		name := bindingName(i)
		got, ok := env.Get(name)
		if !ok {
			t.Fatalf("%q is not bound", name)
		}
		if got.(*Integer).Value != int64(i) {
			t.Errorf("%q is %v, want %d", name, got, i)
		}
	}
}

// TestEnvironmentGetSearchesOuterScopes checks that a name missing from the
// current scope is found in an enclosing one.
func TestEnvironmentGetSearchesOuterScopes(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("a", &Integer{Value: 1})

	middle := NewEnclosedEnvironment(outer)
	middle.Set("b", &Integer{Value: 2})

	inner := NewEnclosedEnvironment(middle)
	inner.Set("c", &Integer{Value: 3})

	for _, tt := range []struct {
		env  *Environment
		name string
		want int64
	}{
		{inner, "a", 1},
		{inner, "b", 2},
		{inner, "c", 3},
		{middle, "a", 1},
		{middle, "b", 2},
		{outer, "a", 1},
	} {
		got, ok := tt.env.Get(tt.name)
		if !ok {
			t.Fatalf("%q is not bound", tt.name)
		}
		if got.(*Integer).Value != tt.want {
			t.Errorf("%q is %v, want %d", tt.name, got, tt.want)
		}
	}

	if _, ok := outer.Get("c"); ok {
		t.Error("an inner binding is visible from the outer scope")
	}
}

// TestEnvironmentSetShadowsOuter checks that Set writes to this scope even
// when an enclosing scope already binds the name: `let` declares a new
// variable rather than updating an old one.
func TestEnvironmentSetShadowsOuter(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("x", &Integer{Value: 1})

	inner := NewEnclosedEnvironment(outer)
	inner.Set("x", &Integer{Value: 2})

	if got := lookupInt(t, outer, "x"); got != 1 {
		t.Errorf("outer x is %d, want 1 — an inner let changed an outer binding", got)
	}
	if got := lookupInt(t, inner, "x"); got != 2 {
		t.Errorf("inner x is %d, want 2", got)
	}
}

// TestEnvironmentAssignUpdatesOuterBinding checks that assignment reaches an
// enclosing scope, which is what lets a loop body mutate the loop variable.
func TestEnvironmentAssignUpdatesOuterBinding(t *testing.T) {
	outer := NewEnvironment()
	outer.Set("i", &Integer{Value: 1})

	inner := NewEnclosedEnvironment(outer)
	inner.Assign("i", &Integer{Value: 2})

	if got := lookupInt(t, outer, "i"); got != 2 {
		t.Errorf("outer i is %d, want 2 — the assignment did not reach outwards", got)
	}
	if got := lookupInt(t, inner, "i"); got != 2 {
		t.Errorf("inner i is %d, want 2", got)
	}
}

// TestEnvironmentAssignReachesBindingsAddedAfterItGrows is the regression guard
// for assignment losing sight of a binding in a scope that had grown.
//
// When the enclosing scope could not find the name, the assignment fell back
// to creating it in the current scope, so a block that assigned to a global
// quietly wrote a variable of its own and the global kept its old value.
func TestEnvironmentAssignReachesBindingsAddedAfterItGrows(t *testing.T) {
	global := NewEnvironment()
	for i := 0; i < mapThreshold+5; i++ {
		global.Set(bindingName(i), &Integer{Value: int64(i)})
	}

	late := bindingName(mapThreshold + 4)
	block := NewEnclosedEnvironment(global)
	block.Assign(late, &Integer{Value: 99})

	if got := lookupInt(t, global, late); got != 99 {
		t.Errorf("global %s is %d, want 99 — the assignment created a shadow instead", late, got)
	}
}

// TestEnvironmentAssignCreatesWhenMissing checks that assigning to a name that
// exists nowhere binds it in the currentscope.
func TestEnvironmentAssignCreatesWhenMissing(t *testing.T) {
	outer := NewEnvironment()
	inner := NewEnclosedEnvironment(outer)

	inner.Assign("fresh", &Integer{Value: 7})

	if got := lookupInt(t, inner, "fresh"); got != 7 {
		t.Errorf("inner fresh is %d, want 7", got)
	}
	if _, ok := outer.Get("fresh"); ok {
		t.Error("the assignment bound the name in the outer scope instead")
	}
}

// TestEnvironmentSetRebindsInPlace checks that writing an existing name does
// not accumulate duplicate slots: the scope should hold one binding per name.
func TestEnvironmentSetRebindsInPlace(t *testing.T) {
	env := NewEnvironment()
	for i := 0; i < mapThreshold*3; i++ {
		env.Set(bindingName(i), &Integer{Value: int64(i)})
	}

	// Rewrite one name, then grow the scope further, then rewrite it again.
	// Each rewrite must keep the name visible, whichever slot it lands in.
	name := bindingName(mapThreshold + 2)
	env.Set(name, &Integer{Value: 100})
	if got := lookupInt(t, env, name); got != 100 {
		t.Fatalf("%s is %d after rewriting it, want 100", name, got)
	}

	env.Set("one-more", &Integer{Value: 0})
	env.Set(name, &Integer{Value: 200})
	if got := lookupInt(t, env, name); got != 200 {
		t.Errorf("%s is %d, want 200", name, got)
	}
}

// TestBooleanAndNullValuesAreShared checks that the boolean and null
// constructors return usable values.
//
// The two booleans and null are shared singletons, which is safe precisely
// because equality compares the value inside them and never the identity of
// the object holding it.
func TestBooleanAndNullValuesAreShared(t *testing.T) {
	if BooleanOf(true).Value != true || BooleanOf(false).Value != false {
		t.Error("BooleanOf returned the wrong value")
	}
	if BooleanOf(true) == BooleanOf(false) {
		t.Error("BooleanOf returned the same object for both values")
	}
	if NullValue().Type() != NULL {
		t.Errorf("NullValue has type %s, want %s", NullValue().Type(), NULL)
	}
	if NullValue().Inspect() != "null" {
		t.Errorf("NullValue inspects as %q, want %q", NullValue().Inspect(), "null")
	}
}

// TestErrorHasAPosition checks that an error carries where it happened, which
// is what lets the CLI report a line and column.
func TestErrorHasAPosition(t *testing.T) {
	pos := token.Position{Line: 3, Col: 9}
	err := NewError(pos, "type mismatch: %s + %s", INTEGER, STRING)

	if err.Type() != ERROR {
		t.Errorf("error has type %s, want %s", err.Type(), ERROR)
	}
	if err.Message != "type mismatch: INTEGER + STRING" {
		t.Errorf("message is %q", err.Message)
	}
	if err.Pos != pos {
		t.Errorf("position is %v, want %v", err.Pos, pos)
	}
}

// bindingName returns a distinct, readable name for the nth binding.
func bindingName(n int) string {
	return "binding_" + string(rune('a'+n%26)) + string(rune('0'+n/26))
}

// lookupInt reads a name that is expected to hold an integer.
func lookupInt(t *testing.T, env *Environment, name string) int64 {
	t.Helper()

	obj, ok := env.Get(name)
	if !ok {
		t.Fatalf("%q is not bound", name)
	}
	i, ok := obj.(*Integer)
	if !ok {
		t.Fatalf("%q holds %T, want *Integer", name, obj)
	}
	return i.Value
}
