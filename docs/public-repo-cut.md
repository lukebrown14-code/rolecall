# Public release checklist

Follow this once, after the v1.0 implementation is complete. `lukebrown14-code/rolecall` is
already public with its full history, and it stays that way (ADR-0009): this checklist proves
the tree, the history and the tracker hold no Owner data, then tags v1.0 in place. There is no
archive repository and no fresh history-less repository.

## 1. Confirm release blockers

- [ ] Fetch and merge the latest integration branch containing this checklist.
- [ ] Review the v1.0 release issue and its linked work. Confirm that every other v1.0 blocker
  has been merged into the release tree. Do not treat an open blocker as complete.
- [ ] Check the release tracker and each blocker in GitHub. Confirm every blocker other than this
  task is closed and its work is present in the release commit; check the release tree's
  history when the issue state alone does not show that the change merged.
- [ ] Confirm the release tree is the intended v1.0 source and has no uncommitted changes.
- [ ] Stop if any blocker remains open or any required work has not merged.

## 2. Run the personal-data guard over the tree, history and tracker

The guard works by Workspace structure: it fails when Workspace data (`workspace.json`,
`config.toml`, `jobs/`, `profile/`, `cv/`, `sent/`, `meta.yml`, PDFs, `.rolecall-*`) is tracked
outside `fixtures/` and `examples/`, and it requires fictional values inside them. It also
scans for emails, phone numbers, profile URLs and street addresses.

- [ ] Put your own name, former employers, locations and visa terms, one per line, in the
  gitignored `.rolecall-personal-data-terms` (check `git check-ignore
  .rolecall-personal-data-terms`). Do not add those terms to repository files. CI never sees
  them, so this run is the only Owner-terms check of the history and the tracker.
- [ ] Fetch every branch and tag (`git fetch --all --tags`), then from the repository root run:

  ```sh
  python3 scripts/check-personal-data.py . --history --github lukebrown14-code/rolecall
  ```

  This checks the tracked tree, every file version and commit message on any branch, tag or
  remote ref, and every issue and pull request title, body, comment and review (through `gh`,
  which must be signed in). It must exit zero with no findings. Do not widen the allowlist for
  real data.
- [ ] Fix each finding where it lives:
  - **Tracked tree**: edit the file and commit.
  - **Issue or pull request text**: edit it on GitHub, then open the item's "edited" history and
    delete every earlier revision that shows the term.
  - **History** (a file version or commit message no longer in the tree): rewrite the affected
    commits with `git filter-repo` and force-push every affected branch and tag. GitHub keeps
    pull request refs and cached views of the old commits, so also ask GitHub Support to purge
    them. Delete stale branches instead of rewriting them where possible.
- [ ] Run the full command again after any cleanup and confirm it exits zero.

Commit author names and emails are not scanned; they stay as they are. Before the release
commit, confirm `git config user.email` is the Owner's GitHub noreply address so new commits do
not add a personal email.

## 3. Read the publishable content by hand

- [ ] Read every file under `docs/`, including all ADRs.
- [ ] Read `README.md` and check its privacy statements and setup instructions.
- [ ] Read `.github/ISSUE_TEMPLATE/bug_report.md` and confirm it warns reporters not to paste
  `meta.yml`, a CV, or the Profile, and asks them to reproduce with the example Workspace.
- [ ] Read every file under `fixtures/` and `examples/`, including the showcase Workspace, its
  built PDFs and the generated-document fixtures. Confirm names and personal facts are
  fictional and no real Owner data remains.
- [ ] Resolve any concern and rerun the guard after edits.

## 4. Configure repository settings

- [ ] Protect `main`: require pull requests before merging, require the status checks `CI / go`
  and `CI / python` to pass, and prevent force pushes and branch deletion. Also require
  `CI / macos`. Do this after any history rewrite in step 2.
- [ ] Add repository topics that describe the project, such as `job-search`, `terminal`, and
  `privacy`.
- [ ] Confirm a push to `main` runs CI successfully; the personal-data guard needs no secret.

## 5. Tag v1.0

After `main` and its CI are verified:

```sh
git tag -a v1.0 -m "rolecall v1.0"
git push origin v1.0
```

- [ ] Verify the `v1.0` tag points to the reviewed release commit.
- [ ] Stop here. Do not close release issues as part of this checklist.
