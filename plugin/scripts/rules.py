#!/usr/bin/env python3
"""Search rules: the Owner's settings that rule a role out, applied the same way everywhere.

    python3 rules.py --data <ws> show                      # the effective rules as JSON
    python3 rules.py --data <ws> label "1mo ago"           # how a posting-age label reads
    python3 rules.py --data <ws> check                     # dry run over saved Applications
    python3 rules.py --data <ws> check --apply             # dismiss the certain Leads
    python3 rules.py --data <ws> check --apply --include-drafts   # Drafts too

The rules live in workspace.json ``search_rules`` (SCHEMA.md), with per-source caps in the
``sources`` registry. The Sweep drops new roles with them, ``meta.py create`` refuses a Lead
that breaks one, and ``check`` lists the saved Applications a rule would remove before
anything is written. ``--apply`` appends ``Dismissed`` with a ``filter_reason`` and a
``filter_note``; folders never move. An Application the Owner restored after a rule
dismissed it carries ``filter_overruled: true`` and is never dismissed by a rule again.
"""

import argparse
import json
import re
import sys
from dataclasses import dataclass, field
from datetime import date, datetime, timezone
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from workspace import add_data_argument, load, workspace_from_cli  # noqa: E402

# Days an "N months ago" label can mean: sites round anything from four weeks to just
# under the next month, so "1mo ago" is 28 to 61 days.
MONTH_MIN, MONTH_SPAN = 28, 33
DEFAULT_EXPIRY_WARNING_DAYS = 5

EXPIRED, BLOCKER, APPLICANT_CAP, LOCATION, CONTRACT_LENGTH = (
    "expired", "blocker", "applicant_cap", "location", "contract_length",
)
REASONS = (EXPIRED, BLOCKER, APPLICANT_CAP, LOCATION, CONTRACT_LENGTH)


class RulesError(ValueError):
    """workspace.json search_rules holds a value of the wrong shape."""


@dataclass
class Rules:
    max_posting_age_days: int | None = None
    expiry_warning_days: int = DEFAULT_EXPIRY_WARNING_DAYS
    ambiguous_age_labels: dict = field(default_factory=dict)  # normalised label -> discard|keep
    max_applicants: dict = field(default_factory=dict)  # source name (lower) -> cap
    max_contract_months: int | None = None
    source_age: dict = field(default_factory=dict)  # source name (lower) -> its own age limit

    def age_limit(self, source=""):
        return self.source_age.get(source_key(source), self.max_posting_age_days)

    def applicant_cap(self, source=""):
        return self.max_applicants.get(source_key(source))

    def to_dict(self):
        return {
            "max_posting_age_days": self.max_posting_age_days,
            "expiry_warning_days": self.expiry_warning_days,
            "ambiguous_age_labels": self.ambiguous_age_labels,
            "max_applicants": self.max_applicants,
            "max_contract_months": self.max_contract_months,
            "source_age": self.source_age,
        }


def source_key(source):
    """The registry name a source string refers to: 'greenhouse:canva' -> 'greenhouse'."""
    return (source or "").split(":", 1)[0].strip().lower()


def normalise_label(label):
    return re.sub(r"\s+", " ", (label or "").strip().lower())


def _whole(value, what, minimum=0):
    if isinstance(value, bool) or not isinstance(value, int) or value < minimum:
        raise RulesError(f"search_rules.{what} wants a whole number of at least {minimum}")
    return value


def rules_from(raw, sources=()):
    """Rules from the search_rules object and the source registry's per-source rules."""
    rules = Rules()
    if raw is None:
        raw = {}
    if not isinstance(raw, dict):
        raise RulesError("search_rules wants an object")
    if raw.get("max_posting_age_days") is not None:
        rules.max_posting_age_days = _whole(raw["max_posting_age_days"], "max_posting_age_days", 1)
    if raw.get("expiry_warning_days") is not None:
        rules.expiry_warning_days = _whole(raw["expiry_warning_days"], "expiry_warning_days")
    if raw.get("max_contract_months") is not None:
        rules.max_contract_months = _whole(raw["max_contract_months"], "max_contract_months", 1)
    labels = raw.get("ambiguous_age_labels") or {}
    if not isinstance(labels, dict) or any(v not in ("discard", "keep") for v in labels.values()):
        raise RulesError('search_rules.ambiguous_age_labels maps a label to "discard" or "keep"')
    rules.ambiguous_age_labels = {normalise_label(k): v for k, v in labels.items()}
    caps = raw.get("max_applicants") or {}
    if not isinstance(caps, dict):
        raise RulesError("search_rules.max_applicants maps a source name to a cap")
    rules.max_applicants = {source_key(k): _whole(v, f"max_applicants.{k}") for k, v in caps.items()}
    for entry in sources:
        own = entry.get("rules") if isinstance(entry, dict) else None
        if not isinstance(own, dict):
            continue
        name = source_key(entry.get("name", ""))
        if own.get("max_applicants") is not None:
            rules.max_applicants[name] = _whole(own["max_applicants"], f"{name}.rules.max_applicants")
        if own.get("max_posting_age_days") is not None:
            rules.source_age[name] = _whole(own["max_posting_age_days"], f"{name}.rules.max_posting_age_days", 1)
    return rules


def load_rules(workspace):
    config = load(workspace)
    return rules_from(config.raw.get("search_rules"), config.source_registry)


# ------------------------------------------------------------------ posting age

@dataclass
class Age:
    """How old a posting is: exactly (min == max), at least min (max None), or a range."""
    min_days: int
    max_days: int | None
    label: str = ""

    @property
    def exact(self):
        return self.max_days == self.min_days

    def text(self):
        if self.exact:
            return f"{self.min_days} days old"
        if self.max_days is None:
            return f"{self.min_days}+ days old"
        return f"{self.min_days}-{self.max_days} days old"


_UNITS = [
    (r"(?:h|hr|hrs|hour|hours|m|min|mins|minute|minutes)", 0, 0),
    (r"(?:d|day|days)", 1, 0),
    (r"(?:w|wk|wks|week|weeks)", 7, 6),
    (r"(?:mo|mos|mth|mths|month|months)", MONTH_MIN, MONTH_SPAN),
    (r"(?:y|yr|yrs|year|years)", 365, 364),
]


def parse_age_label(label):
    """An Age from a site's label ("14d ago", "Posted 30+ Days Ago", "1mo ago"), or None."""
    text = normalise_label(label)
    if not text:
        return None
    if re.search(r"\b(just posted|today|new|moments? ago|active today)\b", text):
        return Age(0, 0, label)
    if "yesterday" in text:
        return Age(1, 1, label)
    m = re.search(r"(\d+)\s*(\+)?\s*([a-z]+)", text)
    if not m:
        return None
    n, plus, unit = int(m.group(1)), bool(m.group(2)), m.group(3)
    for pattern, per, span in _UNITS:
        if re.fullmatch(pattern, unit):
            low = n * per
            high = None if plus else low + (span if per > 1 else 0)
            if per == MONTH_MIN and not plus:
                high = n * MONTH_MIN + MONTH_SPAN
            return Age(low, high, label)
    return None


def age_of(posted="", posted_label="", today=None):
    """The best Age known: the posted date wins over the label."""
    today = today or date.today()
    if posted:
        try:
            days = (today - datetime.strptime(posted, "%Y-%m-%d").date()).days
        except ValueError:
            days = None
        if days is not None:
            return Age(max(days, 0), max(days, 0), posted_label)
    return parse_age_label(posted_label)


@dataclass
class Finding:
    """One rule a role breaks. certain is False for a borderline age the Owner must judge."""
    reason: str
    note: str
    certain: bool = True
    advisory: bool = False  # flagged, never dismissed (a preference, not a rule)


def check_age(age, rules, source=""):
    """A Finding when the posting is past the age limit (age >= limit), else None.

    A label that may or may not be past it (a range across the limit) is borderline:
    ``ambiguous_age_labels`` decides it, else it is returned uncertain for the Owner."""
    limit = rules.age_limit(source)
    if limit is None or age is None:
        return None
    if age.min_days >= limit:
        return Finding(EXPIRED, f"{age.text()}{_label_tail(age)}; limit {limit} days")
    if age.max_days is not None and age.max_days < limit:
        return None
    decision = rules.ambiguous_age_labels.get(normalise_label(age.label))
    if decision == "keep":
        return None
    note = f"{age.text()}{_label_tail(age)}, borderline; limit {limit} days"
    return Finding(EXPIRED, note, certain=decision == "discard")


def _label_tail(age):
    return f' ("{age.label.strip()}")' if age.label.strip() else ""


CONTRACT_RE = re.compile(r"(\d{1,2})\s*(?:-|to)?\s*(?:\d{1,2}\s*)?[- ]?months?\b[^.\n]{0,20}?\b(contract|fixed[- ]term)", re.I)


def contract_months(text):
    m = CONTRACT_RE.search(text or "")
    return int(m.group(1)) if m else None


def evaluate(rules, *, source="", posted="", posted_label="", applicants=None, text="",
             blockers=None, today=None):
    """Every rule a role breaks, in the order the report lists them."""
    findings = []
    age = check_age(age_of(posted, posted_label, today), rules, source)
    if age:
        findings.append(age)
    cap = rules.applicant_cap(source)
    if cap is not None and applicants is not None and applicants > cap:
        findings.append(Finding(APPLICANT_CAP, f"{applicants} applicants; {source_key(source)} cap {cap}"))
    if blockers and text:
        from signals import blocker_hits  # noqa: PLC0415 -- only check needs the patterns
        hits = blocker_hits(text, blockers)
        if hits:
            findings.append(Finding(BLOCKER, "; ".join(hits)))
    months = contract_months(text)
    if rules.max_contract_months and months and months > rules.max_contract_months:
        findings.append(Finding(CONTRACT_LENGTH, f"{months}-month contract; preferred at most "
                                f"{rules.max_contract_months}", advisory=True))
    return findings


def deciding(findings):
    """The Finding that dismisses the role (first certain, non-advisory), or None."""
    return next((f for f in findings if f.certain and not f.advisory), None)


# ------------------------------------------------------------------------ check

def _plan(workspace, rules, today=None):
    import meta as meta_store  # noqa: PLC0415 -- meta imports this module lazily too
    from signals import blockers_for  # noqa: PLC0415
    blockers = blockers_for(load(workspace).blockers)
    plan = []
    for app in meta_store.scan(workspace):
        if app.malformed or app.status not in ("Lead", "Draft") or app.filter_overruled:
            continue
        try:
            text = (Path(app.folder) / "jd.md").read_text()
        except OSError:
            text = ""
        findings = [f for f in evaluate(rules, source=app.source, posted=app.posted,
                                        posted_label=app.posted_label, applicants=app.applicants,
                                        text=text, blockers=blockers, today=today)
                    if not f.advisory]
        if findings:
            plan.append((app, findings))
    return plan


def _describe(app, findings):
    reasons = "; ".join(f"{f.reason}: {f.note}" for f in findings)
    docs = f", {app.documents.lower()} documents" if app.documents else ""
    return f"- {app.company} — {app.title} ({app.status}{docs}) — {reasons}\n  {app.folder}"


def check(workspace, apply=False, include_drafts=False, today=None, out=sys.stdout):
    """List what the rules would dismiss; with apply, dismiss it. Returns the count written."""
    import meta as meta_store  # noqa: PLC0415
    rules = load_rules(workspace)
    plan = _plan(workspace, rules, today)
    leads = [(a, f) for a, f in plan if a.status == "Lead" and deciding(f)]
    borderline = [(a, f) for a, f in plan if not deciding(f)]
    drafts = [(a, f) for a, f in plan if a.status == "Draft" and deciding(f)]
    if not plan:
        print("No saved Lead or Draft breaks a Search rule.", file=out)
        return 0
    sections = [
        (f"Would dismiss {len(leads)} Lead(s)", leads),
        (f"Borderline, kept until you decide ({len(borderline)}) — add the label to "
         "search_rules.ambiguous_age_labels as discard or keep", borderline),
        (f"Drafts a rule would remove ({len(drafts)}) — dismissed only with --include-drafts",
         drafts),
    ]
    for head, rows in sections:
        if rows:
            print(f"## {head}", file=out)
            for app, findings in rows:
                print(_describe(app, findings), file=out)
            print(file=out)
    if not apply:
        print("Dry run: nothing written. Re-run with --apply once the Owner confirms.", file=out)
        return 0
    targets = leads + (drafts if include_drafts else [])
    written = 0
    for app, findings in targets:
        f = deciding(findings)
        if meta_store.dismiss_by_rule(app.folder, f.reason, f.note):
            written += 1
    print(f"Dismissed {written} Application(s) by rule. They show under Leads → Filtered, "
          "where the Owner can restore any of them.", file=out)
    return written


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    add_data_argument(parser)
    commands = parser.add_subparsers(dest="command", required=True)
    commands.add_parser("show", help="print the effective rules as JSON")
    label = commands.add_parser("label", help="read a posting-age label")
    label.add_argument("text")
    label.add_argument("--source", default="")
    run = commands.add_parser("check", help="dry run the rules over saved Applications")
    run.add_argument("--apply", action="store_true", help="dismiss the certain Leads")
    run.add_argument("--include-drafts", action="store_true",
                     help="with --apply, dismiss Drafts a rule removes too")
    run.add_argument("--today", help="YYYY-MM-DD (tests)")
    args = parser.parse_args(argv)
    workspace = workspace_from_cli(args.data)
    try:
        if args.command == "show":
            print(json.dumps(load_rules(workspace).to_dict(), indent=2))
        elif args.command == "label":
            rules = load_rules(workspace)
            age = parse_age_label(args.text)
            finding = check_age(age, rules, args.source)
            print(json.dumps({
                "label": args.text,
                "min_days": age.min_days if age else None,
                "max_days": age.max_days if age else None,
                "verdict": ("unknown" if age is None else "keep" if finding is None
                            else "discard" if finding.certain else "borderline"),
                "note": finding.note if finding else "",
            }))
        else:
            today = (datetime.strptime(args.today, "%Y-%m-%d").date() if args.today
                     else datetime.now(timezone.utc).date())
            check(workspace, args.apply, args.include_drafts, today)
    except RulesError as error:
        print(f"rolecall: {error}", file=sys.stderr)
        raise SystemExit(1) from error


if __name__ == "__main__":
    main()
