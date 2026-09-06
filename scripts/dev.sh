#!/usr/bin/env bash
# GoSplit dev tasks. The only place that knows how to build/test/lint/scan this
# repo -- CI calls task names only. Keep dev.ps1 behaviourally identical.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
DIST="$REPO_ROOT/dist"
mkdir -p "$LOG_DIR"
cd "$REPO_ROOT"

export NO_COLOR=1

# Toolchain parity with CI: go.mod's `toolchain` directive is the single pin,
# and GOTOOLCHAIN makes any go binary honour it exactly. Set only when unset,
# so an exported value wins.
if [ -z "${GOTOOLCHAIN:-}" ] && [ -f "$REPO_ROOT/go.mod" ]; then
  t="$(sed -n 's/^toolchain //p' "$REPO_ROOT/go.mod")"
  [ -n "$t" ] && export GOTOOLCHAIN="$t"
fi

# --- output helpers ---------------------------------------------------------
c() { printf '\033[%sm%s\033[0m\n' "$1" "$2"; }
step() { c '1;36' "==> $*"; }
ok()   { c '1;32' "ok: $*"; }
warn() { c '1;33' "warn: $*"; }
die()  { c '1;31' "error: $*"; exit 1; }

now() { date +%Y-%m-%dT%H:%M:%S%z; }

# Truncate this task's log with a header, then everything tees onto it.
log_begin() {
  printf '=== %s | %s ===\n' "$(now)" "$1" > "$LOG_DIR/$1.log"
}

# finish <task> <exit-code> <elapsed-seconds>
finish() {
  local task=$1 code=$2 secs=$3 status
  if [ "$code" -eq 0 ]; then status=OK; else status="FAILED (exit $code)"; fi
  printf '%s | %s | %ss | %s\n' "$(now)" "$task" "$secs" "$status" \
    | tee -a "$LOG_DIR/$task.log"
}

# Strip ANSI/CSI so logs stay readable plain text.
strip_csi() { sed -E $'s/\x1b\\[[0-9;?]*[a-zA-Z]//g'; }

# run <task> <cmd...> -- tees combined output, returns the command's code.
run() {
  local task=$1; shift
  "$@" 2>&1 | strip_csi | tee -a "$LOG_DIR/$task.log"
  return "${PIPESTATUS[0]}"
}

# --- target resolution ------------------------------------------------------
# CI sets TARGET_OS/TARGET_ARCH; unset means host. Translated to GOOS/GOARCH
# here and nowhere else.
host_os() {
  case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
    mingw*|msys*|cygwin*) echo windows ;;
    darwin) echo darwin ;;
    *) echo linux ;;
  esac
}
host_arch() { case "$(uname -m)" in x86_64|amd64) echo amd64;; aarch64|arm64) echo arm64;; *) uname -m;; esac; }
T_OS="${TARGET_OS:-$(host_os)}"
T_ARCH="${TARGET_ARCH:-$(host_arch)}"
BIN_NAME="gosplit-$T_OS-$T_ARCH"
[ "$T_OS" = "windows" ] && BIN_NAME="$BIN_NAME.exe"

# --- version ----------------------------------------------------------------
# The git tag is the single source of version truth. tag.yml passes VERSION to
# the image build from the tag ref, and this script passes it from `git
# describe`, so neither depends on `git describe` running inside the build
# container -- where .dockerignore's exclusions would make it report -dirty.
# Images take the tag with the leading v stripped (registry convention) -- the
# only place the v drops.
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
IMAGE_TAG="${VERSION#v}"
# Local image name/tag (override with IMAGE_NAME). The published multi-arch
# image is built by tag.yml, not here.
IMAGE="${IMAGE_NAME:-gosplit:$IMAGE_TAG}"
# Trivy runs as a container (no local binary needed); override the tag.
TRIVY_IMAGE="${TRIVY_IMAGE:-aquasec/trivy:latest}"

# --- tasks ------------------------------------------------------------------
task_build() {
  mkdir -p "$DIST"
  # CGO-free (modernc.org/sqlite is pure Go) -> a static binary on every target.
  # -X main.version stamps the tag in; without it the binary reports "dev".
  run build env CGO_ENABLED=0 GOOS="$T_OS" GOARCH="$T_ARCH" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
      -o "$DIST/$BIN_NAME" ./cmd/gosplit
}

task_vet() { run vet go vet ./...; }

task_test() { run test go test ./... -count=1; }

task_cov() {
  local prof="$LOG_DIR/coverage.out" html="$LOG_DIR/coverage.html"
  # -p 1 serializes package test binaries: on Windows, parallel runs race to
  # exec the shared covdata.exe and hit "file in use" locks (often Defender).
  run cov go test -covermode=atomic -coverprofile="$prof" -p 1 -count=1 ./... || return $?
  run cov go tool cover -html="$prof" -o "$html" || return $?
  # The printed total is the floor for the next run; logs/cov.log keeps it.
  go tool cover -func="$prof" | tail -1 | tee -a "$LOG_DIR/cov.log"
  ok "cov -> $html"
}

# GitHub's windows runners cannot run linux containers, and a machine without
# Docker has nothing to build against. Both are "does not apply here", not a
# misconfiguration -- the ubuntu leg covers the image tasks.
docker_linux() {
  command -v docker >/dev/null 2>&1 || return 1
  [ "$(docker version -f '{{.Server.Os}}' 2>/dev/null)" = "linux" ]
}

# build_image <log-task> -- the actual build, logged under the calling task so
# a standalone `scan` never appends to a stale image.log.
build_image() {
  run "$1" docker build --progress=plain \
    --build-arg "VERSION=$VERSION" -t "$IMAGE" "$REPO_ROOT"
}

task_image() {
  docker_linux || { warn "no docker daemon running linux containers; skipping image"; return 0; }
  build_image image
}

# One task, every applicable check. FATAL on fixable CVEs.
task_scan() {
  local code=0
  # go tool, not `go run pkg@version`: the latter runs module-less and ignores
  # go.mod's toolchain pin, so it builds against the minimum Go its own module
  # accepts and then refuses to load this module's packages.
  run scan go tool govulncheck ./... || code=$?
  [ "$code" -ne 0 ] && return "$code"

  # Image half: never against a stale image -- build first, because `image` is
  # not in `all`.
  docker_linux || { warn "no docker daemon running linux containers; skipping image scan"; return 0; }
  build_image scan || return $?
  # Trivy runs in a container, so it needs the Docker socket to read the image
  # just built -- without it it finds nothing locally and tries to PULL the
  # image from Docker Hub, which 401s. The named volume caches the vulnerability
  # DB between runs (the DB itself still refreshes; only the re-fetch is saved).
  # --pull=always matters more than the tag: without it docker reuses whatever
  # :latest resolved to months ago -- unpinned AND stale.
  # --scanners vuln keeps this gate about CVEs. Trivy also ships secret and
  # misconfiguration detectors, and with --exit-code 1 those would hard-fail
  # scan (and CI, and the release) on advisory findings this task never claimed
  # to cover.
  run scan docker run --rm --pull=always \
    -e NO_COLOR=1 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v trivy-cache:/root/.cache/ \
    "$TRIVY_IMAGE" image \
    --quiet --scanners vuln --exit-code 1 --severity HIGH,CRITICAL --ignore-unfixed "$IMAGE"
}

task_up()   { run up   docker compose up -d; }
task_down() { run down docker compose down; }

# Local only: the graph is a developer artifact, not a CI output.
task_graphify() {
  [ -n "${CI:-}" ] && { warn "graphify is local-only; skipping in CI"; return 0; }
  command -v graphify >/dev/null || { warn "graphify not on PATH; skipping"; return 0; }
  run graphify graphify update .
}

# --- dispatch ---------------------------------------------------------------
ALL="build vet test"
FULL="build vet test cov image scan graphify"

usage() {
  cat <<EOF
usage: $(basename "$0") <task>...

  build vet test cov scan image up down graphify
  all   = $ALL            (what CI runs, as: all scan)
  full  = $FULL           (pre-tag sweep)

  version $VERSION -> image $IMAGE
  override: VERSION, IMAGE_NAME, TRIVY_IMAGE, TARGET_OS, TARGET_ARCH
EOF
}

expand() {
  case "$1" in
    all)  echo "$ALL" ;;
    full) echo "$FULL" ;;
    *)    echo "$1" ;;
  esac
}

[ $# -eq 0 ] && { usage; exit 0; }
case "${1:-}" in -h|--help|help) usage; exit 0 ;; esac

TASKS=""
for a in "$@"; do TASKS="$TASKS $(expand "$a")"; done

FAILED=0
for task in $TASKS; do
  type "task_$task" >/dev/null 2>&1 || die "unknown task: $task"
  step "$task"
  log_begin "$task"
  start=$SECONDS
  code=0
  "task_$task" || code=$?
  finish "$task" "$code" "$((SECONDS - start))"
  if [ "$code" -ne 0 ]; then
    FAILED=1
    warn "$task failed; stopping"
    break   # build/vet/test/scan are all fatal
  fi
  ok "$task"
done
exit "$FAILED"
