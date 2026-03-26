#!/usr/bin/env bash
# fix.sh — auto-fix everything a linter or formatter would complain about.
#
# What it does:
#   Backend (Go)
#     1. go mod tidy          — remove unused deps, add missing ones
#     2. gofmt -w             — canonical formatting
#     3. goimports -w         — organise + add/remove imports
#     4. go vet               — catch suspicious constructs
#     5. staticcheck          — extra static analysis (if installed)
#     6. golangci-lint run --fix  — auto-fixable linter rules (if installed)
#
#   Frontend (TypeScript / React)
#     7. npm install          — ensure node_modules are in sync with package.json
#     8. eslint --fix         — auto-fixable ESLint rules
#     9. tsc --noEmit         — type-check (no fixes, but surfaces type errors)
#
# Usage:
#   ./scripts/fix.sh            # run everything
#   ./scripts/fix.sh --go       # Go only
#   ./scripts/fix.sh --frontend # frontend only
#
# Exit code: 0 if everything passes, 1 if any step fails.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BACKEND="$ROOT/backend"
FRONTEND="$ROOT/frontend"

# ── Argument parsing ───────────────────────────────────
RUN_GO=true
RUN_FRONTEND=true

for arg in "$@"; do
  case "$arg" in
    --go)       RUN_FRONTEND=false ;;
    --frontend) RUN_GO=false ;;
  esac
done

ERRORS=0

ok()   { echo "  ✅  $*"; }
fail() { echo "  ❌  $*"; ERRORS=$((ERRORS + 1)); }
step() { echo; echo "── $* ──"; }

# ══════════════════════════════════════════════════════
# GO
# ══════════════════════════════════════════════════════
if $RUN_GO; then
  echo
  echo "╔══════════════════════════════╗"
  echo "║         Go backend           ║"
  echo "╚══════════════════════════════╝"
  cd "$BACKEND"

  step "go mod tidy"
  if go mod tidy 2>&1; then ok "go mod tidy"; else fail "go mod tidy"; fi

  step "gofmt"
  # -l lists files that would change; -w rewrites them
  UNFORMATTED=$(gofmt -l . 2>&1)
  gofmt -w . 2>&1
  if [ -z "$UNFORMATTED" ]; then
    ok "gofmt (already clean)"
  else
    ok "gofmt — reformatted:$(echo "$UNFORMATTED" | sed 's/^/\n    /')"
  fi

  step "goimports"
  if command -v goimports &>/dev/null; then
    goimports -w -local land-of-stamp-backend . 2>&1
    ok "goimports"
  else
    echo "  ⚠️  goimports not found — skipping (install: go install golang.org/x/tools/cmd/goimports@latest)"
  fi

  step "go vet"
  if go vet ./... 2>&1; then ok "go vet"; else fail "go vet"; fi

  step "staticcheck"
  if command -v staticcheck &>/dev/null; then
    SC_OUT=$(staticcheck ./... 2>&1 || true)
    # Suppress version-mismatch warnings — they come from the Go toolchain
    # being newer than the staticcheck binary, not from our code.
    REAL_ISSUES=$(echo "$SC_OUT" | grep -v "module requires at least go" | grep -v "application built with go" || true)
    if [ -n "$REAL_ISSUES" ]; then
      echo "$REAL_ISSUES"
      fail "staticcheck"
    else
      [ -n "$SC_OUT" ] && echo "  ⚠️  staticcheck: Go version mismatch (upgrade staticcheck to silence this)"
      ok "staticcheck"
    fi
  else
    echo "  ⚠️  staticcheck not found — skipping (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"
  fi

  step "golangci-lint"
  if command -v golangci-lint &>/dev/null; then
    if golangci-lint run --fix ./... 2>&1; then ok "golangci-lint"; else fail "golangci-lint (see above)"; fi
  else
    echo "  ⚠️  golangci-lint not found — skipping (install: https://golangci-lint.run/usage/install)"
  fi
fi

# ══════════════════════════════════════════════════════
# FRONTEND
# ══════════════════════════════════════════════════════
if $RUN_FRONTEND; then
  echo
  echo "╔══════════════════════════════╗"
  echo "║      Frontend (TS/React)     ║"
  echo "╚══════════════════════════════╝"
  cd "$FRONTEND"

  step "npm install"
  if npm install --prefer-offline 2>&1 | tail -3; then ok "npm install"; else fail "npm install"; fi

  step "eslint --fix"
  if npx eslint --fix . 2>&1; then ok "eslint --fix"; else fail "eslint (unfixable issues remain — see above)"; fi

  step "tsc --noEmit"
  if npx tsc --noEmit 2>&1; then ok "tsc"; else fail "tsc (type errors — see above)"; fi
fi

# ══════════════════════════════════════════════════════
# Summary
# ══════════════════════════════════════════════════════
echo
if [ "$ERRORS" -eq 0 ]; then
  echo "✅  All checks passed."
  exit 0
else
  echo "❌  $ERRORS step(s) failed — review the output above."
  exit 1
fi

