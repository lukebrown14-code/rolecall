# Codebase Review Report

- **Branch:** `feat/codebase-review-simplify`
- **Base:** `origin/main` @ `df18409` (TUI polish: layout system, semantic theme, and internal packages #132)
- **Date:** 2026-11-12
- **Method:** five independent reviewers, each covering a distinct slice of the ~32k-line Go + Python codebase. Reviews are read-only; no source files were modified.

---

## Executive summary

| # | Slice | Score |
|---|-------|-------|
| 1 | Core / entry / config / CI | **5 / 10** |
| 2 | Store (data layer) | **6 / 10** |
| 3 | UI core (model / layout / theme / view) | **6 / 10** |
| 4 | UI features (detail / profile / settings / stats) | **6 / 10** |
| 5 | Python plugin / tests / docs | **5 / 10** |
| | **Overall** | **5.6 / 10** |

The architecture is fundamentally sound — `reporting` is a pure package with a clean seam, `theme` owns every colour with a real contrast test, `store` has an injected `FileSystem` and careful patch/preserve machinery, and the `screenDefs` "declare yourself" pattern is good. But the layers around those seams carry real duplication, **8 confirmed, reproduced bugs**, and measurable hot-path performance waste. Common themes across every slice: copy-pasted parsers and writers, God-files in the UI, duplicated key dispatch vs. help descriptions, and repeated full-tree / full-document reads on every frame.

---

## Confirmed bugs (🔴 critical)

1. **Cursor left out of range → `View()` panics** — `internal/ui/model.go:266` (overlay actions return before the cursor clamp at `:398-404` runs; `detail.go:140`, `view.go:192`, `view.go:311` then index unguarded). Reproduced on the Drafts screen by moving the last row.
2. **Mouse hit-testing breaks in ASCII mode** — `internal/ui/mouse.go:79-86, 128-131` (matches literal `─` and `×`, but `theme.ASCIIFold` rewrites them). With `ROLECALL_ASCII=1` every label/row click lands on the wrong target.
3. **`rolecall init` deadlocks on non-TTY stdin** — `cmd/rolecall/init.go:186` (default path enters a huh form with no `term.IsTerminal` check; reproduced 3/3 with `</dev/null`).
4. **`doctor` emits 30 false warnings on a fresh Workspace** — `internal/doctor/permissions.go:37` (walks `.git`, flags hooks/samples as 0644/0755).
5. **`writeWorkspaceConfig` diverges from `config.Patch`** — `cmd/rolecall/init.go:236-265` vs `internal/config/patch.go:21` (appends after tables, overwrites keys inside tables, different quoting). Three concrete, buggy divergences.
6. **Store writers mutate a newer-schema `workspace.json`** — `internal/store/tracks.go:27-66` (validates only "is JSON"; a `schema: 2` file is silently rewritten, against `SCHEMA.md`).
7. **`roles.json` rewrite matches any nested `"track"` key** — `internal/store/track_rename.go:108-117` (regex, not a parse; corrupts `notes` objects inside string values).
8. **argv bug in 5 of 7 Python scripts** — `plugin/scripts/meta.py:785`, `set-fit.py:29`, `new-job.py:606`, `sweep.py:656`, `sync-github.py:132` (`main(argv)` parses `argv` but resolves the Workspace from `sys.argv`).

---

## Agent 1 — Core / entry / config / CI — **5/10**

**Justification.** The shape is right — `reporting` is genuinely pure, `doctor`'s injected lookups make it testable without spawning processes (ADR-0003 respected), and `process` is a tight 45-line boundary. But the onboarding + settings half collapses: `init.go` re-implements `config.Patch` (worse, and they now disagree), `init` deadlocks on non-TTY stdin, `doctor` emits 30 false warnings on a Workspace it just created, and two privacy/tooling test files are never executed by CI.

### 🔴 Critical

- **C1 — `rolecall init` hangs forever when stdin is not a TTY.** `cmd/rolecall/init.go:186` (via `promptMissing`). The default path always runs a huh form with no `term.IsTerminal` check; on EOF it blocks forever in the event loop. The Workspace is sometimes already created by the time it hangs, so a retry then fails "not empty". Fix: detect non-TTY stdin and fall back to accessible mode or return a clear error.
- **C2 — `doctor` reports 30 false positives on a fresh Workspace.** `internal/doctor/permissions.go:37` (`WalkDir`). Essentially all `.git` internals. Fix: skip `.git` (and `SkipDir` on dot-directories) in the walk, plus a regression test running `git init` in the fixture root.
- **C3 — `writeWorkspaceConfig` is a divergent re-implementation of `config.Patch`.** `cmd/rolecall/init.go:236-265` vs `internal/config/patch.go:21`. Three divergences: appends after tables (vs. inserting before the first table), overwrites keys inside tables, and uses `json.Marshal` with a discarded error (vs. hand-rolled escaping). Fix: call `config.Patch(config.Path(home), "workspace", root)`.

### 🟠 Major

- **M1 — Two shell quoters, one redundant.** `internal/doctor/permissions.go:204` vs `internal/ui/handoff.go:55` produce the same `'"'"'` splice with different conditionals. `init.go:293` adds a third (JSON) quoter. Put one in a leaf package.
- **M2 — `Config` spends two full TOML parses to detect one missing key.** `internal/config/config.go:85-92`. Parse once into a struct with `FollowUpDays *int`; the swallowed error at `:92` is exactly what a missing `errcheck` linter lets through.
- **M3 — `reporting` duplicates the Response rule and hides a second default.** `internal/reporting/report.go:137` and `:276` ("Response = ever Interviewing or Offer" written twice), and `defaultFollowUpDays = 14` (`:15`) duplicates `config.defaultFollowUpDays`. Make `followUpDays` a required parameter; validate `follow_up_days`.
- **M4 — A bad StatusLog date makes an Application invisible with no warning.** `internal/reporting/report.go:189-197`. `parseLocal` returns `("", false)` for anything not `2006-01-02`/RFC3339, and callers just `continue`. Reuse `store.Application.Malformed` or count a skipped field.
- **M5 — `doctor` swallows the real cause and calls everything "offline".** `internal/doctor/permissions.go:86-93`. A rate-limit 403, DNS failure, proxy error, and deliberate skip are indistinguishable. Pass the message through.
- **M6 — `SetupReport.Warnings` is production-dead.** `internal/doctor/setup.go:122-130` (consumed only by tests; `main.go:96-106` re-implements the formatting). Pick one owner.
- **M7 — Lint config lets the whole class of bug through.** `.golangci.yml:3-7` enables only `dupl`, `goconst`, `mnd` — no `errcheck`, `staticcheck`, `unused`, `ineffassign`, `gosec`. `dupl: threshold: 100` is too high to catch the C3 clone.
- **M8 — The privacy-scanner test and pre-commit test are never run by CI.** `scripts/test_check_personal_data.py`, `scripts/test-pre-commit.sh` are referenced nowhere in `go.yml`.

### 🟡 Minor

- **Y1** `runInit` is an untested indirection (`main.go:57-59`) — delete it.
- **Y2** `initWorkspaceFiles` has a pointless tail (`main.go:61-69`).
- **Y3** Duplicated `home → config.Load → store.Open` preamble (`main.go:71-82` vs `111-120`) — extract a helper.
- **Y4** Inconsistent exit codes across subcommands (`main.go:20,35,45-47,53`).
- **Y5** `doctor` silently ignores extra positional args (`main.go:30-42`); `init` rejects them.
- **Y6** The 14-option `ui.New` call is unreadable and belongs in `ui` (`main.go:131`).
- **Y7** `init` cannot be scripted without answering every prompt (`init.go:163-184`).
- **Y8** Three copies of the "required" validator and four copies of the form options (`init.go`).
- **Y9** Magic strings for the Search-area remote rule in three places (`init.go:105,175,230`).
- **Y10** `writeWorkspaceConfig` is not atomic and does not repair directory modes (`init.go:237-264`); same defect in `config.write` (`patch.go:82-87`).
- **Y11** Four copies of the permission policy (`config/patch.go:13-16`, `store/modes.go:7-8`, `init.go:24-25`, `doctor/permissions.go:49-51`).
- **Y12** Capitalised mid-sentence error strings; two exact duplicates (`init.go:84,93`).
- **Y13** `wasSet` is O(flags) × 13 calls (`init.go:210-218`).
- **Y14** `editorSource` duplicates `ResolveEditor` (`config.go:47-57` vs `editor.go:7-14`).
- **Y15** `doctor` re-parses commands with `strings.Fields` (`doctor/setup.go:96-100`) — use `config.CommandName`.
- **Y16** `lookPathOnPath` resolves from CWD on an empty `$PATH` element (`doctor/setup.go:140`).
- **Y17** Windows code paths in a project that never builds Windows (`doctor/setup.go:142-148,161`).
- **Y18** Homebrew hints printed on every platform (`doctor/setup.go:71-73`).
- **Y19** `templateFonts` is a hard-coded coupling to the Plugin's font inventory (`doctor/setup.go:40-49`).
- **Y20** `config_test.go:21-22,31` ignore errors; `TestPlugin`/`TestHarness` duplicate a `write` closure.
- **Y21** `TestPlugin`'s last case asserts the opposite of what the code does (`config_test.go:76-81`).
- **Y22** `TestCLIInitRefusesNonEmptyDirectory` does not isolate `HOME` (`tests/main_test.go:273`).
- **Y23** Eight `go build` invocations, three ignoring the existing `buildRolecall` helper (`tests/main_test.go:16,31,72,179`).
- **Y24** `TestCLI` sets `GOCACHE` on the built binary's environment (inert noise).
- **Y25** Fixture-coupled "unit" test `TestStatsFunnelOverFixtures` (`report_test.go:23-89`).
- **Y26** `FunnelStatuses` is an exported mutable `var` (`reporting/report.go:19`).
- **Y27** `Rate int + OK bool` appears three times (`report.go:44-45,55-56,75-76`).
- **Y28** `weeksOf` keys a map by `time.Time` (`report.go:227-241`).
- **Y29** Four passes over `apps` plus an O(tracks × apps) inner loop (`report.go:94,124,146,147,263`).
- **Y30** `process.RunCaptured` mutates the caller's `*exec.Cmd`; no context/timeout (`process.go:29-35`).
- **Y31** `config.Patch` compiles a regexp on every call (`patch.go:41`).
- **Y32** Two different trailing-comment lexers (`init.go:267-287` vs `config/patch.go:61-80`).
- **Y33** `finishProfile` re-reads the config it just wrote (`init.go:300-303`).
- **Y34** Skill names are string literals in `cmd` (`init.go:293,306,308,319`).
- **Y35** `check-personal-data.py` silently skips undecodable files (`scripts/check-personal-data.py:111-114`).
- **Y36** `snap.sh` fails silently if the test is renamed (`scripts/snap.sh:10`).
- **Y37** `check-skills.py` is module-level code with no `__main__` and a too-broad regex (`scripts/check-skills.py:7-21`).
- **Y38** No version in the binary, no ldflags in the release (`.goreleaser.yaml`, `cmd/rolecall/main.go`).

### ⚪ Polish

- **P1** Import grouping inconsistent (`main.go:3-15`, `config.go:4-12`).
- **P2** `strconvQuote` is misnamed (it is JSON quoting, `init.go:289`).
- **P3** `githubRemoteParts = 2` reads as a magic-number appeasement (`doctor/permissions.go:29`).
- **P4** `t.Run(remote, …)` uses a URL as a subtest name (`doctor/remotes_test.go:46`).
- **P5** `tooNewToJudge`/`Judged`/`Stats`/`funnel` are vocabulary the GLOSSARY does not define.
- **P6** `doctor` reads Workspace files directly (an undocumented exception to CODING_STANDARDS).
- **P7** `config.Load` sets six `Source*` fields in three separate clumps.
- **P8** `Ensure` opens `O_RDWR` (`config.go:67`).
- **P9** `main.go:88` prints a period-terminated sentence among colon-formatted lines.

### Tooling / build

- **T1** golangci-lint runs twice per CI job (`.github/workflows/go.yml:29` + `scripts/check-go.sh:25`).
- **T2** The `ui` suite runs four times per push (`go.yml:31-32`).
- **T3** `cache: false` on `actions/setup-go` (`go.yml:9`, `release.yml:14`).
- **T4** The personal-data scanner runs twice on `main` (`go.yml:8,10-17`); no `concurrency:` block.
- **T5** `ruff`/`pytest` unpinned in CI (`go.yml:43`).
- **T6** `release.yml` releases untested tags; add goreleaser `before: hooks: - go test ./...`.
- **T7** goreleaser pinned to v2.9.0 for Homebrew; no tracking issue.
- **T8** `.githooks/pre-commit` doesn't run any tests.

### Agent 1 — Top 10

1. Delete `writeWorkspaceConfig`/`trailingTOMLComment`/`strconvQuote`; call `config.Patch` (`init.go:236-289`).
2. Skip `.git` in `doctor.Check`'s walk + regression test (`permissions.go:37`).
3. Refuse to run a huh form when stdin is not a TTY (`init.go:186`).
4. Make `config.Load` parse TOML once (`config.go:85-92`).
5. Make `reporting.Build`'s `followUpDays` a required parameter; delete the default (`report.go:15,81-85`).
6. Add `errcheck`, `staticcheck`, `unused`; lower `dupl` to ~60 (`.golangci.yml`).
7. Run `pytest scripts`, `sh scripts/test-pre-commit.sh`, `ruff check scripts` in CI (`.github/workflows/go.yml`).
8. Extract `isResponse(a)` and use it in both places (`report.go:137,276`).
9. Have `doctor.CheckSetup` use `config.CommandName` instead of `strings.Fields` (`setup.go:96-100`).
10. Fold `runDoctor`/`run`'s shared preamble into one helper; delete `runInit` (`main.go:57-59,71-82,111-120`).

---

## Agent 2 — Store (data layer) — **6/10**

**Justification.** The core `meta.yml` path — line-preserving patching, comment/unknown-key preservation, strict parsing with `path:line` errors that never echo private values, and the injectable `FileSystem` + golden tests — is genuinely strong. But the layer around it drifts from `SCHEMA.md` in ways that were reproduced: writers silently mutate a `schema: 2` `workspace.json`, a Track rename regex rewrites any nested `"track"` key, and the Contact table round-trip leaks `\|` into values.

### Contract

- 🔴 **`tracks.go:27-66` — writers mutate a newer-schema `workspace.json`.** `setWorkspaceKey` validates only "is valid JSON". Reproduced: `SetGate("letterWordCap","300")` on `{"schema":2,…}` returned `nil` and rewrote the file. Fix: read `schema` and refuse `> 1` (reuse one helper with `reads.go:28-42`).
- 🔴 **`settings.go:50-66` — `Settings()` never reads `schema`.** The Settings screen can neither show the upgrade warning nor avoid the silent rewrite. Fix: add `Schema`/`SchemaErr` to `WorkspaceSettings`.
- 🟠 **`notes.go:9-13` — `EnsureNotes` creates `notes.md` in a folder that is not an Application.** `Rescan` returns `(nil, nil)` for a missing `meta.yml`, and the result is discarded. Fix: refuse when `a == nil || a.Malformed != nil`.
- 🟠 **`track_rename.go:108-117` — the `roles.json` rewrite is a regex, not a parse.** Matches `"track"` at any depth, including inside string values. Fix: decode with `json.Decoder` and re-emit only `roles[].track`.
- 🟠 **`workspace_init.go:91` — the Contact pipe escape does not round-trip.** `\|` is written but `parseContact` (`profile.go:263-268`) doesn't unescape it.
- 🟡 **`folder.go:40-52` — `jd.md` front matter counts as text.** SCHEMA.md:56 says readers ignore it.
- 🟡 **`store.go:36-44` — `Open` does no schema check.** Startup half-loads a future Workspace.
- ⚪ **`store.go:57`, `create.go:17` — `"_archive"` is a magic string** never listed in SCHEMA's layout.

### Duplication

- 🟠 **Three `ValidTrackName`s.** `track_rename.go:125-138` and `settings/settings.go:118-131` are byte-identical; `create.go:17` is a third inline variant that has already drifted (refuses `_archive`, `.`, `..`, `\`). Fix: one `store.ValidTrackName`.
- 🟠 **Two independent `workspace.json` parsers** (`reads.go:12-44` vs `settings.go:37-83`), already drifted. Fix: one `readWorkspaceJSON(root)`.
- 🟠 **`FindByURL` is a full `Scan()`** (`reads.go:47-59`) — reads every `meta.yml` plus all presence stats to compare one string. Fix: reuse the `eachMeta` walker.
- 🟡 **`divider()` vs `tableDivider()`** (`profile.go:274-281` vs `:178-185`) — same predicate, different rune sets.
- 🟡 **`sent.go:46-68` and `download.go:13-35` duplicate** the "open source, open dest with `O_EXCL`, `io.Copy`, close" pattern with different error handling.
- 🟡 **`profileFileNames` and the `init` `files` map** hold the same Profile file list.
- 🟡 **Magic file names and keys** — `"meta.yml"`, `"notes.md"`, `"fit.md"`, `"status_log"`, `"url"`, `"closes"`, `"fit"`, `"resume_only"`, `"schema"` are bare strings while other keys are constants (`terms.go`).
- 🟡 **`search_area.go:27-30` re-declares `{Places, Remote}`** with different tags.
- 🟡 **Test fixtures copied inline 15 times** despite `fixtureProfileStore` (`profile_test.go:16`) existing.

### Performance

- 🟠 **`TrackUse` walks the whole tree; `settings.go:337-342` calls it once per Track** (`track_rename.go:45-54`). Fix: one `TrackCounts()` single pass.
- 🟠 **13+ syscalls per Application in `folder.go:16-81`.** Fix: one `ReadDir` to fill presence + `out/`, then read only needed files.
- 🟠 **`resume.md` read twice** whenever the Application is not Ready (`folder.go:48-52,71-79`).
- 🟡 **`AppendStatus` parses the same `meta.yml` three times** (`write.go:59,86,67`).
- 🟡 **`SetGate`/`RenameTrack` read `workspace.json` twice** with a TOCTOU window.
- 🟡 **A rename walks the tree three times** (`track_rename.go:28-41,79-94`).
- ⚪ **`meta.go:36-41` allocates four copies of the file** — use `bytes`.
- ⚪ **`write.go:70` compares `string(out) == string(b)`** — use `bytes.Equal`.

### Modularity

- 🟠 **The `FileSystem` seam covers only `meta.yml` writes.** `WriteDocument`, `EnsureNotes`, `CopyPDF`, `RecordSent`, `InitializeWorkspace`, and the `jd.md` write in `Create` all call `os.*` directly. Route every write through `FileSystem`.
- 🟠 **No single "this folder is an Application" gate.** That inconsistency produced the `EnsureNotes` bug. Fix: one `application(folder)` used by every folder-scoped method.
- 🟠 **`track_rename.go:79-119` — the Workspace-wide rename has no rollback and writes in the worst order.** Meta files first, then `workspace.json`, then `roles.json`. Write `workspace.json` + `roles.json` first.
- 🟡 **`Scan` and `eachMeta` are two near-identical tree walkers** with different rules.
- 🟡 **`Application` is a 22-field flat struct; `Schema` is dead** (only ever assigned `1`, zero readers outside the package).
- 🟡 **`workspace_init.go:51` — `"c" + "v"`** is unexplained obfuscation for `"cv"`.
- 🟡 **`sent.go:18-23` — a partial `sent/` is indistinguishable from a complete one.** Copy into `sent.tmp` then rename.
- ⚪ **`documents.go:10-16` — `ReadDocument`/`WriteDocument` accept an arbitrary `name`** allowing `../../x`.
- ⚪ **`workspace_init.go:70-81` writes from a `map[string]string`** — non-deterministic order.

### Maintainability

- 🟠 **`download.go:22-34` — a failed `CopyPDF` destroys the existing download and leaves a partial file.** Copy to `<name>.tmp`, sync, close, rename.
- 🟠 **Zero tests for seven exported functions:** `RecordSent`, `CopyPDF`, `EnsureNotes`, `ReadDocument`, `WriteDocument`, `InitializeWorkspace`, `ValidateWorkspacePath` — the two riskiest operations (irreversible `RecordSent`, `CopyPDF` writing outside the Workspace) have no coverage.
- 🟠 **`store_test.go:40-44` — `expected[i]` indexed by `range got`** with no length check; panics or passes silently.
- 🟡 **`track_rename.go:158-166` — `IsDir` checked after `os.ReadFile`.** Survives only because `ENOTDIR` ≈ `ErrNotExist`.
- 🟡 **`create.go:27-29` reports Add-form validation as `meta.yml:N: …` with an empty folder** (validates its own output, not the input).
- 🟡 **`gates.go:31-32` — a negative number reports "enter a whole number"** (locked in by `gates_test.go:29`).
- 🟡 **`write.go:30` discards `scalarToken`'s error.**
- 🟡 **Ignored errors throughout the tests** (`write_test.go:18`, `folder_test.go:39`, etc.).
- ⚪ **Untested branches that matter:** `entryCount`'s object branch, `orderedObject` duplicate keys, atomic `chmod`/`createtemp` failures, `Patch` inserting a key when no `status_log:` exists, `RenameTrack` partial failure.
- ⚪ **`build_integration_test.go:47` uses the literal `"python3"`** after resolving it with `exec.LookPath`.

### Readability

- 🟡 **`folder.go:9-14` — four one-line `const` blocks separated by blank lines.**
- 🟡 **Naming drifts from GLOSSARY:** `Documents` (`string`) holds a state, not documents; `HasSent` is a folder; `Sections[].Kind` mixes "entries/rows/bullets/items".
- 🟡 **`store.go:69-72` — one unreadable document kills the whole `Scan`.** Give `Application` a `FolderErr` and keep scanning.
- ⚪ **`meta.go:126` — missing-required-key errors always report line 1.**
- ⚪ **`terms.go:16` — `var Statuses = []string{…}` is mutable and exported.**

**What is good:** `scalar.go` and the patch/preserve machinery in `write.go` are careful, well-commented, and correctly tested. `atomic.go`'s chmod→write→sync→close→rename with defer cleanup is right. Error messages deliberately name the path without echoing values, with tests enforcing that.

### Agent 2 — Top 10

1. Enforce `workspace.json` schema in one place (`tracks.go:27`, `settings.go:50`).
2. Replace the `roles.json` regex with a JSON token walk (`track_rename.go:109`).
3. Make `EnsureNotes` refuse a non-Application (`notes.go:10`).
4. Fix the Contact `\|` round-trip (`workspace_init.go:91` or `profile.go:263`).
5. Delete the duplicated `ValidTrackName`; call `store.ValidTrackName`.
6. Collapse the three `workspace.json` parsers into one; make `FindByURL` use the `meta.yml`-only walk.
7. Replace per-Track `TrackUse` with one `TrackCounts()`.
8. Read `resume.md` once; cut `readFolder` to one `ReadDir` + needed reads.
9. Make `CopyPDF` write `<name>.tmp` then rename, cleaning up on failure.
10. Add tests for the seven untested exports; fix `store_test.go:40`'s unchecked `expected[i]`.

---

## Agent 3 — UI core (model / layout / theme / view) — **6/10**

**Justification.** The bones are genuinely good: `layout.go` is a clean size-budget seam, `theme` owns every colour with a real contrast test, `screenDefs` is a sound pattern, and comment discipline is better than most Go. But the slice carries a confirmed crash, a dead parallel rendering system in `columns.go` kept alive only by its own tests, a 1,050-line `view.go` that mixes layout, disk I/O and prose, and a frame that costs **10 ms / 2.8 MB / 47k allocations at 200 Applications** because it renders every row twice and discards 93% of them.

### 🔴 Critical

- **C1 — Cursor left out of range after an overlay action → `View()` panics.** `model.go:266-270` returns before the clamp at `:398-404`; `detail.go:140`, `view.go:192`, `view.go:311` index unguarded. Reproduced. Fix: clamp once inside `Update` before every return; route row access through one `selected()` accessor.
- **C2 — Mouse hit-testing breaks in ASCII mode.** `mouse.go:79-86,128-131` matches literal `─`/`×`, but `theme.ASCIIFold` rewrites them. Fix: render the rule/chip through a glyph resolver and match on that, or record geometry while drawing.

### 🟠 Major

- **M1 — `columns.go` is a dead parallel rendering system.** `RenderColumns` has zero production callers; `ApplicationColumns` has one use (a flag probe at `view.go:774`). Delete `RenderColumns` and its test; replace the flag probe with `followUpDue()`.
- **M2 — `view.go` is a 1,050-line god-file that does disk I/O inside `View()`.** `preview → docText → ReadDocument` runs three times per frame (`fit.md`, `jd.md`, `notes.md`). 17.4% of View CPU is `os.Open`. Fix: cache `(folder, file) → text` invalidated by the watch generation; move preview rendering into `preview.go`.
- **M3 — Rendering cost dominated by building rows that are never shown, twice.** `listBlocks` renders every row, then `scroll` keeps ~14; when overflowing it calls `build(true)` then `build(false)`. Measured: ~107 allocations per row, each row rendered twice. Fix: compute block heights arithmetically, render only `blocks[start:end]`.
- **M4 — `tabs()` recomputes every screen's counts once per candidate detail level.** `fits(level)` builds all 6 screens per level; one frame makes 60 `totalCount` scans, 48 from discarded builds. Fix: compute labels once.
- **M5 — Key dispatch implemented twice (act vs. describe) and they drift.** `model.go:308-394` vs `help.go:149-226`. Already drifted: `b`/`f`/`p` run unconditionally while help hides them. Fix: one table of `{key, help, priority, enabled, run}`.
- **M6 — Global keys re-implemented in four places** (`q`/`?`/`r` in `model.go`, `stats.go`, `profile.go`, `settings.go`), while `,`/`:` are correctly handled once. Fix: hoist them next to the `,`/`:` block.
- **M7 — `helpBox` mutates model state during `View()`** (`help.go:410-416`). Fix: compute paging in `Update`.
- **M8 — Settings is special-cased out of the pattern that exists to remove special cases** (`mapping.go:83-85` and `model.go:230-233`). Fix: add `enter: model.loadSettings` to the Settings `screenDef`.

### 🟡 Minor

- **m1** Five implementations of "read a date out of an `At`/`Closes` field" (`urgency.go:51-58`, `view.go:111-121`, `view.go:692-695`, `columns.go:19-28`, `detail.go:584-587`, plus a time-of-day-sensitive `columns.go:62-69`).
- **m2** Three scroll/clip implementations (`view.go:699-761`, `layout.go:176-203`, `layout.go:307-345`).
- **m3** `labelWidth` (`view.go:22`) is dead; date constants live far from use.
- **m4** `mode.boxed()` declared, implemented 11 times, never called.
- **m5** Dead declarations: `hintNext`/`hintSave`, redundant `case "q","ctrl+c"`, `previewW` when `!split`.
- **m6** State reconstructed by parsing English prose (`movedStatus` sniffs toast prefixes, `view.go:270-282`).
- **m7** ASCII coverage gaps in strings, not glyphs (`“”` at `view.go:178`, `docs.go:33`; `≤` at `view.go:485`).
- **m8** Accent used decoratively against CODING_STANDARDS (`view.go:259,392`, `styles.go:156`, `profile.go:549`, `settings.go:275,323`, `view.go:958`).
- **m9** Screen name and table name share literal strings with implicit coupling (`mapping.go:10-34`, `help.go:111`).
- **m10** Hardcoded screen counts in two places while a third computes it (`view.go:311`, `help.go:237`).
- **m11** `basePal`/`dimmedPal` are dark-theme globals used inside live rendering (`styles.go:49-53`, `view.go:779`).
- **m12** `m.err` conflates error kinds; callers re-derive with `errors.As`.

### ⚪ Polish

- **p1** `bothSides = 2` used as a divisor — rename `half`.
- **p2** Nine overlapping chrome constants across three files.
- **p3** Two identical pop-up chrome constants (`help.go:399`, `palette.go:12`).
- **p4** Three ad-hoc truncation budgets — one `truncate(s, budget)`.
- **p5** `overlay.go:30` mixes `len` and `xansi.StringWidth`.
- **p6** `p.StateColor` is a one-case `switch` with a callback-shaped signature.
- **p7** `joinColumns` builds a fresh style per call.
- **p8** `ago` returns `(string, int)`; most callers discard one half.
- **p9** `m.pal()`/`m.dimmedPal()` re-read env vars and rebuild ~20 colours on every call — compute once in `New`.
- **p10** Function aliases (`var isDark = …`) with no test seam.

**What is good:** `layout.go` is the right seam with size-class tests asserting the contract at every boundary. Colour/contrast discipline is real. `screenDefs` is a good "declaration, not conditionals" pattern. The help-key registry is a good idea — the problem is only that dispatch is a second, parallel copy.

### Agent 3 — Top 10

1. Fix the crash: clamp `m.cursor` on every `Update` exit; route row access through `selected()`.
2. Render only visible rows (compute heights, drop the double build) — removes ~90% of a 10 ms frame.
3. Stop reading documents in `View()`: cache `docText` per folder+file.
4. Make `tabs()` count once (pre-render label fragments).
5. Delete `RenderColumns` and the flag probe; keep `SortApplications`.
6. Make mouse hit-testing glyph-aware.
7. Unify key dispatch and help into one table; hoist `q`/`?`/`r` out of the screen handlers.
8. Give Settings an `enter` in `screenDefs`; delete both special cases.
9. Purge dead declarations (`labelWidth`, `boxed()`, `hintNext`/`hintSave`).
10. One date parser: `parseDay`/`dayDiff`/`followUpDue`.

---

## Agent 4 — UI features (detail / profile / settings / stats) — **6/10**

**Justification.** Good engineering instincts: one `frame`/`layout` abstraction, one `mode` interface instead of an overlay switch, colour roles in `pal`, `ui` never touching Workspace files, and behaviour tests driving `Update` with faked externals. But the Settings family (~1,550 lines across 7 files + a subpackage) is four near-identical copies of the same group-handler pattern wired by hand-rolled index chains whose group/row identity is hardcoded integers that break silently. Plus a facade file of dead re-exports, a dozen dead constants/fields, `reporting.Build` executed 2–3× per frame, and N full `meta.yml` scans per Settings load.

### Duplication

- 🟠 **D1 — Prefs dispatch copy-pasted four times** (`settings_tracks.go:31-49`, `settings_gates.go:16-32`, `settings_search.go:22-37`, `settings_machine.go:215-227`). Fix: one `groupHandler` table.
- 🟠 **D2 — Four hand-built undo closures** (each save repeats "write → loadSettings → build undo → saved"). Fix: one `savedWrite` helper.
- 🟠 **D3 — `detailCard` is a copy of `preview`** (`detail.go:677-699` vs `view.go:946-990`). Fix: one `cardBody(p, a, inner, link)`.
- 🟠 **D4 — Six copies of modal boilerplate** (`mode.go:163-245`, `settings_tracks.go:214-224`, `settings_rename.go:107-125`).
- 🟠 **D5 — Editor launch in three shapes** (`external.go:147-190`). Fix: one `runEditor(folder, path, line)`.
- 🟠 **D6 — Settings group identity duplicated as magic ints** (`settings_search.go:14`, `settings_gates.go:13`, `settings_tracks.go:16`, `settings_machine.go:28`). Fix: export `settings.Group*` constants.
- 🟠 **D7 — Constants declared two or three times** (focus enum, file names, field labels, `fieldURL`, `yesNo`, `plural`, form metrics).
- 🟡 **D8 — Two "Can't read …" formatters** (`feedback.go:69-74` vs `:78-83`).
- 🟡 **D9 — Workspace-relative stripping twice** (`search.go:15`, `feedback.go:57`).
- 🟡 **D10 — Seven copies of the fake editor runner in tests.** Fix: one `recordingEditor`.

### Readability

- 🟠 **R1 — `detailView` is 80 lines of nested conditionals** (`detail.go:409-487`). Extract `detailTitle`/`detailWhen`/`detailPane`.
- 🟠 **R2 — `settingsRowsView` is 78 lines with three width passes** (`settings.go:289-366`).
- 🟡 **R3 — `settingsKey` mixes six concerns** (`settings.go:139-192`); unknown keys fall into stringly navigation.
- 🟡 **R4 — A field label derived from the last word of a sentence** (`settings_gates.go:39-41`).
- 🟡 **R5 — `any`-typed setting values** (`settings_machine.go:162-196`).
- 🟡 **R6 — Bare string keys beside constants** (`settings_machine.go:103-108`).
- 🟡 **R7 — A hand-rolled shell tokenizer in the UI layer** (`external.go:44-79`).
- 🟡 **R8 — `esc` means different things per screen** (search-filter clear not centralized).
- 🟡 **R9 — Toasts parsed as state** (`view.go:270-282`, `feedback.go:128`, `move.go:64-74`).
- ⚪ **R10 — Inconsistent tab constants** (`detail.go:26`).

### Modularity

- 🟠 **M1 — `form.go` vs `forms/form.go` are a facade with dead weight** (`form.go:12-27`, `forms/form.go:70-72`).
- 🟠 **M2 — The generic form engine knows the Add form's URL field** (`forms/form.go:134,321`).
- 🟠 **M3 — The `settings` package leaks display order into `ui`** (mutable `Groups` slice indexed directly).
- 🟡 **M4 — A generic with one instantiation** (`settings.State[Undo any]`).
- 🟡 **M5 — The Document set described in five places** (`docs.go:14-22,54-55,74-80`, `detail.go:24-49`, `watch.go:241-248`).
- 🟡 **M6 — Settings held together by call order** (`settingsKey`'s chain: `renameKey` before `r` rescan).
- 🟡 **M7 — `settingsMachine()` rebuilt on every keystroke** (`settings.go:409-412`).
- 🟡 **M8 — Two ways to enter a screen** (Profile uses `enter`; Settings special-cased).

### Performance

- 🟠 **P1 — Stats builds its whole report 3× per frame** (`stats.go:73-77`, plus one per `j/k/G`). Fix: build once per `Update`.
- 🟠 **P2 — Settings load re-parses every `meta.yml` once per Track** (`settings/settings.go:331-344` → `store.TrackUse`). Fix: `store.TrackUseAll()`.
- 🟠 **P3 — Detail view re-reads and re-renders Markdown on every message** (`detail.go:206-230` → Glamour render each `Update`). Fix: cache keyed on `(contentHash, width, theme)`.
- 🟡 **P4 — Same pattern on Profile** (`profile.go:175-191`).
- 🟡 **P5 — `r` on Profile rescans the whole Workspace** for a screen that shows neither.
- 🟡 **P6 — Settings rows rebuilt several times per frame.**
- 🟡 **P7 — Row list re-filtered and re-sorted on every call** (`rows()`/`current()`).
- 🟡 **P8 — Two counting paths for Track use** (`settings_tracks.go:157-171` vs `State.TrackUse`).

### Maintainability

- 🟠 **T1 — Dead code** (`detail.go:63`, `settings.go:384,386,388,406`, `form.go:20,25`, `forms/form.go:70-72`; `keyBackspace` in the wrong home).
- 🟠 **T2 — `nextAction` hardcodes the keymap** (`next.go:27-71`), which must stay in sync with `model.go` and `help.go`.
- 🟠 **T3 — Parse fixes keyed by the parser's prose** (`feedback.go:21-33`). Export typed reasons.
- 🟠 **T4 — Enter-on-row depends on positional coincidence** (`settings_gates.go:27`, `settings_search.go:32`, `settings_machine.go:68`). Identify rows by key/label.
- 🟡 **T5 — The watcher fails silently** (`watch.go:97-100,144-169`).
- 🟡 **T6 — `loadSettings` mutates state after installing it** (`settings.go:87-88`).
- 🟡 **T7 — A failed rename wipes move-undo** (`settings_rename.go:128-143`).
- 🟡 **T8 — Detail view silently swallows shared actions** (`detail.go:335-338`).
- 🟡 **T9 — `nextLine` may return two lines; detail reserves one** (`next.go:99-105` vs `detail.go:472-476`).
- 🟡 **T10 — Downloads don't report skips** (`download.go:47-52,74-78`).
- 🟡 **T11 — Context row promises a file `e` won't open** (`settings.go:102-103`).
- 🟡 **T12 — Tests coupled to backspace counts and row offsets.**
- 🟡 **T13 — Missing error-path coverage** (patch, CopyPDF failure, SetGate failure, editConfig failure).
- 🟡 **T14 — Inconsistent locking in a test helper** (`detail_test.go:295`).
- ⚪ **T15 — Tests re-declare production strings.**

**What is good:** `mode` + `Model.Update` routing is the right shape. `frame`/`layout`/`ClipLines`/`MoreCue` centralise layout properly. `pal` role naming holds. `docs.go` keeps the Document vocabulary precise. `watch`/`debounce` injection of the scheduler is clean and tested without sleeping.

### Agent 4 — Top 10

1. Collapse the four Settings group handlers into one table.
2. Export group identity from the `settings` package; delete the magic ints.
3. One-pass track counting (`store.TrackUseAll()`).
4. Build the Stats report once per `Update`.
5. Cache the rendered body in `detailState`/`profileState`.
6. Delete dead code (`settings.go:384-406`, `form.go:20,25`, `forms/form.go:70-72`, `detail.go:63`).
7. Share one card body between detail card and preview.
8. Identify Settings rows by key/label instead of position.
9. Fold the three editor launches into one `runEditor`.
10. Make the toast a typed value, not a formatted string parsed in three files.

---

## Agent 5 — Python plugin / tests / docs — **5/10**

**Justification.** The core design is genuinely good: `meta.py` is a single validating atomic writer, `identity.py` keeps the Owner's name out of the code, the Python writers are tested at the CLI seam against the *Go* goldens, and `glossary_test.go` mechanically enforces GLOSSARY terms on Go string literals. But the slice is riddled with copy-paste: four `workspace.json` readers, seven identical `--data` flags (five dead), seven `sys.path.insert` preambles, three table parsers, 36 copies of `os.CopyFS` in Go tests, nine near-identical `*Harness` types each with its own `wait()`, and five copies of the same Contact table. Worst, the `main(argv=None)` seam is a lie in five of seven scripts.

### Duplication

- 🔴 **D-1 — Same argv bug in 5 of 7 scripts** (`meta.py:785-786`, `set-fit.py:29`, `new-job.py:606`, `sweep.py:656`, `sync-github.py:132`). Fix: one `resolve_workspace(args.data)`; delete `workspace_from_cli()`.
- 🟠 **D-2 — The same `--data` argument declared seven times and used zero times.** Fix: `workspace.add_data_argument(parser)` + `resolve(...)`.
- 🟠 **D-3 — Four independent readers of `workspace.json`** (`new-job.py:109-113`, `sweep.py:120-125`, `build.py:46-58`, `migrate.py:250-261`). Fix: one `workspace.load(path)`.
- 🟠 **D-4 — `migrate.plan()` builds the SHORTLIST/DISMISSED changes twice** (`migrate.py:464-545`).
- 🟠 **D-5 — Two full-HTML fetchers with divergent error handling** (`new-job.py:362-392` vs `sweep.py:195-203`).
- 🟠 **D-6 — Two `scan()` functions with different vocabularies for the same signal** (`new-job.py:149-153` vs `sweep.py:606-616`); the consumer (`fetch.go:103-110`) throws the kind away.
- 🟡 **D-7 — Contact table copied five times** (`master-profile.md`, `profile-contact.json`, `test_build.py`, `test_identity.py`, `test_create_profile_contract.py`).
- 🟡 **D-8 — The same golden asserted twice from Python** (`test_meta_writes.py:33-44` vs `test_set_fit.py:178-187`).
- 🟡 **D-9 — Three Markdown table parsers** (`identity.py:22`, `build.py:249-259`, `migrate.py:159-177`).
- 🟠 **D-10 — Go: 36 hand-rolled fixture copies, four different relative depths**, and the shared helper buried in `layout_test.go`.
- 🟠 **D-11 — Nine harness types, nine `wait()`s, two identical editor fakes.**
- 🟡 **D-12 — `buildHarness` re-implements the command drain loop** that `drive_test.go` already owns.
- 🟡 **D-13 — `resume.typ` and `cover-letter.typ` duplicate the same preamble.**

### Readability

- 🟠 **R-1 — `sweep.main()` is a 170-line God function** (`sweep.py:654-820`); same shape in `new-job.main()` and `migrate.plan()` (245 lines with a nested closure).
- 🟠 **R-2 — `new-job.warn()` is named like a warning but always exits** (`new-job.py:420-425`).
- 🟡 **R-3 — The 200-character sanity check written twice** (`new-job.py:637-638,652-653`).
- 🟡 **R-4 — `migrate.Report` is a one-field wrapper around `str`** dispatched by `isinstance` chains.
- 🟡 **R-5 — `meta._read_meta(folder, data)` takes a folder purely for an error string** (`meta.py:253-258`).
- 🟡 **R-6 — `build.owner_identity()` is a one-line wrapper hiding a global** (`build.py:66-73`).
- ⚪ **R-7 — `driver` keeps teatest method names and a `...any` variadic** (`drive_test.go:125-126`).
- ⚪ **R-8 — Two key-driving APIs in the same package** (`drive_test.go:119` vs `:51`).
- ⚪ **R-9 — Stale snapshot names and a duplicated snapshot state** (`snapshot_test.go:18,41,55`).

### Modularity

- 🟠 **M-1 — The "shared Workspace library" is called `meta.py`, and `workspace.py` is 53 lines.** Fix: split `workspaceio.py` (`atomic_write`, `quote_scalar`, folder naming) out of `meta.py`.
- 🟠 **M-2 — `SCHEMA.md` calls itself the contract but omits `jobs/roles.json` and `jobs/sweep-candidates.md`.** The "anything else under `jobs/` is ignored" sentence actively mis-describes `roles.json`.
- 🟡 **M-3 — `blockers` in `workspace.json` has no documented shape and no validation** (a malformed value raises `re.error` as a traceback).
- 🟠 **M-4 — The repo-level personal-data scanner is tested from the plugin's test suite** (`test_personal_data_scanner.py`). Wrong home; move to `scripts/tests/`.
- 🟠 **M-5 — Python scripts can't be imported as a package; each repeats the same bootstrap** (`sys.path.insert` ×7; `sweep.py:52-58` loads `new-job.py` by path as `"nj"`).
- 🟠 **M-6 — Global mutable state in `build.py` and `sweep.py` forces tests to re-import modules** (`build.py:38,48`, `sweep.py:117,242,517`).
- 🟡 **M-7 — `identity.py` is single-consumer and overlaps `build.py`'s Profile reading.**

### Performance

- 🟡 **P-1 — `master-profile.md` read from disk 4+ times per build; each source document 3-5 times** (`build.py`). Fix: read once in `main`, pass text into the gates.
- 🟠 **P-2 — `migrate.plan()` re-reads `workspace.json` once per row** (`migrate.py:473,514`).
- 🟠 **P-3 — `migrate.apply()` writes `meta.yml` twice per folder** (`migrate.py:603-605`).
- ⚪ **P-4 — `sweep.py:752-753` splits kept roles with O(n²) membership.**
- ⚪ **P-5 — `_word_re` recompiles a regex per Place per candidate** (`sweep.py:492-493,558`).
- ⚪ **P-6 — `sync-github` runs three `gh` subprocesses per repo, serially.**
- ⚪ **P-7 — Go tests copy the 152 KB / 55-file fixture 36 times per package run.**

### Maintainability

- 🟠 **M-8 — Dead code that tests keep alive** (`build.fit_verdict`, `new-job.fetch`).
- 🟠 **M-9 — The cross-language contract test cannot see a new field** (`test_meta_contract.py:15-62` projects onto a hand-maintained key list).
- 🟠 **M-10 — `build.load_config` trusts `workspace.json` types and crashes on hand edits** (`build.py:55-58` → `TypeError`).
- 🟡 **M-11 — Observed flaky live test** (`live_test.go:173-196`) — scroll assertion races the watcher.
- 🟠 **M-12 — `scripts/test_check_personal_data.py` is tab-indented, one test, and never runs in CI.**
- 🟡 **M-13 — Test-only date dependence in `test_newjob.py`** (crosses-midnight failure).
- 🟠 **M-14 — Track selection logic exists in three places with three behaviours**, including `TRACK_KEYWORDS` hardcoding that contradicts `find-jobs/SKILL.md:25`.
- ⚪ **M-15 — Build junk not gitignored** (`.pytest_cache/`, `.ruff_cache/`).
- ⚪ **M-16 — `ruff.toml`'s `line-length = 100` is decorative** (E501 not selected; 54 lines exceed 100).
- ⚪ **M-17 — `sync-github` writes `projects.md` non-atomically and parses `gh` output unguarded.**
- ⚪ **M-18 — Writers that "refuse" still print to stdout** (gates `sys.exit`), making "failed" vs "warned" indistinguishable.

### Consistency (docs ↔ code ↔ GLOSSARY)

- 🟠 **C-1 — `plugin/README.md:103` points at a test file that does not exist at that path** (`store/build_integration_test.go` → `internal/store/tests/build_integration_test.go`).
- 🟠 **C-2 — `find-jobs/SKILL.md` mis-describes the `sources` shape** (says `platform: target` entries; it's a platform → list mapping).
- 🟠 **C-3 — GLOSSARY forbids "empty" as a Document state; `SCHEMA.md:100` and `build.py:619-622` use it.** The `glossary_test.go` guard covers Go string literals only.
- 🟡 **C-4 — The plugin uses "identity", which GLOSSARY lists under Contact's `_Avoid_`.**
- 🟡 **C-5 — Docs' file inventories have drifted from `plugin/scripts/`** (`docs/spec.md:43`, `plugin/writing-rules.md:151-152`).
- 🟡 **C-6 — Stale "dashboard" references** (`sweep.py:9`, `plugin/README.md:112`).
- 🟡 **C-7 — `sync-github`'s marker names the wrong path** (`scripts/sync-github.py` → `plugin/scripts/sync-github.py`).
- 🟡 **C-8 — `CODING_STANDARDS.md` says only three test kinds keep a whole teatest program; `acceptance_test.go` breaks that.**
- ⚪ **C-9 — `assess-fit` says "score a job" (a GLOSSARY `_Avoid_` for Fit); three copies of the four verdicts.**
- ⚪ **C-10 — Skill/script names use "job" where GLOSSARY says Application** (carved out only in a Go test map).
- ⚪ **C-11 — `test_skills.py` guards prose drift for three files only.**

### Agent 5 — Top 10

1. One Workspace seam (`resolve(args.data)`); delete every `workspace_from_cli()`; add in-process `main(["--data", …])` tests.
2. `workspace.load()` returning tracks/search_area/gates/blockers/sources; delete the four readers.
3. New `signals.py` (move `BLOCKERS`/`FAVOURABLE`/`blocker_hits`/one `scan`); delete the `importlib` load of `new-job.py`; keep the signal kind through `fetch.go`.
4. New `workspaceio.py` (move `atomic_write`, `quote_scalar`, folder naming out of `meta.py`); use `atomic_write` in `sync-github.py`.
5. `SCHEMA.md` — document `jobs/roles.json` and `jobs/sweep-candidates.md`; fix the "anything else under jobs/ is ignored" sentence.
6. `helpers_test.go` in both Go test packages — move `fixtureCopy`, `writeFile`, `readFile`, `fixtureStore`; replace all 36 `os.CopyFS` blocks.
7. Collapse nine harnesses into one configurable `harness` with one `wait()` and one recording-editor fake.
8. `build.py` — read each file once per run, pass text into gates; make gates return `list[str]` instead of `sys.exit`.
9. `migrate.py` — one `apply` write per folder (`meta.update`), hoist `_default_track` out of row loops, dedupe the SHORTLIST/DISMISSED builders.
10. Docs sweep: fix `plugin/README.md:103` path, `find-jobs/SKILL.md:16-17` source shape, file inventories, "dashboard" references, and rename the "identity" gate to "Contact".

---

## Consolidated priorities

### Highest-impact (fix these first)

1. **Cursor clamp + `selected()` accessor** — fixes the panic (`model.go`, `detail.go`, `view.go`).
2. **Single `workspace.json` reader + schema guard** — fixes the schema-2 mutation and collapses three parsers (`store`).
3. **Delete `init`'s config writer; call `config.Patch`** — fixes three config bugs and removes a divergent re-implementation.
4. **One Python Workspace seam (`resolve(args.data)`)** — fixes the argv bug in 5 scripts and deletes 7 hand-rolled `--data` flags.
5. **Unify key dispatch and help into one table** — removes a whole class of drift between what keys do and what help says.

### Quick wins (low risk, high leverage)

- Enable `errcheck` + `staticcheck` + `unused`; lower `dupl` threshold to ~60 (`.golangci.yml`).
- Skip `.git` in the `doctor` walk + regression test.
- `store.TrackUseAll()` single-pass track counting.
- Delete dead code (`RenderColumns`, `boxed()` ×11, unused constants/fields, dead re-exports).
- Cache detail/profile/preview renders keyed on the watch generation.
- Hoist shared Go test helpers (`fixtureCopy`, `recordingEditor`, the nine harnesses).
- Document `roles.json` / `sweep-candidates.md` in `SCHEMA.md`.

### Cross-cutting themes

- **Duplication (~46 findings)** is the dominant problem: multiple config writers, `workspace.json` readers, `ValidTrackName`s, date parsers, key/help dispatch, shell quoters, editor launchers, test harnesses, and fixture copies.
- **Performance (~21 findings)** is concentrated in the UI hot path (row rendering ×2, document reads ×3/frame, Stats ×3/frame, N tree walks per Settings load) and in the Python build/migrate paths (files read 3–5× each).
- **Modularity (~24 findings)** is strongest in the store's `meta.yml` handling and weakest in the Settings family (7 files + subpackage wired by magic-int indices) and the Python scripts (shared logic trapped in `meta.py`).
