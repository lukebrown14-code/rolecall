---
status: accepted
---
# The public repo keeps its history

ADR-0005 planned to publish v1.0 as a fresh, history-less repo and keep this one as the private
`rolecall-archive`. By the time that cut was due, this repo had already been public with its full
history for about a week. Hiding it now would not undo that exposure, so we keep
`lukebrown14-code/rolecall` public, history and tracker included, and release v1.0 from it in
place. We chose this over going private and cutting a fresh repo because the cut would protect
history that had already been published, and would cost the project its issues, PRs and links.

## Consequences

- The personal-data guard covers the whole history and the tracker, not just the tree.
  `scripts/check-personal-data.py --history --github lukebrown14-code/rolecall` applies the
  Workspace-structure rules and generic patterns to every file version and commit message on
  any branch, tag or remote ref, plus every issue and PR title, body, comment and review. The
  Owner runs it once with their local terms file before tagging v1.0; CI keeps checking only
  the tree.
- A history finding is fixed in place: rewrite the commits (`git filter-repo`) and force-push.
  GitHub keeps PR refs and cached views of old commits, so removing them fully also needs a
  request to GitHub Support. A tracker finding is fixed by editing the text and deleting the
  old revision from its edit history.
- Issue and PR numbers in docs stay valid, because the tracker they point to is public.
- Commit author names and emails stay as they were. New commits use the Owner's GitHub noreply
  email.
