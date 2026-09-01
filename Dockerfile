# syntax=docker/dockerfile:1

# --- build stage ----------------------------------------------------------
# --platform=$BUILDPLATFORM pins the compiler to the native architecture and
# cross-compiles via GOARCH, so the linux/arm64 leg of a multi-arch build is
# not compiled wholly under QEMU emulation.
FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src

# tag.yml passes no --build-arg (it is copied verbatim into every repo), so the
# version is derived from the .git directory the fetch-depth:0 checkout leaves
# in the build context. `dev.sh image` passes VERSION directly and skips that.
ARG VERSION=""
ARG TARGETARCH
RUN apk add --no-cache git

# Cache modules first.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# CGO-free build (modernc.org/sqlite is pure Go) -> a static binary.
# -X main.version stamps the tag in; an unstamped binary reports "dev", which
# the release check treats as a failure.
# Separate statements, not an && chain: a failing `git describe` makes the
# assignment itself non-zero, which would abort with a bare exit code instead of
# the message below. An undeterminable version is fatal -- shipping a release
# image whose binary disagrees with its tag is worse than failing here.
RUN git config --global --add safe.directory /src; \
    VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null)}"; \
    if [ -z "$VERSION" ]; then \
      echo "cannot determine version: pass --build-arg VERSION=<tag>, or build with .git in the context" >&2; \
      exit 1; \
    fi; \
    echo "building gosplit $VERSION for linux/${TARGETARCH}"; \
    CGO_ENABLED=0 GOOS=linux GOARCH="${TARGETARCH}" \
      go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o /out/gosplit ./cmd/gosplit

# --- runtime stage --------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/gosplit /app/gosplit

# Data volume holds the SQLite file + uploads.
VOLUME ["/data"]
ENV DATABASE_URL=file:/data/gosplit.db
ENV UPLOAD_DIR=/data/uploads
ENV ADDR=:8080
EXPOSE 8080

# Distroless has no shell; run the binary directly.
ENTRYPOINT ["/app/gosplit"]
