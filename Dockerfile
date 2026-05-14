# Build stage: Use Alpine Linux for building
FROM golang:1.25-alpine AS builder

# Install build dependencies (git for version detection, ca-certificates for HTTPS)
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files first (cache layer)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for version info and target platform
ARG VERSION=dev
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH

# Build the binary
# CGO_ENABLED=0: static binary (modernc.org/sqlite is pure Go, no CGo needed)
# -ldflags="-s -w": strip debug info for smaller binary
# -trimpath: reproducible builds
RUN \
  TARGETOS=${TARGETOS:-linux} \
  TARGETARCH=${TARGETARCH:-amd64} \
  CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -trimpath \
    -o niyantra ./cmd/niyantra

# Verify the binary works (only if native build)
RUN ./niyantra version || echo "Cross-compiled binary, skipping version check"

# Create data directory owned by nonroot user (UID 65532)
RUN mkdir -p /data && chown 65532:65532 /data

# ─────────────────────────────────────────────────────────────────────────────
# Shell variant: Alpine-based image with /bin/sh for docker exec access
# Build with: docker build --target runtime-shell -t niyantra:shell .
# ─────────────────────────────────────────────────────────────────────────────
FROM alpine:3.21 AS runtime-shell

RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -g 65532 -S nonroot && \
    adduser -u 65532 -S -G nonroot -h /app nonroot

ARG VERSION=dev
LABEL maintainer="bhaskarjha-com"
LABEL description="Niyantra — Local-first AI operations dashboard (shell variant)"
LABEL version="${VERSION:-dev}"

WORKDIR /app
COPY --from=builder /build/niyantra /app/niyantra
COPY --from=builder --chown=65532:65532 /data /data

EXPOSE 9222

ENV NIYANTRA_DB=/data/niyantra.db \
    NIYANTRA_PORT=9222 \
    NIYANTRA_BIND=0.0.0.0

USER nonroot
ENTRYPOINT ["/app/niyantra"]
CMD ["serve"]

# ─────────────────────────────────────────────────────────────────────────────
# Default runtime: Distroless for minimal, secure production image (~15 MB)
# This is the last stage so "docker build ." (no --target) builds this
# ─────────────────────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

ARG VERSION=dev
LABEL maintainer="bhaskarjha-com"
LABEL description="Niyantra — Local-first AI operations dashboard for Antigravity, Codex, Claude, Cursor, Gemini CLI"
LABEL version="${VERSION:-dev}"

WORKDIR /app
COPY --from=builder /build/niyantra /app/niyantra
COPY --from=builder --chown=65532:65532 /data /data

EXPOSE 9222

ENV NIYANTRA_DB=/data/niyantra.db \
    NIYANTRA_PORT=9222 \
    NIYANTRA_BIND=0.0.0.0

# distroless has no shell — use exec form
ENTRYPOINT ["/app/niyantra"]
CMD ["serve"]
