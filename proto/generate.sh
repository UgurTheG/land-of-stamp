#!/usr/bin/env bash
# Generates Go (protobuf + ConnectRPC) and TypeScript code from proto/stempelkarte.proto
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

echo "=== Generating Go protobuf + ConnectRPC ==="
mkdir -p "$ROOT/backend/gen/pb"
protoc \
  --proto_path="$ROOT" \
  --experimental_allow_proto3_optional \
  --go_out="$ROOT/backend/gen/pb" \
  --go_opt=module=land-of-stamp-backend/gen/pb \
  --connect-go_out="$ROOT/backend/gen/pb" \
  --connect-go_opt=module=land-of-stamp-backend/gen/pb \
  "$ROOT/proto/stempelkarte.proto"
echo "  → backend/gen/pb/stempelkarte.pb.go"
echo "  → backend/gen/pb/pbconnect/stempelkarte.connect.go"

echo "=== Generating TypeScript protobuf + services ==="
mkdir -p "$ROOT/frontend/src/gen/proto"

# @bufbuild/protoc-gen-es v2 generates both messages and service descriptors
protoc \
  --proto_path="$ROOT" \
  --experimental_allow_proto3_optional \
  --plugin="protoc-gen-es=$ROOT/frontend/node_modules/.bin/protoc-gen-es" \
  --es_out="$ROOT/frontend/src/gen" \
  --es_opt=target=ts \
  "$ROOT/proto/stempelkarte.proto"
echo "  → frontend/src/gen/proto/stempelkarte_pb.ts"

echo "✅ Proto generation complete"
