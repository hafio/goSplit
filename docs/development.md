# Development

Building, testing, releasing and extending GoSplit. For contributors.

## Prerequisites

- Go 1.27. `go.mod` pins `toolchain go1.27.0` and the dev scripts export `GOTOOLCHAIN`
  from it when it is not already set, so any Go 1.21+ on your PATH fetches and uses
  exactly that toolchain. An exported `GOTOOLCHAIN` (for example `local`) wins; unset it
  to get CI's toolchain.
- Docker, for the `image`, `scan`, `up` and `down` tasks. `image` and `scan` warn and skip
  their image work when no daemon running Linux containers is present; `up` and `down`
  simply fail without Docker. Everything else works.
- Optionally graphify, for the knowledge graph in `graphify-out/`; see
  [Knowledge graph](#knowledge-graph).

## Running locally

```sh
cp .env.example .env        # SQLite, links printed to the log; set SESSION_SECRET
set -a; . ./.env; set +a    # load it into the shell (the binary reads no .env file)
go run ./cmd/gosplit
```

The database and uploads land in `./data/`. Register at `http://localhost:8080`;
registering signs you in, and any magic-link or reset mail is printed to the log. Put your address in `ADMIN_EMAILS` first if you want the admin
console.

## Dev scripts

`scripts/dev.sh` (bash) and `scripts/dev.ps1` (PowerShell) are the only place that knows
how to build, test, lint or scan this repository. CI calls task names, never commands, so
what passes locally is what passes in CI.

```sh
./scripts/dev.sh build     # dist/gosplit-<os>-<arch>[.exe], version stamped in
./scripts/dev.sh vet       # go vet ./...
./scripts/dev.sh test      # go test ./... -count=1
./scripts/dev.sh cov       # coverage profile -> scripts/logs/coverage.html, prints the total
./scripts/dev.sh image     # docker build of the distroless image (local, single-arch)
./scripts/dev.sh scan      # govulncheck, then Trivy against a freshly built image
./scripts/dev.sh up        # docker compose up -d      (down to stop)
./scripts/dev.sh graphify  # graphify update .          (local only, skipped in CI)
```

Two aggregates, and only two:

| Aggregate | Tasks | Use |
| --- | --- | --- |
| `all` | `build vet test` | the inner loop; CI runs `all scan` |
| `full` | `build vet test cov image scan graphify` | the pre-tag sweep |

Each task truncates `scripts/logs/<task>.log`, tees its output there, and ends with a
footer line `<ISO-8601> | <task> | <secs>s | OK` or `... | FAILED (exit <code>)`. Any
failure stops the queue and the script exits 1. Three checks can bow out cleanly instead,
warning and passing: `image` and the image half of `scan` without a Linux Docker daemon,
and `graphify` when `CI` is set or the binary is not on `PATH`.

Overrides: `VERSION`, `IMAGE_NAME`, `TRIVY_IMAGE`, and `TARGET_OS` / `TARGET_ARCH` for
cross-compilation. The PowerShell script assumes a Windows host when `TARGET_OS` is
unset; the bash script derives the host from `uname`.

### What `scan` checks

- `go tool govulncheck ./...` (a pinned `tool` directive in `go.mod`, so it runs under the
  same toolchain). Any finding fails.
- Trivy, run as a container against the image just built, with `--severity HIGH,CRITICAL
  --ignore-unfixed`. A fixable high or critical CVE fails; lower severities and
  unfixable findings are not reported. The release pipeline runs the same Trivy gate a
  second time against the pushed multi-arch digest.

When `scan` reports a fixable CVE, bump the dependency as its own change and re-run the
gates.

## Testing

```sh
./scripts/dev.sh test
./scripts/dev.sh cov
```

- Tests are standard-library `testing`, table-driven, co-located with the code. Shared
  fixtures live next to the packages that use them (`openTestStore` in
  `internal/store`, `newHarness` in `internal/httpapp`, `newTestService` in
  `internal/service`).
- 100% coverage is the target on the split engine, balance view, debt simplification
  and money math. The Golden Scenario in `internal/store` is a committed regression
  fixture for balances.
- **The coverage floor is the last total you recorded.** `cov` prints the total and writes
  it at the end of `scripts/logs/cov.log`. That file is gitignored and rewritten on every
  run, and CI never produces one (`all scan` does not include `cov`), so note the number
  before you re-run. A drop needs either more tests or a stated reason in the change.
- Every test runs against SQLite. There is no live-PostgreSQL harness, by choice. The
  engine-specific pieces that are pure functions (boolean binding per engine, `?` to `$n`
  rebinding) are unit-tested by passing the PostgreSQL engine value in. The
  PostgreSQL-only statements inside the restore transaction (`LOCK TABLE`, sequence
  resets) are exercised only by the manual cross-engine restore done before a release.
- Cover the lines you touch in the same change. A new branch gets a test; a removed
  branch loses or repurposes one. Security and error boundaries always get both the
  rejection case and the "downstream call did not happen" assertion.

## Knowledge graph

`graphify-out/` holds a code graph used for navigation and impact analysis. Query it before
grepping for cross-file questions:

```sh
graphify query "how does the fragment registry reach handlers"
graphify path "Server.render" "RenderFragment"
graphify explain "balance_view"
```

Refresh it after changing code with `./scripts/dev.sh graphify` (or `graphify update .`).

## Extending

### Adding a locale

Copy `internal/i18n/locales/en.json` to `<lang>.json`, translate the values, keep the
keys. The catalogue is embedded and discovered by listing the directory, so no code
changes and no new test: `TestNoOrphanTranslationKeys` in `internal/i18n` walks every
catalogue and fails on a key that `en.json` does not have. A key missing from a
translation is tolerated and falls back to English.

### Adding a page

1. Template in `internal/web/templates/<name>.html` defining `content`.
2. Register it in the `pageFiles` map in `internal/web/view.go`; an unregistered page is
   never parsed.
3. Handler in `internal/httpapp/`, route in `server.go`, behind `RequireUser` (and
   `RequireAdmin`) as appropriate.
4. If the page should refresh itself, add a `content` entry to the `fragments` registry in
   `view.go`. If it is a form, do not: form pages are never polled.
5. Translations for every new key in all nine locales.

### Adding a fragment region

Give the region a stable DOM id that exists in **every** state of the page (empty and
populated), wrap it in a `{{define}}` block, and add `page -> id -> block` to the
`fragments` registry. The renderer validates the entry at startup. Point the region's
forms at it with `hx-target="#<id>" hx-swap="morph:outerHTML"`.

### Adding a configuration setting

Add the field and its `getEnv*` line in `internal/config/config.go`, validate it in
`Load` if a bad value should stop the boot, document it in
[configuration.md](configuration.md) and `.env.example` in the same change.

### Adding a table

Write the migration for both dialects under `internal/store/migrations/`, then add the
table and its column kinds to the registry in `internal/backup/schema.go`. A test
compares the registry against the live schema and fails if they drift, so a table missing
from backups cannot ship unnoticed.

## Versioning and release

The git tag is the single source of version truth: no VERSION file, no hard-coded
constant. `vMAJOR.MINOR.PATCH` reaches the binary through `-X main.version`, and the
image is tagged with the `v` stripped.

```sh
gosplit version            # also: gosplit -version, gosplit --version
```

A development build reports `git describe --tags --always --dirty`. A `docker build` run
by hand also reports `-dirty`, because `.dockerignore` drops tracked directories from the
context; pass `--build-arg VERSION=...` if that matters.

### GitHub Actions

Nothing runs on an ordinary push. Two workflows:

- **`ci.yml`** runs `all scan` on `ubuntu-24.04` and `windows-2025` and uploads
  `scripts/logs/` as an artifact (`logs-<os>`, kept 30 days). It has no push trigger. It
  runs when called by `tag.yml`, manually from the Actions tab, or on a pull request that
  touches the workflows, Dependabot config or dev scripts, which is what gates
  Dependabot's weekly, grouped action bumps.
- **`tag.yml`** is the only automatic pipeline. Push a `v*` tag and it runs: plan (reads
  the repository variables `SHIP_IMAGE` and `BUILD_TARGETS` and fails fast if they are
  malformed or nothing would ship) -> gates (`ci.yml`) -> multi-arch image (`linux/amd64`, `linux/arm64`) pushed to
  `ghcr.io/hafio/gosplit` with tags `X.Y.Z`, `X.Y`, `X` and `latest`, provenance and SBOM
  attached -> a second Trivy gate on the pushed digest -> a GitHub Release created last,
  with generated notes and the image digest. A failure anywhere leaves no release. The
  release step is idempotent, so re-running a tag does not fail on an existing release.

Binaries are attached to a release only when the repository variable `BUILD_TARGETS` is
set to a JSON array of `{os, arch}`. It is unset for this repository, so releases ship the
image only.

### Cutting a release

1. On a clean checkout: `./scripts/dev.sh full`. This is the only check that catches a
   file you never committed.
2. Compare the `cov` total it printed against the number you noted from your previous
   run (`scripts/logs/cov.log` holds only the current run).
3. Tag and push: `git tag v1.2.3 && git push origin v1.2.3`.
4. Watch the Actions run; on failure, read the `logs-<os>` artifact, fix, and tag again.

## Conventions

- Plain ASCII in code, comments and docs.
- Handlers thin, services own the rules, SQL lives in `store`. Pass dependencies
  explicitly; no globals.
- Validate at the boundary, encode anything that lands in a structured format, and make
  errors say what failed, why, and what to do next.
- Docs update in the same change as behaviour. The reference for settings is
  [configuration.md](configuration.md); the reference for features is
  [features.md](features.md).

## See also

- [architecture.md](architecture.md) -- how the pieces fit
- [configuration.md](configuration.md) -- settings reference
- [deployment.md](deployment.md) -- what operators see of a release
