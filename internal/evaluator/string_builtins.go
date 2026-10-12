package evaluator

import (
	"strings"
	"unicode/utf8"

	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// The string builtins. `len` and `+` are enough to join a line together and
// not much else, which is why a program that wants to look at the characters
// of a string — a tokeniser, a parser, a filter — had nowhere to start.
//
// Every index here counts characters rather than bytes, to match `len`, and
// every slice clamps rather than failing. Clamping is deliberate: a parser
// walking a string one character at a time should be able to ask for the next
// five characters near the end and get the three that are left, not an error
// it has to guard against on every call.
const (
	builtinOrd    = "ord"
	builtinChr    = "chr"
	builtinSubstr = "substr"
	builtinSplit  = "split"
	builtinJoin   = "join"
	builtinFind   = "find"
	builtinUpper  = "upper"
	builtinLower  = "lower"
)

// maxCodePoint is the largest value `chr` will turn into a character.
const maxCodePoint = 0x10FFFF

func (e *Evaluator) stringBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		builtinOrd: {
			Name: builtinOrd,
			// The code point of the first character. An empty string has no
			// first character, so it is an error rather than a sentinel: the
			// caller has the length and can test it.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinOrd, args, 1, token.Position{}); err != nil {
					return err
				}
				s, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "argument to `ord` must be a string, got %s", args[0].Type())
				}
				if s.Value == "" {
					return object.NewError(token.Position{}, "argument to `ord` must not be empty")
				}
				r, _ := utf8.DecodeRuneInString(s.Value)
				return &object.Integer{Value: int64(r)}
			},
		},

		builtinChr: {
			Name: builtinChr,
			// The inverse of `ord`. Surrogate code points and anything above
			// the last valid one are rejected, since Go would silently replace
			// them with U+FFFD and hand back a character nobody asked for.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinChr, args, 1, token.Position{}); err != nil {
					return err
				}
				n, ok := args[0].(*object.Integer)
				if !ok {
					return object.NewError(token.Position{}, "argument to `chr` must be an integer, got %s", args[0].Type())
				}
				if n.Value < 0 || n.Value > maxCodePoint {
					return object.NewError(token.Position{}, "argument to `chr` is not a valid code point: %d", n.Value)
				}
				if n.Value >= 0xD800 && n.Value <= 0xDFFF {
					return object.NewError(token.Position{}, "argument to `chr` is a surrogate code point: %d", n.Value)
				}
				return &object.String{Value: string(rune(n.Value))}
			},
		},

		builtinSubstr: {
			Name: builtinSubstr,
			// substr(s, start) is the rest of the string; substr(s, start, n)
			// is n characters of it. Both clamp: a start past the end gives the
			// empty string and a length past the end gives what is left.
			Fn: func(args []object.Object) object.Object {
				if len(args) != 2 && len(args) != 3 {
					return object.NewError(token.Position{}, "wrong number of arguments to `substr`: want 2 or 3, got %d", len(args))
				}

				s, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "first argument to `substr` must be a string, got %s", args[0].Type())
				}
				start, ok := args[1].(*object.Integer)
				if !ok {
					return object.NewError(token.Position{}, "second argument to `substr` must be an integer, got %s", args[1].Type())
				}

				runes := []rune(s.Value)

				from := clamp(start.Value, 0, int64(len(runes)))
				to := int64(len(runes))
				if len(args) == 3 {
					length, ok := args[2].(*object.Integer)
					if !ok {
						return object.NewError(token.Position{}, "third argument to `substr` must be an integer, got %s", args[2].Type())
					}
					to = clamp(from+length.Value, from, int64(len(runes)))
				}

				return &object.String{Value: string(runes[from:to])}
			},
		},

		builtinSplit: {
			Name: builtinSplit,
			// An empty separator splits into characters, which is how a
			// program turns a string into something it can index.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinSplit, args, 2, token.Position{}); err != nil {
					return err
				}
				s, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "first argument to `split` must be a string, got %s", args[0].Type())
				}
				sep, ok := args[1].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "second argument to `split` must be a string, got %s", args[1].Type())
				}

				parts := splitString(s.Value, sep.Value)
				elements := make([]object.Object, len(parts))
				for i, p := range parts {
					elements[i] = &object.String{Value: p}
				}
				return &object.Array{Elements: elements}
			},
		},

		builtinJoin: {
			Name: builtinJoin,
			// Each element is rendered as `str` would render it, so joining an
			// array of numbers is not a type error.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinJoin, args, 2, token.Position{}); err != nil {
					return err
				}
				arr, ok := args[0].(*object.Array)
				if !ok {
					return object.NewError(token.Position{}, "first argument to `join` must be an array, got %s", args[0].Type())
				}
				sep, ok := args[1].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "second argument to `join` must be a string, got %s", args[1].Type())
				}

				parts := make([]string, len(arr.Elements))
				for i, el := range arr.Elements {
					parts[i] = el.Inspect()
				}
				return &object.String{Value: strings.Join(parts, sep.Value)}
			},
		},

		builtinFind: {
			Name: builtinFind,
			// The character index of the first occurrence, or -1. The index is
			// in characters, not bytes, so it can be handed straight to
			// substr; strings.Index reports a byte offset, which has to be
			// counted back into characters first.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinFind, args, 2, token.Position{}); err != nil {
					return err
				}
				s, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "first argument to `find` must be a string, got %s", args[0].Type())
				}
				sub, ok := args[1].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "second argument to `find` must be a string, got %s", args[1].Type())
				}

				byteIndex := strings.Index(s.Value, sub.Value)
				if byteIndex < 0 {
					return &object.Integer{Value: -1}
				}
				return &object.Integer{Value: int64(utf8.RuneCountInString(s.Value[:byteIndex]))}
			},
		},

		builtinUpper: {
			Name: builtinUpper,
			Fn: func(args []object.Object) object.Object {
				return mapCase(builtinUpper, args, strings.ToUpper)
			},
		},

		builtinLower: {
			Name: builtinLower,
			Fn: func(args []object.Object) object.Object {
				return mapCase(builtinLower, args, strings.ToLower)
			},
		},
	}
}

// splitString splits s on sep, with an empty separator meaning "characters".
func splitString(s, sep string) []string {
	if sep != "" {
		return strings.Split(s, sep)
	}

	runes := []rune(s)
	parts := make([]string, len(runes))
	for i, r := range runes {
		parts[i] = string(r)
	}
	return parts
}

// mapCase applies a case conversion to a builtin's single string argument.
func mapCase(name string, args []object.Object, convert func(string) string) object.Object {
	if err := expectArgs(name, args, 1, token.Position{}); err != nil {
		return err
	}
	s, ok := args[0].(*object.String)
	if !ok {
		return object.NewError(token.Position{}, "argument to `%s` must be a string, got %s", name, args[0].Type())
	}
	return &object.String{Value: convert(s.Value)}
}

// clamp constrains v to the range [low, high].
func clamp(v, low, high int64) int64 {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}
