#!/bin/sh
set -eu

BINARY_NAME="${BINARY_NAME:-mautrix-ghdiscussions}"
OUTPUT="${OUTPUT:-${BINARY_NAME}}"

TAG="${TAG:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}"
BUILDTIME="${BUILDTIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

LDFLAGS="-s -w \
  -X main.Tag=${TAG} \
  -X main.Commit=${COMMIT} \
  -X main.BuildTime=${BUILDTIME}"

echo "Building ${OUTPUT} (${TAG} @ ${COMMIT})"
go build -ldflags "${LDFLAGS}" -o "${OUTPUT}" ./cmd/mautrix-ghdiscussions/
