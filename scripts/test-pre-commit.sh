#!/bin/sh
set -eu
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/scripts" "$root/bin"
cp .githooks/pre-commit "$root/pre-commit"
cat >"$root/bin/python3" <<'EOF'
#!/bin/sh
exit "${SCANNER_STATUS:-0}"
EOF
cat >"$root/scripts/check-go.sh" <<'EOF'
#!/bin/sh
touch "$CHECK_GO_MARKER"
EOF
chmod +x "$root/bin/python3" "$root/scripts/check-go.sh"

if (cd "$root" && PATH="$root/bin:$PATH" SCANNER_STATUS=1 CHECK_GO_MARKER="$root/ran" sh ./pre-commit); then
  echo "pre-commit succeeded after scanner failure" >&2
  exit 1
fi
if [ -e "$root/ran" ]; then
  echo "check-go ran after scanner failure" >&2
  exit 1
fi
(cd "$root" && PATH="$root/bin:$PATH" SCANNER_STATUS=0 CHECK_GO_MARKER="$root/ran" sh ./pre-commit)
[ -e "$root/ran" ]
