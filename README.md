# magpie-empty-enum-fix

A standalone, source-free repository for one small bug in
[yetone/magpie](https://github.com/yetone/magpie): `plainSchema()` forwards an
empty string inside a tool-schema `enum` to Antigravity, and Google rejects
the whole request with `enum[0]: cannot be empty`.

This repository carries the patch, its regression test, an independently
runnable self-check suite, and the evidence behind them. It is **not** a fork
of magpie, it does **not** contain magpie's source tree, and applying it never
requires touching a running magpie instance or `~/.config/magpie`.

中文说明见 [README.zh-CN.md](README.zh-CN.md).

---

## The bug

Google's `GenerateContentRequest` validation refuses a function declaration
whose `enum` contains an empty string:

```
Antigravity: * GenerateContentRequest.tools[0].function_declarations[N]
  .parameters.properties[modelOverride].properties[effort]
  .enum[0]: cannot be empty
```

`internal/gateway/plainSchema()` (in magpie's `internal/gateway/codeassist.go`)
converts a JSON Schema into the OpenAPI-style subset Antigravity accepts. Its
`enum` branch only dropped non-string members:

```go
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

`""` **is** a valid Go string, so it was forwarded. An empty enum member is a
legitimate JSON Schema value (commonly meaning "unset / follow the default"),
so the caller is not doing anything wrong either. The same function also turns
a string `const` into a one-element `enum`, so `const: ""` produced the
identical invalid payload even after the `enum` branch was fixed.

### Why it looks like a rate-limit / risk-control problem

magpie classifies this 400 as `shapeRefused` and retries with another group
member. The turn still ends in **200**, but the first token arrives much later
and the Antigravity seat is wasted. On one 2026-11-04 routing ledger:

| caller | Antigravity attempts | outcome |
| --- | --- | --- |
| agent carrying the offending tool | 29 records | 29 got at least one Antigravity 400 |
| agent carrying the offending tool, successful records | 15 | 0 carried the tool |
| another agent without that tool | 75 | 74 succeeded directly |

Those numbers are from one machine on one day, not a general benchmark. They
are evidence of the mechanism, not a performance claim.

---

## The fix

Two guards, both in `plainSchema()`:

```diff
-					if s, ok := e.(string); ok {
+					if s, ok := e.(string); ok && s != "" {
...
-				if s, ok := c.(string); ok {
+				if s, ok := c.(string); ok && s != "" {
```

When any member is an empty string the **whole `enum` keyword is dropped**,
following `plainSchema`'s existing rule for enums containing a non-string
value. Filtering only the empty member out of the list would be wrong: the
model could still answer with `""` and be rejected a second time. The same
guard is applied to the `const` branch because it is the sibling of the same
bug.

See [patches/empty-enum.patch](patches/empty-enum.patch) for the plain diff and
[patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch](patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch)
for a `git format-patch` mbox with a full commit message.

---

## Repository layout

| path | what it is |
| --- | --- |
| `README.md`, `README.zh-CN.md` | this document and the Chinese companion |
| `patches/empty-enum.patch` | plain unified diff, `git apply` / `patch -p1` |
| `patches/0001-…patch` | `git format-patch` mbox with a complete commit message, for `git am` |
| `tests/plain_schema_empty_enum_test.go` | copy of the regression test added by the patch; runs **inside** magpie's `internal/gateway` |
| `selfcheck/` | dependency-free Go module modelling the two changed branches; runs **here** |
| `scripts/verify.sh` | one-shot local verification of this repository |
| `scripts/apply.sh` | check or apply the patch against a magpie checkout you name |
| `.github/workflows/ci.yml` | CI: gofmt, vet, selfcheck tests, patch integrity |
| `docs/VERIFICATION.md` | exact commands and their real output |
| `docs/UPSTREAM-REPORT.md` | ready-to-paste report for magpie's Discord / issue |
| `LICENSE`, `NOTICE` | MIT for this repository's own material; magpie attribution |

---

## Quick start (no magpie needed)

```bash
./scripts/verify.sh
```

Or directly:

```bash
cd selfcheck
go test ./... -count=1
go vet ./...
```

`selfcheck/` contains **no magpie source code**. It models the enum/const
branches in isolation so the bug and the fix can be demonstrated with zero
dependencies, zero network and zero model tokens. Its tests also verify that
the shipped patches contain both guards and that the upstream regression test
is present.

The authoritative test against the real function is
`tests/plain_schema_empty_enum_test.go`, which must be run inside a magpie
checkout.

---

## Applying the patch

Give `apply.sh` an explicit magpie checkout path. It never defaults to a
running installation, never touches `~/.config/magpie`, and never restarts a
service.

```bash
# read-only: verify the patch applies cleanly against HEAD
./scripts/apply.sh /path/to/magpie --check

# apply to the working tree (no commit)
./scripts/apply.sh /path/to/magpie --apply

# apply as a commit, preserving the full commit message
./scripts/apply.sh /path/to/magpie --am
```

Or by hand:

```bash
cd /path/to/magpie
git apply --check /path/to/magpie-empty-enum-fix/patches/0001-…patch
git am /path/to/magpie-empty-enum-fix/patches/0001-…patch
go test ./internal/gateway/ -run TestPlainSchema -count=1
```

`git am` needs a configured git identity; `git apply` does not. That is why
`--check` and `--apply` work on a bare clone without any git config.

Baseline: this patch was produced against magpie `a3768cdd1113118998863ed36bd6dd77c8e9e2cc`.
If upstream has since changed `internal/gateway/codeassist.go`, rebase the two
hunks rather than forcing a 3-way merge.

---

## Verification

`docs/VERIFICATION.md` records the exact commands and their real output on
Go 1.27.0 / windows-amd64 in an **isolated sparse clone**:

- without the patch, `TestPlainSchemaDropsEnumWithEmptyValue` fails with
  `enum:[""]`; with the patch it passes
- `gofmt`, `go vet ./internal/gateway/`, `go build ./internal/gateway/` pass
- `git apply --check`, `git apply`, `patch --dry-run -p1` pass on a clean copy
- the full `internal/gateway` suite has 9 pre-existing red tests on that
  machine (Windows socket handover, sandbox env, proxy/WSL port 3425 conflict);
  the **identical 9** fail with the fix reverted, so they are unrelated
- full-repository and cross-platform (`GOOS=linux`) builds were **not** run;
  see the document for the complete list
- no live gateway was contacted, no production config was touched, no model
  request was sent

---

## Upstream status

magpie's `README.md` states:

> **This repository doesn't take pull requests.** Only maintainers can open
> them. If you'd like a fix or a feature, describe it on Discord and we'll
> build it.

So there is deliberately **no pull request** here. `docs/UPSTREAM-REPORT.md`
is a ready-to-paste description for the Discord / an issue. Per magpie's
`docs/contributors.md`, if a maintainer uses the root cause, error text or
test fixture, the commit should credit the reporter.

This repository has no remote configured until the publication channel is
chosen by its owner.

---

## License

This repository's own material (selfcheck module, scripts, docs, tests) is
[MIT](LICENSE), Copyright (c) 2026 Gruu6y.

The patch modifies magpie's own MIT-licensed code, Copyright (c) 2026 yetone;
that code is not redistributed here, only the diff. See [NOTICE](NOTICE) for
the full attribution and the exact scope of each file.
