"""Ad facts shared by the plugin scripts: pay, contract terms, likely duplicates.

Mirrors the Go store (internal/store) so the TUI and the scripts agree:

- ``annual_aud`` turns a pay figure into an annual AUD estimate and flags a figure whose
  unit looks wrong ("$900-$1,100 an hour" is almost certainly a day rate).
- ``title_key`` / ``company_key`` / ``jd_hash`` and ``likely_duplicates`` find a role
  already saved under another URL or a slightly different title.
- ``read_terms`` reads pay, employment type, term, extension and sponsorship from ad text.
  It reports only what the text states; anything unclear stays empty.
"""

import hashlib
import json
import re
from pathlib import Path

ANNUAL = {"hour": 1976, "day": 230, "week": 52, "month": 12, "year": 1}
FX_AUD = {"AUD": 1.0, "USD": 1.55, "GBP": 1.95, "EUR": 1.65, "NZD": 0.92, "CAD": 1.12,
          "SGD": 1.15}
TITLE_NOISE = {"level", "the", "a", "an"}
MIN_JD_FOR_HASH = 200


# ------------------------------------------------------------------------ pay

def fx_table(workspace=None):
    """The AUD rates, with workspace.json ``pay_fx`` overriding or adding codes."""
    table = dict(FX_AUD)
    if workspace is not None:
        try:
            raw = json.loads((Path(workspace) / "workspace.json").read_text()).get("pay_fx")
        except (OSError, ValueError, AttributeError):
            raw = None
        if isinstance(raw, dict):
            for code, rate in raw.items():
                if isinstance(code, str) and isinstance(rate, (int, float)) and not isinstance(rate, bool) and rate > 0:
                    table[code.upper()] = float(rate)
    return table


def mislabel(pay_min, pay_max, unit):
    """Why a figure looks mislabelled, or ''."""
    if pay_min is None:
        return ""
    top = pay_max if pay_max is not None else pay_min
    if unit == "hour" and pay_min >= 300:
        return "looks like a day rate"
    if unit == "day" and pay_min >= 5000:
        return "looks like an annual figure"
    if unit == "year" and top < 1000:
        return "looks like an hourly or daily rate"
    return ""


def annual_aud(pay_min, pay_max, unit, currency, fx=None):
    """(low, high) annual AUD, or None when the unit or currency is unknown."""
    fx = fx or FX_AUD
    currency = (currency or "AUD").upper()  # no currency stated reads as AUD, as in Go
    if pay_min is None or unit not in ANNUAL or currency not in fx:
        return None
    rate = ANNUAL[unit] * fx[currency]
    high = pay_max if pay_max is not None else pay_min
    return pay_min * rate, high * rate


def pay_text(pay_min, pay_max, unit, currency, fx=None):
    """"~A$140k-170k/yr", with any mislabel warning, or ''."""
    est = annual_aud(pay_min, pay_max, unit, currency, fx)
    if est is None:
        return ""
    low, high = (round(v / 1000) for v in est)
    text = f"~A${low}k/yr" if low == high else f"~A${low}k–{high}k/yr"
    why = mislabel(pay_min, pay_max, unit)
    return f"{text} ({why})" if why else text


_MONEY = r"(?:(A|AU|US|NZ|C|S)?\$|(AUD|USD|NZD|GBP|EUR|CAD|SGD)\s*|£|€)\s*([0-9][0-9,]*(?:\.[0-9]+)?)\s*(k)?"
PAY_RE = re.compile(
    _MONEY + r"(?:\s*(?:-|–|—|to)\s*" + r"(?:(?:A|AU|US|NZ|C|S)?\$|£|€)?\s*([0-9][0-9,]*(?:\.[0-9]+)?)\s*(k)?)?"
    r"\s*(?:\+\s*super\s*)?(?:per|an|a|/|p\.?)?\s*(hour|hr|h|day|daily|d|week|wk|month|mth|year|yr|annum|a\.?|annual)?\b",
    re.I,
)
_UNIT = {"hour": "hour", "hr": "hour", "h": "hour", "day": "day", "daily": "day", "d": "day",
         "week": "week", "wk": "week", "month": "month", "mth": "month", "year": "year",
         "yr": "year", "annum": "year", "a": "year", "a.": "year", "annual": "year"}
_PREFIX_CCY = {"A": "AUD", "AU": "AUD", "US": "USD", "NZ": "NZD", "C": "CAD", "S": "SGD"}


def _amount(digits, thousands):
    value = float(digits.replace(",", ""))
    return value * 1000 if thousands else value


def read_pay(text, default_currency="AUD"):
    """{pay_min, pay_max, pay_unit, currency} from the first pay figure in text, or {}."""
    for m in PAY_RE.finditer(text or ""):
        prefix, code, low, low_k, high, high_k, unit = m.groups()
        pay_min = _amount(low, low_k)
        pay_max = _amount(high, high_k or low_k) if high else None
        if pay_min < 10:  # "$5 off", list numbering: not pay
            continue
        currency = (code or "").upper() or _PREFIX_CCY.get((prefix or "").upper())
        if not currency:
            currency = {"£": "GBP", "€": "EUR"}.get(m.group(0).strip()[:1], default_currency)
        unit = _UNIT.get((unit or "").lower().rstrip("."), "") or ("year" if pay_min >= 20000 else "")
        if not unit:
            continue
        out = {"pay_min": _number(pay_min), "pay_unit": unit, "currency": currency}
        if pay_max is not None and pay_max != pay_min:
            out["pay_max"] = _number(pay_max)
        return out
    return {}


def _number(value):
    return int(value) if float(value).is_integer() else round(value, 2)


# ---------------------------------------------------------------------- terms

TERM_RE = re.compile(r"\b(\d{1,2})[- ]?(?:\+\s*)?(month|mth|week)s?\b(?:[^.\n]{0,25}?)\b(contract|fixed[- ]term|term|engagement|assignment)", re.I)
EXTENSION_RE = re.compile(r"\b(possible|potential|likely|option(?:al)?|view) (?:of |to |for )?(?:an )?extension|extension (?:possible|likely|available)", re.I)
NO_EXTENSION_RE = re.compile(r"\bno (?:possibility of )?extension\b", re.I)
SPONSOR_NO_RE = re.compile(r"\b(no|not|unable to|cannot|can't|won't|will not)\b[^.\n]{0,30}\bsponsor", re.I)
SPONSOR_YES_RE = re.compile(r"\b(visa sponsorship (?:is )?(?:available|offered|provided)|we (?:can|will) sponsor|sponsorship available)\b", re.I)


def read_terms(text):
    """Employment type, term_months, extension and sponsorship the ad states, as strings."""
    text = text or ""
    out = {}
    m = TERM_RE.search(text)
    if m:
        n = int(m.group(1))
        months = max(1, round(n / 4.33)) if m.group(2).lower().startswith("week") else n
        out["term_months"] = str(months)
        out["employment_type"] = "fixed-term" if "fixed" in m.group(3).lower() else "contract"
    elif re.search(r"\bcontract(?:or)?\b", text, re.I) and not re.search(r"\bpermanent\b", text, re.I):
        out["employment_type"] = "contract"
    elif re.search(r"\bpermanent\b", text, re.I):
        out["employment_type"] = "permanent"
    elif re.search(r"\bcasual\b", text, re.I):
        out["employment_type"] = "casual"
    if NO_EXTENSION_RE.search(text):
        out["extension"] = "no"
    elif EXTENSION_RE.search(text):
        out["extension"] = "possible"
    if SPONSOR_NO_RE.search(text):
        out["sponsorship"] = "not-offered"
    elif SPONSOR_YES_RE.search(text):
        out["sponsorship"] = "offered"
    return out


# ----------------------------------------------------------------- duplicates

def title_key(title):
    """A title with case, punctuation, a trailing (…) and filler words removed."""
    text = re.sub(r"\s*\([^)]*\)\s*$", "", title or "").lower()
    words = re.sub(r"[^a-z0-9]+", " ", text).split()
    return " ".join(w for w in words if w not in TITLE_NOISE)


def company_key(company):
    text = re.sub(r"\b(pty|ltd|limited|inc|group|australia)\b", " ", (company or "").lower())
    return " ".join(re.sub(r"[^a-z0-9]+", " ", text).split())


def jd_hash(text):
    """sha256 of the ad text, lower-cased with whitespace collapsed; '' when too short."""
    norm = " ".join((text or "").lower().split())
    if len(norm) < MIN_JD_FOR_HASH:
        return ""
    return hashlib.sha256(norm.encode("utf-8")).hexdigest()


def _employers(app):
    keys = {company_key(app.company)}
    if getattr(app, "recruiter", ""):
        keys.add(company_key(app.recruiter))
    return keys - {""}


def likely_duplicates(apps, company, title, jd_text="", recruiter=""):
    """Saved Applications that are probably the same role, with why each matched."""
    wanted_title = title_key(title)
    wanted = {company_key(company), company_key(recruiter)} - {""}
    digest = jd_hash(jd_text)
    out = []
    for app in apps:
        if getattr(app, "malformed", None):
            continue
        reasons = []
        if wanted_title and title_key(app.title) == wanted_title and wanted & _employers(app):
            reasons.append("same employer and title")
        if digest:
            try:
                theirs = (Path(app.folder) / "jd.md").read_text()
            except OSError:
                theirs = ""
            if jd_hash(theirs) == digest:
                reasons.append("same ad text")
        if reasons:
            out.append((app, ", ".join(reasons)))
    return out
