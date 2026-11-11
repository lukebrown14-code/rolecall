#!/bin/sh
# Mechanical Go checks shared by CI and the pre-commit hook.
set -eu
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "gofmt needed:"; echo "$unformatted"; exit 1
fi

# ADR-0003: only ui/external.go may launch processes.
launches=$(grep -rln 'os/exec\|ExecProcess' --include='*.go' . | grep -v '_test\.go$' | grep -v '^\./ui/external\.go$' || true)
if [ -n "$launches" ]; then
  echo "process launch outside ui/external.go (ADR-0003):"; echo "$launches"; exit 1
fi

go vet ./...
if command -v golangci-lint >/dev/null; then
  golangci-lint run ./...
else
  echo "golangci-lint not installed; skipping (CI runs it)"
fi
