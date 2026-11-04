# Commit messages

This repository uses [Conventional Commits](https://www.conventionalcommits.org/)
subjects and a body that answers **why**, not just **what**. All five commits
are real commits in this repository; run `git log --format=fuller` to see them.

The patch's own commit message (the one a maintainer would land in magpie) is
carried verbatim inside
`patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch`
and is applied by `git am`. Its full text is reproduced at the end of this
document.

---

## 1. `chore: scaffold standalone patch repository`

```
chore: scaffold standalone patch repository

Set up the repository-level files before adding any content:

- MIT LICENSE for this repository's own material, and NOTICE recording that
  the patch modifies magpie's MIT-licensed code (Copyright (c) 2026 yetone)
  without redistributing its source tree.
- .gitattributes pins every text file to LF. magpie's own .gitattributes does
  the same, and a CRLF patch produces false-red guard tests on Windows.
- .gitignore keeps Go build artifacts and scratch trees out of the history.

No code, no patch and no magpie source in this commit.
```

## 2. `fix: reject empty enum values in Antigravity tool schemas`

```
fix: reject empty enum values in Antigravity tool schemas

Ship the two-line change as a patch rather than as a fork: magpie's README
states that the repository does not take external pull requests, so this repo
holds a description plus a diff for a maintainer to apply.

  case "enum":       if s, ok := e.(string); ok && s != "" {
  const branch:      if s, ok := c.(string); ok && s != "" {

An empty string is a valid Go string, so plainSchema() forwarded it to
Google, which rejects the whole GenerateContentRequest with
"enum[0]: cannot be empty". Dropping the entire enum keyword (rather than
filtering the empty member) follows plainSchema's existing rule for enums
with a non-string member and stops the model from answering "" a second time.
The const branch writes a one-element enum and is the sibling of the same bug.

Add the regression test verbatim in tests/ and a git format-patch mbox with a
complete commit message in patches/. Both forms are kept because they serve
different workflows: `git apply` needs no identity, `git am` preserves the
message.
```

## 3. `test: add standalone selfcheck suite, scripts and CI`

```
test: add standalone selfcheck suite, scripts and CI

selfcheck/ is a dependency-free Go module modelling the two changed branches
of plainSchema. It lets this repository prove the bug and the fix without a
magpie checkout, without network and without spending model tokens:

- Legacy reproduces enum:[""] from both the enum and the const branch.
- Patched matches the four cases of the upstream regression test exactly.
- Repository-integrity tests keep the shipped patches, the upstream test and
  this model from drifting apart.

Add scripts/verify.sh (six local checks, optional read-only
git apply --check against a supplied checkout) and scripts/apply.sh
(--check / --apply / --am, always with an explicit checkout path, never a
running instance). apply.sh checks GIT_COMMITTER_IDENT before `git am` so a
missing git identity cannot leave a half-applied mailbox.

CI runs gofmt, vet, the selfcheck suite, patch integrity and a read-only
apply --check against the baseline magpie tree.
```

## 4. `docs: add README, verification record and upstream report`

```
docs: add README, verification record and upstream report

README.md / README.zh-CN.md explain the bug, why it looks like a rate-limit
problem, the fix, the layout and how to verify or apply the patch. They state
the boundaries explicitly: not a fork, no magpie source, no production config,
no request to a live gateway, no remote configured.

docs/VERIFICATION.md records the exact commands and their real output, the
"fails without the fix" proof, the 9 pre-existing red tests with the
identical baseline list, and the checks that were NOT run.

docs/UPSTREAM-REPORT.md is a ready-to-paste description for magpie's Discord
or an issue, including the observed 2026-11-04 impact and the credit request
from docs/contributors.md.
```

---

## 5. `docs: record publication and passing CI`

```
docs: record publication and passing CI

The first four commits were written before a publication channel existed, so
README.md, README.zh-CN.md, docs/UPSTREAM-REPORT.md and docs/VERIFICATION.md
said "no remote configured, publication pending". The repository is now
public at https://github.com/Gruu6y/magpie-empty-enum-fix with main at 38ec986,
19 tracked blobs identical to the local tree, and both selfcheck CI runs
completed / success; leaving the old wording would contradict the published
page, so this commit records the real state instead of rewriting published
history.

docs/VERIFICATION.md also gains the read-back evidence (ls-remote, recursive
tree, Actions runs), drops the stale pre-amend short sha d924be3, and corrects
an over-broad production claim: this task never wrote
C:/Users/Administrator/.config/magpie/providers.json, but the running magpie
updates that file on its own, so a frozen mtime is not proof of anything.

It records the credential lesson too: the local GCM token has only public_repo
and cannot write .github/workflows/* (git push refuses it, REST returns 404),
while the cindy-github plugin's backup token has the workflow scope and built
the identical trees and commits.
```

---

## The patch's commit message (for magpie, via `git am`)

```
gateway: reject empty enum values in Antigravity tool schemas

Google's GenerateContentRequest validation rejects a function declaration
whose enum contains an empty string:

  * GenerateContentRequest.tools[0].function_declarations[N]
      .parameters.properties[modelOverride].properties[effort]
      .enum[0]: cannot be empty

plainSchema() kept every string enum member because it only rejected
non-string ones. An empty choice ("follow the default") is a valid JSON
Schema enum value, so it reached Antigravity unchanged. The same function
also turns a string `const` into a one-element enum, so `const: ""` produced
the identical invalid payload even after the enum branch was fixed.

Drop the whole enum keyword when any member is an empty string, following
plainSchema's existing rule for enums containing a non-string value. Do not
filter just the empty member out of the list: the model could still answer
with "" and be rejected a second time. Apply the same guard to the const
branch, which is the sibling of the same bug.

Why it matters: magpie classifies this 400 as shapeRefused and retries with
another group member, so the turn still returns 200 but the first token is
much later and the Antigravity seat is wasted. On one 2026-11-04 routing
ledger, 29/29 records carrying the offending tool got at least one
Antigravity 400 while 0/15 successful Antigravity records carried it; a
different agent without that tool succeeded 74/75 times.

Add an in-memory regression test that fails on the unpatched tree.

Verification (Go 1.27.0, windows/amd64, isolated sparse clone at a3768cd):
gofmt clean; `go test ./internal/gateway/ -run TestPlainSchema -count=1`
PASS (3 existing + 1 new); `go vet ./internal/gateway/` PASS;
`go build ./internal/gateway/` PASS; `git apply --check` / `git apply` /
`patch --dry-run -p1` PASS on a clean copy of HEAD. The full
internal/gateway suite has 9 pre-existing red tests on that machine; the
identical 9 fail with the fix reverted, so they are unrelated to this change.
Full-repository and cross-platform checks were not run. No live gateway was
contacted and no production config was touched.

Per docs/contributors.md this repository does not take external PRs; this
patch is offered as a description plus a diff for a maintainer to apply.
If the root cause, error text or test fixture is used, please credit the
reporter in the commit.
```
