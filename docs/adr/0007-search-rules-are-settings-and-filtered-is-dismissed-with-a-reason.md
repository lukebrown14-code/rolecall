---
status: accepted
---
# Search rules are settings, and Filtered is Dismissed with a reason

Rules that rule a role out without judgement ("discard roles posted 30 or more days ago",
"LinkedIn: 100 applicants or fewer") used to be bullet points in `writing-preferences.md`.
Nothing enforced them, so the Harness had to remember each one, and it invented a
`_discarded/` folder and a reason field when it applied them. They are now structured
settings in `workspace.json` `search_rules`, and Sources of every kind share one registry in
`workspace.json` `sources`. The Sweep, the Lead writer (`meta.py create`, which refuses a
Lead that breaks a rule) and a `rules.py check`
over saved Applications all apply the same rules.

A rule-dismissed Application is not a new Status. It is `Dismissed` with `filter_reason` and
`filter_note` in its `meta.yml`, so Status keeps its eight values (ADR-0002) and folders never
move. The TUI lists these apart from the Owner's own dismissals as **Filtered**.

## Consequences

- A rule never removes anything unseen: `rules.py check` is a dry run by default and lists
  Leads, borderline labels and Drafts separately. `--apply` dismisses only certain Leads;
  Drafts need `--include-drafts`. The find-jobs skill asks the Owner before applying.
- A borderline age (a label such as "1mo ago" that spans the limit) is never decided by the
  Harness. `ambiguous_age_labels` decides it, or the Owner does.
- The Harness may append a `Dismissed` entry only through `rules.py check --apply`, after
  the Owner confirms the dry run (SCHEMA.md, "Who may change what").
- Restoring a Filtered Application writes `filter_overruled: true`, and no rule dismisses
  it again. The Owner's decision wins over a rule.
- The new `meta.yml` keys are optional and readers ignore keys they do not know, so the
  schema stays 1. The older `sources` object, `additional_sources` and
  `manual_search_sources` keep working.
- `writing-preferences.md` holds writing style only.
