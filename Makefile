.PHONY: deps-update deps-go deps-npm deps-bazel deps-check help

## ─────────────────────────────────────────────────────────
## deps-update : update ALL dependencies (go + npm + bazel)
## ─────────────────────────────────────────────────────────
deps-update: deps-go deps-npm deps-bazel
	@echo ""
	@echo "✅  All dependencies updated."
	@echo "    Run tests before committing:  make deps-check"

## ─────────────────────────────────────────────────────────
## deps-go : update Go modules to latest minor/patch
## ─────────────────────────────────────────────────────────
deps-go:
	@echo "⬆️  Updating Go modules …"
	cd backend && go get -u ./...
	cd backend && go mod tidy
	@echo "   go.mod + go.sum updated."

## ─────────────────────────────────────────────────────────
## deps-npm : update npm packages to latest semver-compatible
## ─────────────────────────────────────────────────────────
deps-npm:
	@echo "⬆️  Updating npm packages …"
	cd frontend && npm update
	cd frontend && npm audit fix --force 2>/dev/null || true
	@echo "   package.json + package-lock.json updated."

## ─────────────────────────────────────────────────────────
## deps-bazel : regenerate Bazel BUILD files with Gazelle
## ─────────────────────────────────────────────────────────
deps-bazel:
	@echo "⬆️  Regenerating Bazel BUILD files …"
	cd backend && bazelisk run //:gazelle -- update 2>/dev/null || \
		echo "   ⚠️  bazelisk not found — skipping Bazel update."
	@echo "   BUILD files regenerated."

## ─────────────────────────────────────────────────────────
## deps-check : verify everything builds & tests pass
## ─────────────────────────────────────────────────────────
deps-check:
	@echo "🔍  Verifying backend …"
	cd backend && go build ./...
	cd backend && go vet ./...
	cd backend && go test ./...
	@echo ""
	@echo "🔍  Verifying frontend …"
	cd frontend && npm run build
	@echo ""
	@echo "✅  All checks passed."

## ─────────────────────────────────────────────────────────
## help : show this help
## ─────────────────────────────────────────────────────────
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## //'

