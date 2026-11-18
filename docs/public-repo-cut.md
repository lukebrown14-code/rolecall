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

## 2. Run the personal-data scanner with Owner terms

Put one Owner-specific term per line in the gitignored `.rolecall-personal-data-terms` file at
the repository root. Include the Owner's name and known identifying terms such as former
employers, locations, and visa terms; do not add those terms to repository files.

- [ ] Confirm `.rolecall-personal-data-terms` contains the complete Owner-specific term list and
  is not tracked (`git check-ignore .rolecall-personal-data-terms`).
- [ ] Fetch every branch and tag (`git fetch --all --tags`), then from the repository root run:

  ```sh
  python3 scripts/check-personal-data.py . --history --github lukebrown14-code/rolecall
  ```

  This scans the tracked tree, every file version and commit message reachable from any ref,
  and every issue and pull request title, body, comment and review (through `gh`, which must be
  signed in). It reads the default terms file automatically. To store the terms file elsewhere,
  set `ROLECALL_PERSONAL_DATA_TERMS_FILE=/absolute/path/to/terms` for this command.
  Alternatively, provide newline-separated terms with `ROLECALL_PERSONAL_DATA_TERMS`; the
  scanner adds these to the terms in the file. The command must exit zero with no findings. Do
  not add Owner terms to the allowlist.
- [ ] Fix each finding where it lives:
  - **Tracked tree**: edit the file and commit.
  - **Issue or pull request text**: edit it on GitHub, then open the item's "edited" history and
    delete every earlier revision that shows the term.
  - **History** (a file version or commit message no longer in the tree): rewrite the affected
    commits with `git filter-repo` and force-push every affected branch and tag. GitHub keeps
    pull request refs and cached views of the old commits, so also ask GitHub Support to purge
    them. Remove stale branches instead of rewriting them where possible.
- [ ] Run the full scan again after any cleanup and confirm it exits zero.

Commit author names and emails are not scanned; they stay as they are. Before the release
commit, confirm `git config user.email` is the Owner's GitHub noreply address so new commits do
not add a personal email.

## 3. Read the publishable content by hand

- [ ] Read every file under `docs/`, including all ADRs.
- [ ] Read `README.md` and check its privacy statements and setup instructions.
- [ ] Read `.github/ISSUE_TEMPLATE/bug_report.md` and confirm it warns reporters not to paste
  `meta.yml`, a CV, or the Profile, and asks them to reproduce with the example Workspace.
- [ ] Read every file under `fixtures/`, including the example Workspace and generated-document
  fixtures. Confirm names and personal facts are fictional and no real Owner data remains.
- [ ] Resolve any concern and rerun the scanner after edits.

## 4. Configure CI and repository settings

- [ ] In **Settings → Secrets and variables → Actions**, create the repository secret
  `ROLECALL_PERSONAL_DATA_TERMS`. Set its value to the Owner-specific terms, one per line. Keep
  this value out of commits, issues, and logs.
- [ ] Confirm the workflow's Owner-terms step runs for pushes to `main` and scans with the
  configured terms. If the secret is unset it prints a notice and passes without scanning, so
  check that the run's log shows no `Owner-terms guard skipped` notice. Generic scanning also
  runs on pushes and pull requests.
- [ ] Protect `main`: require pull requests before merging, require the status checks `CI / go`
  and `CI / python` to pass, and prevent force pushes and branch deletion. Also require
  `CI / macos` once it has run green. Do this after any history rewrite in step 2.
- [ ] Add repository topics that describe the project, such as `job-search`, `terminal`, and
  `privacy`.
- [ ] Confirm a push to `main` runs CI successfully with the Owner-terms secret configured.

## 5. Tag v1.0

After `main` and its CI are verified:

```sh
git tag -a v1.0 -m "rolecall v1.0"
git push origin v1.0
```

- [ ] Verify the `v1.0` tag points to the reviewed release commit.
- [ ] Stop here. Do not close release issues as part of this checklist.
