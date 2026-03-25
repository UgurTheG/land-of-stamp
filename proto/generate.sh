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
echo "  → backend/gen/pb/ (connect service stubs)"

echo "=== Generating TypeScript protobuf ==="
mkdir -p "$ROOT/frontend/src/gen/proto"

# Use @bufbuild/protoc-gen-es for protobuf messages
protoc \
  --proto_path="$ROOT" \
  --experimental_allow_proto3_optional \
  --plugin="protoc-gen-es=$ROOT/frontend/node_modules/.bin/protoc-gen-es" \
  --es_out="$ROOT/frontend/src/gen" \
  --es_opt=target=ts \
  "$ROOT/proto/stempelkarte.proto"
echo "  → frontend/src/gen/proto/stempelkarte_pb.ts"

# Use @connectrpc/protoc-gen-connect-es for Connect service clients
protoc \
  --proto_path="$ROOT" \
  --experimental_allow_proto3_optional \
  --plugin="protoc-gen-connect-es=$ROOT/frontend/node_modules/.bin/protoc-gen-connect-es" \
  --connect-es_out="$ROOT/frontend/src/gen" \
  --connect-es_opt=target=ts \
  "$ROOT/proto/stempelkarte.proto"
echo "  → frontend/src/gen/proto/stempelkarte_connect.ts"

echo "✅ Proto generation complete"
