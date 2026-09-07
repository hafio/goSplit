# syntax=docker/dockerfile:1

# --- build stage ----------------------------------------------------------
# --platform=$BUILDPLATFORM pins the compiler to the native architecture and
# cross-compiles via GOARCH, so the linux/arm64 leg of a multi-arch build is
# not compiled wholly under QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src

# Both tag.yml and `dev.sh image` pass VERSION directly, so the `git describe`
# fallback below is only for a hand-run `docker build`. It must stay, but it
# cannot be relied on: .dockerignore drops tracked directories to keep the
# context small, so git inside the build container sees them as deleted and
# appends -dirty. Passing VERSION is what keeps a released image's tag and its
# binary in agreement.
ARG VERSION=""
ARG TARGETARCH
RUN apk add --no-cache git

# Both Go caches live in BuildKit cache mounts rather than in image layers.
# modernc.org/sqlite is SQLite transpiled to Go (~235MB of source, plus libc),
# so baking the module cache into one layer and the ~1.5GB of build-cache
# artifacts into the next meant every source edit snapshotted both again --
# 500-odd cache entries and tens of GB of reclaimable cache. The mounts keep
# them out of the layers, so BuildKit manages one reusable cache it can
# garbage-collect as a unit, and a warm cache skips recompiling SQLite, which
# dominates build time.
#
# GOMODCACHE and GOCACHE are set explicitly so the mount targets do not depend
# on the base image's GOPATH/HOME defaults. sharing=locked serializes the two
# legs of a multi-arch build through the module download; the build cache is
# keyed per TARGETARCH instead, since object code is not portable between them.
ENV GOMODCACHE=/gomodcache
ENV GOCACHE=/gocache

# Resolve modules first, so a dependency problem fails before the source copy.
COPY go.mod go.sum* ./
RUN --mount=type=cache,target=/gomodcache,sharing=locked \
    go mod download

COPY . .
# CGO-free build (modernc.org/sqlite is pure Go) -> a static binary.
# -X main.version stamps the tag in; an unstamped binary reports "dev", which
# the release check treats as a failure.
# Separate statements, not an && chain: a failing `git describe` makes the
# assignment itself non-zero, which would abort with a bare exit code instead of
# the message below. An undeterminable version is fatal -- shipping a release
# image whose binary disagrees with its tag is worse than failing here.
RUN --mount=type=cache,target=/gomodcache,sharing=locked \
    --mount=type=cache,target=/gocache,id=gocache-$TARGETARCH \
    git config --global --add safe.directory /src; \
    VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null)}"; \
    if [ -z "$VERSION" ]; then \
      echo "cannot determine version: pass --build-arg VERSION=<tag>, or build with .git in the context" >&2; \
      exit 1; \
    fi; \
    echo "building gosplit $VERSION for linux/${TARGETARCH}"; \
    CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" \
      go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/gosplit ./cmd/gosplit

# An empty /data to copy into the runtime stage with ownership -- see below.
RUN mkdir -p /empty-data

# --- runtime stage --------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/gosplit /app/gosplit

# Ship /data owned by the nonroot UID this image runs as. Docker initializes a
# fresh named volume from the image's directory at that path, ownership
# included, so without this the volume comes up root-owned and the app cannot
# create the SQLite file on a first-ever `docker compose up`. Distroless has no
# shell, so a COPY --chown from the build stage is the only way to set it.
#
# The UID is numeric on purpose: --chown resolves names against the BUILD
# stage's /etc/passwd, where `nonroot` does not exist.
COPY --from=build --chown=65532:65532 /empty-data /data

# Data volume holds the SQLite file + uploads.
VOLUME ["/data"]
ENV DATABASE_URL=file:/data/gosplit.db
ENV UPLOAD_DIR=/data/uploads
ENV ADDR=:8080
EXPOSE 8080

# Distroless has no shell; run the binary directly.
ENTRYPOINT ["/app/gosplit"]
