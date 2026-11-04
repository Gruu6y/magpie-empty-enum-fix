# Antigravity: `plainSchema` sends an empty `enum` value, Google rejects the whole request

**What happened.** An agent whose tool schema contains a string `enum` whose first
value is `""` (an empty choice meaning "follow the default") gets every Antigravity
request rejected with:

```
Antigravity: * GenerateContentRequest.tools[0].function_declarations[24].parameters.properties[modelOverride].properties[effort].enum[0]: cannot be empty
```

The index (`[24]`, `[26]`, `[32]`) just tracks how many tools the client sent.
magpie classifies this 400 as `shapeRefused` and retries with another group member,
so the turn eventually returns 200 — but the first token takes far longer and the
Antigravity seat is wasted. This is a schema-validation failure, **not** a quota,
proxy, account or location issue.

**Where.** `internal/gateway/codeassist.go`, `plainSchema()`:

```go
case "enum":
	// Google takes string enums only
	if es, ok := v.([]any); ok {
		strs := make([]any, 0, len(es))
		for _, e := range es {
			if s, ok := e.(string); ok {
				strs = append(strs, s)
			}
		}
		if len(strs) == len(es) {
			out[k] = strs
		}
	}
```

An empty string is a valid `string`, so it passes through into the Antigravity
`functionDeclarations`. Google refuses the entire `GenerateContentRequest` over it.
The same file also writes `const` as a one-element `enum`, so `const: ""` produces
the identical `enum: [""]` even when the `enum` branch is fixed — the sibling of
the same bug.

**Minimal reproduction** (in-memory, no network, no real magpie settings):

```go
in := `{"type":"object","properties":{"effort":{"type":"string","enum":["","minimal","low","medium","high"]}}}`
// plainSchema(in) currently returns {"type":"object","properties":{"effort":{"type":"string","enum":["","minimal","low","medium","high"]}}}
// Google: * ...enum[0]: cannot be empty
```

**Fix.** Two lines, in `plainSchema()`:

```go
-					if s, ok := e.(string); ok {
+					if s, ok := e.(string); ok && s != "" {
```
```go
-				if s, ok := c.(string); ok {
+				if s, ok := c.(string); ok && s != "" {
```

This drops the **whole** `enum` keyword rather than filtering `""` out of the
list: with an empty choice removed from the array, a model could still answer with
`""` and be rejected a second time. Dropping the keyword follows what
`plainSchema` already does for any enum containing a non-string value. Valid
enums (`["a","b"]`), mixed-type enums and every other keyword are untouched.
The comment above the branch is updated to say why.

**Regression test.** `internal/gateway/plain_schema_empty_enum_test.go`,
`TestPlainSchemaDropsEnumWithEmptyValue`, covers four cases with
`reflect.DeepEqual`: an enum with `""`, `const: ""`, a valid enum kept as-is, and
the existing mixed-type rule kept as-is. Without the fix it fails:

```
--- FAIL: TestPlainSchemaDropsEnumWithEmptyValue (0.00s)
    plain_schema_empty_enum_test.go:35: got {"properties":{"emptyConst":{"enum":[""],"type":"string"},"emptyEnum":{"enum":["","minimal","low","medium","high"],"type":"string"},"keptEnum":{"enum":["a","b"],"type":"string"},"mixedEnum":{"type":"string"}},"type":"object"}
```

**Verification** (Go 1.27.0, windows/amd64, on an isolated sparse clone at
`a3768cdd1113118998863ed36bd6dd77c8e9e2cc`):

| Check | Result |
|---|---|
| `gofmt -l <both files>` | clean |
| `go test ./internal/gateway/ -run TestPlainSchemaDropsEnumWithEmptyValue -count=1` | PASS |
| `go test ./internal/gateway/ -run TestPlainSchema -count=1` | PASS (3 existing + 1 new) |
| `go vet ./internal/gateway/` | PASS |
| `go build ./internal/gateway/` | PASS |
| `go test ./internal/gateway/ -count=1` | 9 pre-existing red tests; **identical list with the fix reverted**, so unrelated to this change (Windows socket handover, `IS_SANDBOX`, fastjson escaping, codex User-Agent version). Full-suite `-tags nogui` and cross-platform vet/build not run. |
| `git apply --check` / `git apply` / `patch --dry-run -p1` on a clean copy of HEAD | all PASS |

No request was sent to a live gateway, no production config was read or written,
and the running magpie instance was not touched.

**Observed impact before the fix** (2026-11-04 routing ledger, one machine):
74/75 DSH requests to `antigravity/gemini-3.8-flash` returned 200; only 16/65
requests from the other agent did. All 29 of that agent's 400-carrying records
included the tool with the empty enum, and none of its 15 successful Antigravity
records did. Requests were comparable in size (~2.5k–4k sent tokens), and all
three Antigravity accounts were used by both agents with 0.2%–3.1% quota used.

**Standalone repository.** The description, patch, regression test, an
independent zero-dependency self-check module, verification scripts, CI
workflow and this report live in `magpie-empty-enum-fix`:

| File | Purpose |
|---|---|
| `patches/empty-enum.patch` | the whole change (2 files, 2501 bytes, LF), `git apply` / `patch -p1` |
| `patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch` | `git format-patch` mbox with the full commit message, for `git am` |
| `tests/plain_schema_empty_enum_test.go` | the new test verbatim |
| `selfcheck/` | dependency-free Go module modelling the two changed branches; `go test ./...` runs without a magpie checkout |
| `scripts/verify.sh`, `scripts/apply.sh` | local verification and a path-explicit `--check` / `--apply` / `--am` applier |
| `docs/VERIFICATION.md` | full command output, the 9-test baseline comparison, and the list of checks not run |
| `LICENSE`, `NOTICE` | MIT for the repository's own material; magpie attribution |

The repository is not a fork and ships no magpie source, only the diff. It has
no remote configured yet; the publication channel is the owner's call.

Per `docs/contributors.md` this repository does not take external PRs, so this is
a description plus a patch rather than a pull request. If you use the root cause,
the error text or the test fixture from this report, please credit the reporter in
the commit (`Co-authored-by: <login> <id>+<login>@users.noreply.github.com`).
