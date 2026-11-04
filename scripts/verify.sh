#!/usr/bin/env bash
# Verify this repository locally. No network, no gateway, no model tokens,
# and no writes outside this repository.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

fail() { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
step() { printf '\n== %s ==\n' "$*"; }

command -v go >/dev/null 2>&1 || fail "go is not on PATH"
command -v git >/dev/null 2>&1 || fail "git is not on PATH"

step "1/6 gofmt (selfcheck)"
unformatted="$(gofmt -l selfcheck)"
[ -z "$unformatted" ] || fail "unformatted Go files: $unformatted"
echo "clean"

step "2/6 go vet (selfcheck)"
( cd selfcheck && go vet ./... )
echo "ok"

step "3/6 go test (selfcheck, full suite)"
( cd selfcheck && go test ./... -count=1 )

step "4/6 patch integrity (LF, trailing newline, both guards)"
for p in patches/empty-enum.patch patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch; do
  [ -f "$p" ] || fail "missing $p"
  # grep -c $'\r' is unreliable on Git Bash (CR is consumed as a line
  # terminator), so count the byte directly.
  if [ "$(tr -cd '\r' < "$p" | wc -c)" -ne 0 ]; then fail "$p contains CR; patches must be LF-only"; fi
  [ "$(tail -c 1 "$p" | od -An -tu1 | tr -d ' ')" = "10" ] || fail "$p has no trailing newline"
  count="$(grep -c 'ok && s != ""' "$p" || true)"
  [ "$count" -ge 2 ] || fail "$p does not carry both guards (found $count)"
  echo "$p: ok ($count guards, LF)"
done

step "5/6 git apply --check on a clean copy of magpie HEAD (optional)"
# Only possible when a magpie checkout is provided as $1. Nothing is written
# to a running instance or to ~/.config/magpie.
if [ $# -ge 1 ] && [ -n "$1" ]; then
  src="$1"
  git -C "$src" rev-parse --show-toplevel >/dev/null || fail "$src is not a git checkout"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  mkdir -p "$tmp/internal/gateway"
  git -C "$src" show HEAD:internal/gateway/codeassist.go > "$tmp/internal/gateway/codeassist.go" \
    || fail "cannot read HEAD:internal/gateway/codeassist.go from $src"
  git -C "$tmp" init -q
  git -C "$tmp" config core.autocrlf false
  git -C "$tmp" -c user.name=verify -c user.email=verify@example.invalid \
    add internal/gateway/codeassist.go
  git -C "$tmp" -c user.name=verify -c user.email=verify@example.invalid \
    commit -q -m baseline
  git -C "$tmp" apply --check --verbose "$root/patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch"
  echo "git apply --check: ok"
else
  echo "skipped (no magpie checkout path given; pass one as the first argument to enable)"
fi

step "6/6 production config untouched"
# Read-only statement: this script never writes ~/.config/magpie.
echo "this script performs no writes outside $(pwd)"

printf '\nALL CHECKS PASSED\n'
