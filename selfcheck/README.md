# selfcheck

A dependency-free Go module that models the two `plainSchema()` branches the
patch changes. It lets this repository prove the bug and the fix **without a
magpie checkout, without network, without a gateway and without model tokens**.

```bash
go test ./... -count=1
go vet ./...
```

Or from the repository root: `./scripts/verify.sh`.

## What it is, and what it is not

- `schema.go` is an original, minimal model. It implements the `enum` and
  `const` handling plus recursion through `properties` / `items`, and drops
  keywords outside the subset — mirroring the relevant rules of magpie's
  `internal/gateway.plainSchema`. It contains **no magpie source code**.
- `Mode` selects the behaviour: `Legacy` reproduces the bug, `Patched` matches
  the patch.
- `EmptyEnumPaths()` / `HasEmptyEnumValue()` detect the exact condition Google
  rejects, so a test can assert "would this payload be refused?".
- The authoritative test against the real function is
  `../tests/plain_schema_empty_enum_test.go`, which must run inside magpie's
  `internal/gateway` package. `TestPatchedMatchesUpstreamExpectation` keeps
  this module's expectation byte-for-byte aligned with those four cases.
- `TestFormatPatchCarriesBothGuardsAndCommitMessage`,
  `TestPlainPatchIsAValidDiff` and `TestUpstreamRegressionTestIsShipped` check
  that the shipped artifacts have not drifted from what is tested here.

Not modelled: `$ref` / `$defs` resolution, `localRef`, `selfRef`, `refPath`,
TypeScript-style `anyOf` unions, depth-limit constants beyond a simple guard,
and magpie's `null` handling. None of those participate in the empty-enum bug.

## Test list

| Test | Proves |
| --- | --- |
| `TestLegacyReproducesTheBug` | both branches emit an empty enum before the fix |
| `TestPatchedMatchesUpstreamExpectation` | patched output equals the upstream regression test's expectation |
| `TestWholeEnumKeywordIsDroppedNotFiltered` | the whole keyword goes, not just `""` |
| `TestEmptyConstDoesNotBecomeEnum` | `const: ""` cannot produce `enum:[""]` |
| `TestConstOverridesEnumIsAlsoGuarded` | the const branch is the sibling that also needed a guard |
| `TestKeptEnumIsUnchanged` | valid enums are identical in both modes |
| `TestNonStringEnumIsDroppedInBothModes` | the existing mixed-type rule is untouched |
| `TestNestedSchemaAndArraysRecurse` | nested `properties` and `items` are handled |
| `TestRealWorldRoutineSaveShape` | the reported `modelOverride.effort` shape is fixed |
| `TestUnknownKeywordsAreDropped` | keywords outside the subset are dropped |
| `TestMalformedInputIsSafe` | invalid JSON cannot look like a rejected schema |
| `TestFormatPatchCarriesBothGuardsAndCommitMessage` | mbox has both guards, subject, credit note, LF, trailing newline |
| `TestPlainPatchIsAValidDiff` | plain diff has the expected hunks and LF |
| `TestUpstreamRegressionTestIsShipped` | `tests/` file is present and complete |
