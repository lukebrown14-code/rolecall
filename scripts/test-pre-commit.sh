#!/bin/sh
set -eu
root=$(mktemp -d)
trap 'rm -rf "$root"' EXIT
mkdir -p "$root/scripts" "$root/bin"
cp .githooks/pre-commit "$root/pre-commit"
cat >"$root/bin/python3" <<'EOF'
#!/bin/sh
printf '%s\n' "$*" >"$SCANNER_ARGS"
exit "${SCANNER_STATUS:-0}"
EOF
cat >"$root/scripts/check-go.sh" <<'EOF'
#!/bin/sh
touch "$CHECK_GO_MARKER"
EOF
chmod +x "$root/bin/python3" "$root/scripts/check-go.sh"

if (cd "$root" && PATH="$root/bin:$PATH" SCANNER_STATUS=1 SCANNER_ARGS="$root/args" CHECK_GO_MARKER="$root/ran" sh ./pre-commit); then
  echo "pre-commit succeeded after scanner failure" >&2
  exit 1
fi
if [ -e "$root/ran" ]; then
  echo "check-go ran after scanner failure" >&2
  exit 1
fi
(cd "$root" && PATH="$root/bin:$PATH" SCANNER_STATUS=0 SCANNER_ARGS="$root/args" CHECK_GO_MARKER="$root/ran" sh ./pre-commit)
[ -e "$root/ran" ]
if [ "$(cat "$root/args")" != "scripts/check-personal-data.py --staged ." ]; then
  echo "pre-commit must scan the staged content: got '$(cat "$root/args")'" >&2
  exit 1
fi

# End to end with the real scanner: the hook judges the index, not the working copy.
repo="$root/repo"
mkdir -p "$repo/scripts"
cp .githooks/pre-commit "$repo/pre-commit"
cp scripts/check-personal-data.py "$repo/scripts/"
cp "$root/scripts/check-go.sh" "$repo/scripts/check-go.sh"
unset GIT_DIR GIT_INDEX_FILE GIT_WORK_TREE
git init --quiet "$repo"
email="jane.real""@""private-domain.com"
run_hook() {
  rm -f "$root/ran"
  (cd "$repo" && CHECK_GO_MARKER="$root/ran" sh ./pre-commit >/dev/null 2>&1)
}

printf 'contact: %s\n' "$email" >"$repo/notes.md"
git -C "$repo" add notes.md
printf 'clean\n' >"$repo/notes.md"
if run_hook; then
  echo "pre-commit passed with personal data staged behind a clean working copy" >&2
  exit 1
fi

git -C "$repo" add notes.md
printf 'contact: %s\n' "$email" >"$repo/notes.md"
if ! run_hook; then
  echo "pre-commit failed on unstaged personal data with a clean stage" >&2
  exit 1
fi
[ -e "$root/ran" ]
