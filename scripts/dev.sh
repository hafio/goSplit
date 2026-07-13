#!/usr/bin/env bash
# GoSplit task runner (bash). Mirror of dev.ps1. Run from anywhere.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
cd "$ROOT"

# Image name/tag (override with IMAGE_NAME env var).
IMAGE="${IMAGE_NAME:-gosplit:dev}"
# Trivy runs as a container image (no local binary needed); override the tag.
TRIVY_IMAGE="${TRIVY_IMAGE:-aquasec/trivy:latest}"
# Python for the graphify update (override with PYTHON env var).
PYTHON="${PYTHON:-python}"; command -v "$PYTHON" >/dev/null 2>&1 || PYTHON=python3

c_reset='\033[0m'; c_step='\033[36m'; c_ok='\033[32m'; c_warn='\033[33m'; c_die='\033[31m'
step() { echo -e "${c_step}==> $*${c_reset}"; }
ok()   { echo -e "${c_ok}ok: $*${c_reset}"; }
warn() { echo -e "${c_warn}warn: $*${c_reset}"; }
die()  { echo -e "${c_die}FAIL: $*${c_reset}"; exit 1; }

run() { # run <logname> <cmd...>
  local name="$1"; shift
  local log="$LOG_DIR/$name.log"
  echo "== $name : $* ==" > "$log"
  "$@" 2>&1 | tee -a "$log"
  return "${PIPESTATUS[0]}"
}

task_build() { step "build"; run build go build ./... && ok build || die build; }
task_vet()   { step "vet";   run vet go vet ./...       && ok vet   || die vet; }
task_test()  { step "test";  run test go test ./...      && ok test  || die test; }
task_cov()   {
  step "cov"
  run cov go test -covermode=atomic -coverprofile="$SCRIPT_DIR/coverage.out" -p 1 -count=1 ./... || die cov
  go tool cover -html="$SCRIPT_DIR/coverage.out" -o "$SCRIPT_DIR/coverage.html"
  go tool cover -func="$SCRIPT_DIR/coverage.out" | tail -1
  ok "cov -> $SCRIPT_DIR/coverage.html"
}
task_vuln()  { step "vuln"; run vuln govulncheck ./... || warn "vuln (report-only)"; }
# Build the distroless container image from the repo Dockerfile.
task_image() { step "image ($IMAGE)"; run image docker build -t "$IMAGE" "$ROOT" && ok image || die image; }
# Scan the built image for CVEs (report-only) using the Trivy container image —
# mounts the Docker socket to read the local image and a named volume to cache
# the vulnerability DB between runs. Requires the image to exist.
task_trivy() {
  step "trivy ($IMAGE)"
  run trivy docker run --rm \
    -e NO_COLOR=1 \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v trivy-cache:/root/.cache/ \
    "$TRIVY_IMAGE" image --quiet --scanners vuln --exit-code 0 --ignore-unfixed "$IMAGE" \
    || warn "trivy (report-only)"
}

# Incrementally update the graphify knowledge graph (AST-only, no API cost).
# Report-only: a missing python/graphify or absent graph warns, never aborts.
task_graphify() { step "graphify"; NO_COLOR=1 run graphify "$PYTHON" -m graphify update . && ok graphify || warn "graphify (report-only)"; }

task_all()   { task_build; task_vet; task_test; task_cov; task_graphify; }
task_scan()  { task_vuln; task_image; task_trivy; }
task_full()  { task_all; task_scan; }

usage() { cat <<EOF
Usage: ./scripts/dev.sh <task> [task...]
  build  vet  test  cov  graphify  vuln  image  trivy  scan  all  full
EOF
}

[ $# -eq 0 ] && { usage; exit 0; }
for t in "$@"; do
  case "$t" in
    build) task_build ;;
    vet)   task_vet ;;
    test)  task_test ;;
    cov)   task_cov ;;
    graphify) task_graphify ;;
    vuln)  task_vuln ;;
    image) task_image ;;
    trivy) task_trivy ;;
    scan)  task_scan ;;
    all)   task_all ;;
    full)  task_full ;;
    -h|--help|help) usage ;;
    *) die "unknown task: $t" ;;
  esac
done
