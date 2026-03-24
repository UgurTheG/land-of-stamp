#!/usr/bin/env bash
# Generates Go and TypeScript code from proto/stempelkarte.proto
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Generating Go protobuf ==="
mkdir -p "$ROOT/backend/gen/pb"
protoc \
  --proto_path="$ROOT" \
  --experimental_allow_proto3_optional \
  --go_out="$ROOT/backend/gen/pb" \
  --go_opt=module=land-of-stamp-backend/gen/pb \
  "$ROOT/proto/stempelkarte.proto"
echo "  → backend/gen/pb/stempelkarte.pb.go"

echo "=== Generating TypeScript protobuf ==="
mkdir -p "$ROOT/frontend/src/gen/proto"
protoc \
  --proto_path="$ROOT" \
  --experimental_allow_proto3_optional \
  --plugin="$ROOT/frontend/node_modules/.bin/protoc-gen-ts_proto" \
  --ts_proto_out="$ROOT/frontend/src/gen" \
  --ts_proto_opt=esModuleInterop=true,onlyTypes=true,useOptionals=messages \
  "$ROOT/proto/stempelkarte.proto"
echo "  → frontend/src/gen/proto/stempelkarte.ts"

echo "✅ Proto generation complete"
