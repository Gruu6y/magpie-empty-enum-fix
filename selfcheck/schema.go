// Package selfcheck is a standalone, dependency-free model of the two
// plainSchema branches that the patch in ../patches changes.
//
// It deliberately contains no magpie source code: it needs no magpie
// checkout, no network, no gateway and no model tokens. It exists so that
// `go test ./...` can be run straight from this repository to demonstrate
// the bug and verify the fix.
//
// The authoritative regression test against the real function is
// ../tests/plain_schema_empty_enum_test.go, which runs inside magpie's
// internal/gateway package.
package selfcheck

import (
	"encoding/json"
	"sort"
	"strings"
)

// Mode selects which behaviour of the enum/const branches to model.
type Mode bool

const (
	// Legacy mirrors magpie before the patch: an empty string is a valid Go
	// string, so the enum keyword is forwarded to Google, which rejects it.
	Legacy Mode = false
	// Patched mirrors magpie after the patch: the whole enum keyword is
	// dropped when any member is not a usable, non-empty string.
	Patched Mode = true
)

const maxDepth = 64

// PlainSchema is a minimal, standalone model of the enum/const/structure
// branches of magpie's internal/gateway.plainSchema.
//
// It keeps the keywords that matter for this bug (type, description,
// nullable, required) and recurses into properties and items, exactly as
// plainSchema does. Keywords outside that subset are dropped, which mirrors
// plainSchema's rule that Antigravity refuses keywords it does not know.
// $ref/$defs resolution is intentionally not modelled here; none of the
// schemas under test use it.
func PlainSchema(raw json.RawMessage, mode Mode) json.RawMessage {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return json.RawMessage("null")
	}
	out, err := json.Marshal(walk(v, mode, 0))
	if err != nil {
		return json.RawMessage("null")
	}
	return out
}

func walk(v any, mode Mode, depth int) any {
	if depth > maxDepth {
		return nil
	}
	switch x := v.(type) {
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			out = append(out, walk(e, mode, depth+1))
		}
		return out
	case map[string]any:
		out := make(map[string]any)
		for k, val := range x {
			switch k {
			case "enum":
				if es, ok := val.([]any); ok {
					strs := make([]any, 0, len(es))
					for _, e := range es {
						if s, ok := e.(string); ok && (mode == Legacy || s != "") {
							strs = append(strs, s)
						}
					}
					// plainSchema drops the whole keyword when any member
					// failed the check, rather than emitting a shortened list.
					if len(strs) == len(es) {
						out[k] = strs
					}
				}
			case "properties":
				if props, ok := val.(map[string]any); ok {
					p := make(map[string]any, len(props))
					for name, schema := range props {
						p[name] = walk(schema, mode, depth+1)
					}
					out[k] = p
				}
			case "items":
				out[k] = walk(val, mode, depth+1)
			case "type", "description", "nullable", "required":
				out[k] = val
			default:
				// A keyword outside the subset: dropped, as in plainSchema.
			}
		}
		// A string const becomes a one-element enum and overwrites the enum
		// computed above. Both branches need the empty-string guard.
		if c, ok := x["const"]; ok {
			if s, ok := c.(string); ok && (mode == Legacy || s != "") {
				out["enum"] = []any{s}
			}
		}
		return out
	default:
		return v
	}
}

// EmptyEnumPaths returns the dotted path of every enum array that contains
// an empty string. That is precisely what Google rejects with
// "GenerateContentRequest.tools[0].function_declarations[N].parameters...enum[0]: cannot be empty".
// Paths are sorted so the result is stable.
func EmptyEnumPaths(raw json.RawMessage) []string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil
	}
	found := map[string]bool{}
	collect(v, "", found, 0)
	paths := make([]string, 0, len(found))
	for p := range found {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return paths
}

func collect(v any, path string, found map[string]bool, depth int) {
	if depth > maxDepth {
		return
	}
	switch x := v.(type) {
	case []any:
		for i, e := range x {
			collect(e, path+"["+itoa(i)+"]", found, depth+1)
		}
	case map[string]any:
		if es, ok := x["enum"].([]any); ok {
			for _, e := range es {
				if s, ok := e.(string); ok && s == "" {
					found[path+".enum"] = true
					break
				}
			}
		}
		for k, val := range x {
			next := k
			if path != "" {
				next = path + "." + k
			}
			if k == "properties" {
				if props, ok := val.(map[string]any); ok {
					for name, schema := range props {
						collect(schema, joinProperty(next, name), found, depth+1)
					}
					continue
				}
			}
			collect(val, next, found, depth+1)
		}
	}
}

func joinProperty(prefix, name string) string {
	return prefix + ".properties[" + name + "]"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// HasEmptyEnumValue reports whether a serialized function-declaration
// parameter schema would be refused by Google for an empty enum value.
func HasEmptyEnumValue(raw json.RawMessage) bool {
	return len(EmptyEnumPaths(raw)) > 0
}

// ErrorShape renders the paths the way Google's error message does, for use
// in test failure output and in docs.
func ErrorShape(paths []string) string {
	if len(paths) == 0 {
		return "no empty enum values"
	}
	return strings.Join(paths, ", ") + ": enum[0]: cannot be empty"
}
