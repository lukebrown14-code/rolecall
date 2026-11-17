# Public repository cut checklist

Follow this once, after the v1.0 implementation is complete. This cut publishes the current
tree without its history; it does not make the private archive public. Keep the archive private
throughout.

## 1. Confirm release blockers

- [ ] Fetch and merge the latest integration branch containing this checklist.
- [ ] Review the v1.0 release issue and its linked work. Confirm that every other v1.0 blocker
  has been merged into the release tree. The public-repo cut itself is performed only after this
  checklist is merged; do not treat an open blocker as complete.
- [ ] Check the release tracker and each blocker in GitHub. Confirm every blocker other than the
  cut task is closed and its work is present in the release commit; check the release tree's
  history when the issue state alone does not show that the change merged.
- [ ] Confirm the release tree is the intended v1.0 source and has no uncommitted changes.
- [ ] Stop if any blocker remains open or any required work has not merged.

## 2. Strip private issue and pull request references

- [ ] Search all documentation and ADRs for references to private issue numbers, pull request
  numbers, and private-repository URLs. Remove or rewrite each reference so published docs do
  not link back to the private archive. Preserve useful product requirements without tracker
  links.
- [ ] Review the staged diff and confirm the documentation cleanup is complete.

## 3. Run the personal-data scanner with Owner terms

The scanner examines tracked text files in the repository. Put one Owner-specific term per
line in the gitignored `.rolecall-personal-data-terms` file at the repository root. Include the
Owner's name and known identifying terms such as former employers, locations, and visa terms;
do not add those terms to repository files.

- [ ] Confirm `.rolecall-personal-data-terms` contains the complete Owner-specific term list and
  is not tracked (`git check-ignore .rolecall-personal-data-terms`).
- [ ] From the repository root, run:

  ```sh
  python3 scripts/check-personal-data.py .
  ```

  The scanner reads the default terms file automatically. To store the terms file elsewhere,
  set `ROLECALL_PERSONAL_DATA_TERMS_FILE=/absolute/path/to/terms` for this command. Alternatively,
  provide newline-separated terms with `ROLECALL_PERSONAL_DATA_TERMS`; the scanner adds these to
  the terms in the file. The command must exit zero with no findings. If it reports a
  finding, fix the source and rerun; do not add Owner terms to the allowlist.
- [ ] Run the scanner again after any cleanup and confirm the final tracked tree passes.

## 4. Read the publishable content by hand

- [ ] Read every file under `docs/`, including all ADRs.
- [ ] Read `README.md` and check its privacy statements and setup instructions.
- [ ] Read `.github/ISSUE_TEMPLATE/bug_report.md` and confirm it warns reporters not to paste
  `meta.yml`, a CV, or the Profile, and asks them to reproduce with the example Workspace.
- [ ] Read every file under `fixtures/`, including the example Workspace and generated-document
  fixtures. Confirm names and personal facts are fictional and no real Owner data remains.
- [ ] Resolve any concern and rerun the scanner after edits.

## 5. Rename this repository and keep it private

- [ ] In GitHub repository settings, rename the current private repository to
  `lukebrown14-code/rolecall-archive`.
- [ ] Verify its visibility is **Private** after the rename. Do not change its visibility or
  delete its history.
- [ ] Confirm the `rolecall` repository name is now available under `lukebrown14-code`. v1 stays
  under personal ownership: no organization transfer, and the Go module path and Homebrew tap
  (`lukebrown14-code/homebrew-tap`) are unchanged.

## 6. Create a fresh public repository with one squashed commit

Use a clean export of the reviewed release tree so no old Git history, branches, or tags enter
the public repository. For example, from a local clone at the reviewed release commit:

```sh
mkdir -p /tmp/rolecall-public
git archive <release-commit> | tar -x -C /tmp/rolecall-public
cd /tmp/rolecall-public
git init -b main
git add -A
git commit -m "Release rolecall v1.0"
git remote add origin <new-public-rolecall-url>
git push -u origin main
```

- [ ] Create the new GitHub repository named `rolecall` as **Public**, without initializing it
  with a README, license, or other files.
- [ ] Substitute the reviewed commit and the new repository URL above, then create and push the
  single initial commit.
- [ ] Verify the public repository has exactly one commit on `main`, contains the reviewed tree,
  and has no branches or tags copied from the private archive.
- [ ] Confirm the Owner-terms file was not included in the export.

## 7. Configure CI and repository settings

- [ ] In the new repository's **Settings → Secrets and variables → Actions**, create the
  repository secret `ROLECALL_PERSONAL_DATA_TERMS`. Set its value to the Owner-specific terms,
  one per line. Keep this value out of commits, issues, and logs.
- [ ] Confirm the workflow's Owner-terms step runs for pushes to `main` of the public repository
  and scans with the configured terms. If the secret is unset it prints a notice and passes
  without scanning, so check that the run's log shows no `Owner-terms guard skipped` notice.
  Generic scanning also runs on pushes and pull requests.
- [ ] Protect `main`: require pull requests before merging, require the status checks `CI / go`
  and `CI / python` to pass, and prevent force pushes and branch deletion. Also require
  `CI / macos` once it has run green on the public repository.
- [ ] Add repository topics that describe the project, such as `job-search`, `terminal`, and
  `privacy`.
- [ ] Confirm a push to `main` runs CI successfully with the Owner-terms secret configured.

## 8. Tag v1.0

After the public `main` branch and its CI are verified:

```sh
git tag -a v1.0 -m "rolecall v1.0"
git push origin v1.0
```

- [ ] Verify the `v1.0` tag points to the reviewed single-commit public release tree.
- [ ] Stop here. Do not close release issues as part of this checklist.
