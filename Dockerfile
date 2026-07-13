# syntax=docker/dockerfile:1

# --- build stage ----------------------------------------------------------
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache modules first.
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
# CGO-free build (modernc.org/sqlite is pure Go) -> a static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gosplit ./cmd/gosplit

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
