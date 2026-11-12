#!/bin/sh
# Mechanical Go checks shared by CI and the pre-commit hook.
set -eu
cd "$(dirname "$0")/.."

unformatted=$(gofmt -l .)
if [ -n "$unformatted" ]; then
  echo "gofmt needed:"; echo "$unformatted"; exit 1
fi

# ADR-0003: process/process.go is the only process launch boundary.
launches=$(grep -rln 'exec.Command\|tea.ExecProcess\|CombinedOutput\|c.Run()' --include='*.go' . | grep -v '_test\.go$' | grep -v '^\./process/process\.go$' || true)
if [ -n "$launches" ]; then
  echo "process launch outside process/process.go (ADR-0003):"; echo "$launches"; exit 1
fi

# Every colour lives in ui/styles.go: no colour or escape literals anywhere else.
colours=$(grep -rnE 'lipgloss\.Color\(|"#[0-9A-Fa-f]{6}"|\\x1b\[|\\033\[' --include='*.go' . | grep -v '_test\.go:' | grep -v '^\./ui/styles\.go:' || true)
if [ -n "$colours" ]; then
  echo "colour literal outside ui/styles.go:"; echo "$colours"; exit 1
fi

go vet ./...
if command -v golangci-lint >/dev/null; then
  golangci-lint run ./...
else
  echo "golangci-lint not installed; skipping (CI runs it)"
fi
