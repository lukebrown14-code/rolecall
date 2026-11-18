"""Follow-up due, as the TUI's flag reckons it (SCHEMA status_log; Go's reporting).

The undo rule (``effective_log``), the day a follow-up counts from, and whether an
Application is due, held by a next step, or not yet due.
"""

from datetime import date, datetime, timedelta

from metayaml import RFC3339, parse_moment, valid_date

# The TUI's default follow_up_days. The setting lives in the TUI's config.toml, which the
# plugin does not read, so a caller passes the Owner's value when they changed it.
DEFAULT_FOLLOW_UP_DAYS = 14
FOLLOW_UP_STATUSES = ("Applied", "Interviewing")


def _day(value):
    """The day a date counts on, as SCHEMA and Go's store.LocalDay read it; or None.

    An RFC 3339 timestamp (a ``status_log`` ``at``) counts on its date in the Owner's
    local zone; a YYYY-MM-DD date, or a zoneless ``next_step`` time, keeps the day written.
    """
    text = str(value or "").strip()
    if RFC3339.fullmatch(text):
        try:
            return parse_moment(text).astimezone().date()
        except ValueError:
            pass
    text = text.split("T")[0]
    return datetime.strptime(text, "%Y-%m-%d").date() if valid_date(text) else None


# How soon after a move an entry restoring the Status before it reads as an undo of that
# move (SCHEMA status_log; reporting.UndoWindow in Go).
UNDO_WINDOW = timedelta(minutes=15)


def _undoes(move, entry):
    """Whether entry undoes move: both timestamped, entry no more than UNDO_WINDOW after."""
    if not (RFC3339.fullmatch(move.at) and RFC3339.fullmatch(entry.at)):
        return False
    gap = parse_moment(entry.at) - parse_moment(move.at)
    return timedelta(0) <= gap <= UNDO_WINDOW


def effective_log(log):
    """A status_log with each undone move taken out, as Go's reporting.EffectiveLog.

    An entry that restores the Status before the previous entry, within UNDO_WINDOW of it,
    is an undo: it and the move it undid are both dropped. Undoing a first move back to
    Lead empties the log. Date-only entries never read as an undo.
    """
    out = []
    for entry in log:
        if len(out) >= 2 and out[-2].status == entry.status and _undoes(out[-1], entry):
            out.pop()
        elif (
            len(out) == 1
            and entry.status == "Lead"
            and out[0].status != "Lead"
            and _undoes(out[0], entry)
        ):
            out.clear()
        else:
            out.append(entry)
    return out


def _last_change(app):
    """When the Application reached its current Status, ignoring undone moves, as Go's
    schedule.LastChange: the effective log's last entry, None when an undo emptied a Lead's log, and
    the raw last entry if the effective log disagrees with the current Status."""
    log = effective_log(app.status_log)
    if not log and app.status == "Lead":
        return None
    if not log or log[-1].status != app.status:
        return app.last_change
    return log[-1]


def follow_up_from(app):
    """The day Follow-up due counts from, and what it is, mirroring the Go TUI.

    It is the later of the last Status change, ignoring undone moves, and ``followed_up``
    (a follow-up the Owner recorded with ``l``); on the same day the Status change wins.
    Returns ``(day, "followed_up" | "last change")``, or ``(None, "")`` when neither reads.
    """
    last = _last_change(app)
    changed = _day(last.at) if last else None
    followed = _day(app.followed_up)
    if followed is not None and (changed is None or followed > changed):
        return followed, "followed_up"
    if changed is not None:
        return changed, "last change"
    return None, ""


def follow_up_status(app, today=None, days=DEFAULT_FOLLOW_UP_DAYS):
    """Follow-up due for one Application, as the TUI's ⚑ flag reckons it.

    Only Applied and Interviewing can be due. The Application is due ``days`` after
    ``follow_up_from``; ``DaysPast`` is whole days past that (0 on the day, negative
    before it). A ``next_step`` on or after today holds it: ``HeldBy`` is that step's
    ``at``, and ``DueOn`` and ``DaysPast`` stay unset.
    """
    today = today or date.today()
    start, since = follow_up_from(app)
    step = _day(app.next_step["At"]) if app.next_step else None
    result = {
        "Status": app.status,
        "FollowedUp": app.followed_up,
        "From": start.isoformat() if start else "",
        "Since": since,
        "Days": days,
        "HeldBy": "",
        "DueOn": "",
        "DaysPast": None,
        "Due": False,
    }
    if app.malformed or app.status not in FOLLOW_UP_STATUSES or start is None:
        return result
    if step is not None and step >= today:
        result["HeldBy"] = app.next_step["At"]
        return result
    due_on = start + timedelta(days=days)
    result["DueOn"] = due_on.isoformat()
    result["DaysPast"] = (today - due_on).days
    result["Due"] = result["DaysPast"] >= 0
    return result
