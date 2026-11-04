#!/usr/bin/env bash
# Check or apply the empty-enum patch to a magpie checkout you name.
#
# Usage:
#   ./scripts/apply.sh /path/to/magpie --check     # read-only, default
#   ./scripts/apply.sh /path/to/magpie --apply     # working tree, no commit
#   ./scripts/apply.sh /path/to/magpie --am        # commit with the full message
#
# Refuses to run without an explicit path. Never touches ~/.config/magpie,
# never restarts magpie, never contacts a running gateway.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
plain="$root/patches/empty-enum.patch"
mbox="$root/patches/0001-gateway-reject-empty-enum-values-in-antigravity-schemas.patch"

usage() { sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//' >&2; exit 2; }
fail()  { printf 'FAIL: %s\n' "$*" >&2; exit 1; }

[ $# -ge 1 ] || usage
src="$1"
mode="${2:---check}"

case "$mode" in
  --check|--apply|--am) ;;
  *) usage ;;
esac

[ -d "$src" ] || fail "not a directory: $src"
git -C "$src" rev-parse --show-toplevel >/dev/null 2>&1 || fail "not a git checkout: $src"
[ -f "$src/internal/gateway/codeassist.go" ] || fail "no internal/gateway/codeassist.go in $src"

# Refuse a checkout whose HEAD differs from the baseline: hunks may still
# apply, but the result should be reviewed rather than assumed.
head="$(git -C "$src" rev-parse HEAD)"
baseline="a3768cdd1113118998863ed36bd6dd77c8e9e2cc"
if [ "$head" != "$baseline" ]; then
  printf 'NOTE: HEAD is %s, baseline for this patch is %s.\n' "$head" "$baseline" >&2
  printf '      Review the applied hunks before building.\n' >&2
fi

case "$mode" in
  --check)
    git -C "$src" apply --check --verbose "$mbox"
    echo "OK: patch applies cleanly to $src (nothing changed)"
    ;;
  --apply)
    git -C "$src" apply --check "$mbox" || fail "patch does not apply; nothing changed"
    git -C "$src" apply "$mbox"
    echo "Applied to $src working tree (no commit created)."
    echo "Next: cd '$src' && go test ./internal/gateway/ -run TestPlainSchema -count=1"
    ;;
  --am)
    git -C "$src" apply --check "$mbox" || fail "patch does not apply; nothing changed"
    # git am needs a configured identity; surface a clear message rather than
    # leaving a half-applied mailbox.
    if ! git -C "$src" var GIT_COMMITTER_IDENT >/dev/null 2>&1; then
      fail "git committer identity is not configured in $src; set user.name/user.email first, or use --apply"
    fi
    git -C "$src" am "$mbox"
    echo "Applied as a commit in $src, with the full commit message preserved."
    echo "Next: cd '$src' && go test ./internal/gateway/ -run TestPlainSchema -count=1"
    ;;
esac
