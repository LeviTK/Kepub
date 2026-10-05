#!/usr/bin/env bash
# Add tests to a disposable, exact upstream checkout; never alter its implementation.
set -euo pipefail
here=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
mode=${1:-core}
if [[ $# -gt 1 || ! "$mode" =~ ^(core|native|native-compile)$ ]]; then
  printf 'Usage: %s [core|native|native-compile]\n' "$0" >&2
  exit 2
fi
commit=d51d2e28dff5b351bc67cf2280b5eef00f1267e8
scratch=$(mktemp -d "${TMPDIR:-/tmp}/kepub-mygo-boundary.XXXXXXXX")
trap 'rm -rf -- "$scratch"' EXIT
export GOTOOLCHAIN=go1.27.1
# Avoid inherited development URLs / generate mode changing trust or lifecycle.
unset MYGO_DEV_URL MYGO_ENV MYGO_READY_SOCKET MYGO_GENERATE MYGO_TEST_SECOND_INSTANCE MYGO_TEST_BEFORE_RUN
unset MYGO_E2E_BEFORE_RUN MYGO_E2E_QUIT_DURING_DIALOG
git clone --quiet --depth 1 --branch v0.2.0 https://github.com/egoist/mygo.git "$scratch/mygo"
actual=$(git -C "$scratch/mygo" rev-parse HEAD)
if [[ "$actual" != "$commit" ]]; then
  printf 'Wrong v0.2.0 commit: %s\n' "$actual" >&2
  exit 1
fi
printf 'Upstream v0.2.0 %s\n' "$actual"
go version
# The additive test layer reuses upstream TestMain + fake backend + helpers.
# No copied/reimplemented trust, secret, routing or cancellation policy.
cp "$here/boundary_test.go.txt" "$scratch/mygo/kepub_boundary_test.go"
cp "$here/native_boundary_test.go.txt" "$scratch/mygo/internal/e2e/kepub_boundary_test.go"
cd "$scratch/mygo"
production='-X github.com/egoist/mygo.production=1'
case "$mode" in
  core)
    printf '\n=== Development normal: upstream core + Kepub boundary ===\n'
    CGO_ENABLED=0 go test -count=1 -v .
    printf '\n=== Development race: upstream core + Kepub boundary ===\n'
    # Go's race instrumentation needs cgo; MyGo source remains cgo-free.
    CGO_ENABLED=1 go test -race -count=1 -v .
    printf '\n=== Production normal: Kepub boundary ONLY ===\n'
    CGO_ENABLED=0 go test -ldflags "$production" -run '^TestKepub' -count=1 -v .
    printf '\n=== Production race: Kepub boundary ONLY ===\n'
    CGO_ENABLED=1 go test -race -ldflags "$production" -run '^TestKepub' -count=1 -v .
    printf '\nPASS: real MyGo core + upstream fake backend only; NOT Kepub GUI acceptance.\n'
    ;;
  native)
    printf '\n=== Production native: Kepub frame/CSP boundary ONLY ===\n'
    MYGO_E2E=1 CGO_ENABLED=0 go test -ldflags "$production" -run '^TestKepub' -count=1 -v ./internal/e2e
    printf '\nPASS: native boundary on this host only; NOT full Kepub GUI acceptance.\n'
    ;;
  native-compile)
    printf '\n=== Compile production native coverage; do NOT run ===\n'
    CGO_ENABLED=0 go test -ldflags "$production" -c -o "$scratch/native.test" ./internal/e2e
    printf '\nPASS: native coverage compiles; no native execution or frame isolation claim.\n'
    ;;
esac
git diff --exit-code
