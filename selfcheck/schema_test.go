package selfcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// asMap parses a schema (input or output) for structural comparison. JSON
// objects decode to map[string]any, so key order never matters.
func asMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("not a JSON object: %s: %v", raw, err)
	}
	return m
}

// The four cases mirror ../tests/plain_schema_empty_enum_test.go exactly,
// so this standalone module and the upstream regression test cannot drift.
const upstreamCases = `{
  "type": "object",
  "properties": {
    "emptyEnum":  {"type": "string", "enum": ["", "minimal", "low", "medium", "high"]},
    "emptyConst": {"type": "string", "const": ""},
    "keptEnum":   {"type": "string", "enum": ["a", "b"]},
    "mixedEnum":  {"type": "string", "enum": ["a", 1]}
  }
}`

func wantPatched(t *testing.T) map[string]any {
	t.Helper()
	return asMap(t, json.RawMessage(`{
  "type": "object",
  "properties": {
    "emptyEnum":  {"type": "string"},
    "emptyConst": {"type": "string"},
    "keptEnum":   {"type": "string", "enum": ["a", "b"]},
    "mixedEnum":  {"type": "string"}
  }
}`))
}

func TestLegacyReproducesTheBug(t *testing.T) {
	got := PlainSchema(json.RawMessage(upstreamCases), Legacy)
	paths := EmptyEnumPaths(got)
	if len(paths) == 0 {
		t.Fatalf("legacy output should contain an empty enum value, got %s", got)
	}
	// Both the enum branch and the const branch must be observable in the
	// unpatched behaviour; otherwise the test suite is not actually testing
	// both halves of the fix.
	var sawEnum, sawConst bool
	for _, p := range paths {
		if strings.Contains(p, "emptyEnum") {
			sawEnum = true
		}
		if strings.Contains(p, "emptyConst") {
			sawConst = true
		}
	}
	if !sawEnum || !sawConst {
		t.Fatalf("expected empty enums from both branches, got %s", ErrorShape(paths))
	}
}

func TestPatchedMatchesUpstreamExpectation(t *testing.T) {
	got := asMap(t, PlainSchema(json.RawMessage(upstreamCases), Patched))
	want := wantPatched(t)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("patched output mismatch\n got: %v\nwant: %v", got, want)
	}
	if HasEmptyEnumValue(PlainSchema(json.RawMessage(upstreamCases), Patched)) {
		t.Fatal("patched output still contains an empty enum value")
	}
}

func TestWholeEnumKeywordIsDroppedNotFiltered(t *testing.T) {
	// Design decision in the patch: dropping only the empty member would let
	// the model still answer with "" — which Google rejects a second time.
	// The entire enum keyword must disappear.
	in := json.RawMessage(`{"type":"string","enum":["","minimal","low"]}`)
	got := asMap(t, PlainSchema(in, Patched))
	if _, ok := got["enum"]; ok {
		t.Fatalf("enum keyword must be absent entirely, got %v", got)
	}
	if !reflect.DeepEqual(got, asMap(t, json.RawMessage(`{"type":"string"}`))) {
		t.Fatalf("unexpected output %v", got)
	}
}

func TestEmptyConstDoesNotBecomeEnum(t *testing.T) {
	// plainSchema turns a string const into a one-element enum. Without the
	// guard on the const branch, const:"" emits enum:[""] and defeats the
	// enum-branch fix.
	in := json.RawMessage(`{"type":"string","const":""}`)
	got := PlainSchema(in, Patched)
	if HasEmptyEnumValue(got) {
		t.Fatalf("empty const still produces a rejected enum: %s", got)
	}
	if !reflect.DeepEqual(asMap(t, got), asMap(t, json.RawMessage(`{"type":"string"}`))) {
		t.Fatalf("unexpected output %s", got)
	}
}

func TestConstOverridesEnumIsAlsoGuarded(t *testing.T) {
	in := json.RawMessage(`{"type":"string","enum":["a","b"],"const":""}`)
	got := PlainSchema(in, Patched)
	if HasEmptyEnumValue(got) {
		t.Fatalf("const override re-introduced an empty enum: %s", got)
	}
	// The patched const branch leaves the valid enum computed by the enum
	// branch untouched; only the empty const is discarded.
	gotEnum := asMap(t, got)["enum"]
	if !reflect.DeepEqual(gotEnum, []any{"a", "b"}) {
		t.Fatalf("patched const override should keep the valid enum, got %v", got)
	}
	legacy := asMap(t, PlainSchema(in, Legacy))
	enum, ok := legacy["enum"].([]any)
	if !ok || len(enum) != 1 || enum[0] != "" {
		t.Fatalf("legacy const override should produce enum:[\"\"], got %v", legacy)
	}
}

func TestKeptEnumIsUnchanged(t *testing.T) {
	in := json.RawMessage(`{"type":"string","enum":["a","b"]}`)
	for _, mode := range []Mode{Legacy, Patched} {
		got := asMap(t, PlainSchema(in, mode))
		want := asMap(t, in)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("mode=%v changed a valid enum: %v", mode, got)
		}
	}
}

func TestNonStringEnumIsDroppedInBothModes(t *testing.T) {
	// Existing plainSchema behaviour: one non-string member means the whole
	// keyword is dropped. The fix must not alter that rule.
	in := json.RawMessage(`{"type":"string","enum":["a",1]}`)
	for _, mode := range []Mode{Legacy, Patched} {
		got := asMap(t, PlainSchema(in, mode))
		if _, ok := got["enum"]; ok {
			t.Fatalf("mode=%v kept a mixed enum: %v", mode, got)
		}
	}
}

func TestNestedSchemaAndArraysRecurse(t *testing.T) {
	in := json.RawMessage(`{
      "type": "object",
      "properties": {
        "modelOverride": {
          "type": "object",
          "properties": {
            "effort": {"type": "string", "enum": ["", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"]}
          }
        },
        "list": {"type": "array", "items": {"type": "string", "enum": [""]}}
      }
    }`)
	legacy := PlainSchema(in, Legacy)
	if len(EmptyEnumPaths(legacy)) < 2 {
		t.Fatalf("legacy recursion lost the empty enums: %s", ErrorShape(EmptyEnumPaths(legacy)))
	}
	patched := PlainSchema(in, Patched)
	if HasEmptyEnumValue(patched) {
		t.Fatalf("patched recursion left an empty enum: %s", ErrorShape(EmptyEnumPaths(patched)))
	}
	effort := asMap(t, patched)["properties"].(map[string]any)["modelOverride"].(map[string]any)["properties"].(map[string]any)["effort"].(map[string]any)
	if _, ok := effort["enum"]; ok {
		t.Fatalf("nested effort enum should be dropped, got %v", effort)
	}
}

// The real shape from the 2026-11-04 incident: Cindy's routine_save tool
// declares modelOverride.effort with "" as its first enum member, meaning
// "follow the companion default".
func TestRealWorldRoutineSaveShape(t *testing.T) {
	in := json.RawMessage(`{
      "type": "object",
      "properties": {
        "enabled":       {"type": "boolean"},
        "modelOverride": {
          "type": "object",
          "properties": {
            "effort": {"type": "string", "enum": ["", "minimal", "low", "medium", "high", "xhigh", "max", "ultra"]},
            "model":  {"type": "string"}
          }
        }
      }
    }`)
	legacy := PlainSchema(in, Legacy)
	if !HasEmptyEnumValue(legacy) {
		t.Fatalf("expected the bug to reproduce for the reported schema: %s", legacy)
	}
	patched := PlainSchema(in, Patched)
	if HasEmptyEnumValue(patched) {
		t.Fatalf("patched schema would still be refused: %s", ErrorShape(EmptyEnumPaths(patched)))
	}
}

func TestUnknownKeywordsAreDropped(t *testing.T) {
	in := json.RawMessage(`{"type":"string","title":"x","default":"y","format":"date","enum":["","a"]}`)
	got := asMap(t, PlainSchema(in, Patched))
	want := asMap(t, json.RawMessage(`{"type":"string"}`))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected output %v", got)
	}
}

func TestMalformedInputIsSafe(t *testing.T) {
	got := PlainSchema(json.RawMessage(`{not json`), Patched)
	if string(got) != "null" {
		t.Fatalf("expected null for malformed input, got %s", got)
	}
	if HasEmptyEnumValue(got) {
		t.Fatal("malformed input must not look like a rejected schema")
	}
}

// ---- Repository integrity: the shipped artifacts must match this model ----

func repoFile(t *testing.T, parts ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{".."}, parts...)...)
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read %s: %v", p, err)
	}
	return string(b)
}

func TestFormatPatchCarriesBothGuardsAndCommitMessage(t *testing.T) {
	patch := repoFile(t, "patches", "0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch")
	for _, want := range []string{
		"Subject: [PATCH] gateway: reject empty enum values in Antigravity tool schemas",
		"plainSchema()",
		"docs/contributors.md",
		"please credit the",
		`if s, ok := e.(string); ok && s != "" {`,
		`if s, ok := c.(string); ok && s != "" {`,
		"new file mode 100644",
		"+++ b/internal/gateway/plain_schema_empty_enum_test.go",
	} {
		if !strings.Contains(patch, want) {
			t.Errorf("format patch is missing %q", want)
		}
	}
	if strings.Contains(patch, "\r") {
		t.Error("format patch must be LF-only")
	}
	if !strings.HasSuffix(patch, "\n") {
		t.Error("format patch must end with a newline")
	}
}

func TestPlainPatchIsAValidDiff(t *testing.T) {
	patch := repoFile(t, "patches", "empty-enum.patch")
	for _, want := range []string{
		"diff --git a/internal/gateway/codeassist.go b/internal/gateway/codeassist.go",
		"@@ -1090,11 +1090,14 @@",
		"+\t\t\t\t\t\t\tif s, ok := e.(string); ok && s != \"\" {",
		"+\t\t\t\tif s, ok := c.(string); ok && s != \"\" {",
		"new file mode 100644",
	} {
		if !strings.Contains(patch, want) {
			t.Errorf("plain patch is missing %q", want)
		}
	}
	if strings.Contains(patch, "\r") {
		t.Error("plain patch must be LF-only")
	}
}

func TestUpstreamRegressionTestIsShipped(t *testing.T) {
	test := repoFile(t, "tests", "plain_schema_empty_enum_test.go")
	for _, want := range []string{
		"package gateway",
		"func TestPlainSchemaDropsEnumWithEmptyValue",
		"emptyEnum",
		"emptyConst",
		"keptEnum",
		"mixedEnum",
		"plainSchema(json.RawMessage(in))",
		"reflect.DeepEqual",
	} {
		if !strings.Contains(test, want) {
			t.Errorf("upstream regression test is missing %q", want)
		}
	}
}
