"""The meta.yml line grammar SCHEMA.md allows, mirroring the Go store.

No YAML library: plain and double-quoted scalars, one-line flow maps, whole
numbers, dates and RFC 3339 timestamps, and ``MetaError`` for anything else.
``metaread`` builds an Application from these; ``metawrite`` checks with them.
"""

import re
from datetime import datetime, timedelta, timezone

KEY = re.compile(r"^[A-Za-z_][A-Za-z0-9_-]*$")

DATE = re.compile(r"^\d{4}-\d{2}-\d{2}$")
RFC3339 = re.compile(
    r"^(\d{4})-(\d{2})-(\d{2})T(\d{1,2}):(\d{2}):(\d{2})(?:\.\d+)?"
    r"(Z|([+-])(\d{2}):(\d{2}))$"
)

# The largest whole number SCHEMA allows: a signed 64-bit integer, as the Go store reads it.
MAX_WHOLE = 2**63 - 1


def whole(value):
    """SCHEMA's whole number: ASCII digits only, no sign, within 64 bits; else None."""
    if not re.fullmatch(r"[0-9]+", value) or int(value) > MAX_WHOLE:
        return None
    return int(value)


def blank(text):
    """SCHEMA's blank: empty or only spaces and tabs (str.strip() also eats \v and U+001C)."""
    return not text.strip(" \t")


ESCAPES = {
    "0": "\x00",
    "a": "\x07",
    "b": "\x08",
    "t": "\x09",
    "n": "\x0a",
    "v": "\x0b",
    "f": "\x0c",
    "r": "\x0d",
    "e": "\x1b",
    " ": " ",
    '"': '"',
    "/": "/",
    "\\": "\\",
    "N": "\x85",
    "_": "\xa0",
    "L": "\u2028",
    "P": "\u2029",
}


class MetaError(Exception):
    """A meta.yml line that breaks the SCHEMA.md format."""


def scalar_parts(raw):
    """Split a scalar into its token and exact whitespace/comment suffix."""
    start = len(raw) - len(raw.lstrip(" \t"))
    end = len(raw)
    if start < len(raw) and raw[start] == '"':
        end = -1
        i = start + 1
        while i < len(raw):
            if raw[i] == "\\":
                i += 2
                continue
            if raw[i] == '"':
                end = i + 1
                break
            i += 1
        if end < 0:
            raise MetaError("invalid quoted scalar")
        tail = raw[end:].lstrip(" \t")
        if tail and not tail.startswith("#"):
            raise MetaError("invalid quoted scalar suffix")
    else:
        i = start
        while i < len(raw):
            if raw[i] == "#" and (i == start or raw[i - 1] in " \t"):
                end = i
                break
            i += 1
        end = len(raw[:end].rstrip(" \t"))
    if end < start:
        end = start
    return raw[start:end], raw[end:]


def _scalar_token(raw):
    return scalar_parts(raw)[0]


def decode_quoted(token):
    out = []
    i = 1
    while i < len(token) - 1:
        if token[i] != "\\":
            out.append(token[i])
            i += 1
            continue
        i += 1
        if i >= len(token) - 1:
            raise MetaError("short YAML escape")
        c = token[i]
        if c in ESCAPES:
            out.append(ESCAPES[c])
            i += 1
            continue
        size = {"x": 2, "u": 4, "U": 8}.get(c)
        if size is None:
            raise MetaError("invalid YAML escape")
        if i + size >= len(token) - 1:
            raise MetaError("short YAML escape")
        digits = token[i + 1 : i + 1 + size]
        if not re.fullmatch(r"[0-9A-Fa-f]+", digits):
            raise MetaError("invalid Unicode escape")
        value = int(digits, 16)
        if value > 0x10FFFF or 0xD800 <= value <= 0xDFFF:
            raise MetaError("invalid Unicode escape")
        out.append(chr(value))
        i += size + 1
    return "".join(out)


def scalar(raw):
    token = _scalar_token(raw)
    if token.startswith('"'):
        return decode_quoted(token)
    if (
        "\t" in token
        or "\r" in token
        or token.startswith(("{", "[", "'"))
        or token in ("|", ">")
        or ": " in token
    ):
        raise MetaError("invalid scalar")
    return token


def valid_date(value):
    if not DATE.fullmatch(value):
        return False
    try:
        datetime.strptime(value, "%Y-%m-%d")
    except ValueError:
        return False
    return True


def valid_timestamp(value):
    if DATE.fullmatch(value):
        try:
            datetime.strptime(value, "%Y-%m-%d")
        except ValueError:
            return False
        return True
    match = RFC3339.fullmatch(value)
    if match is None:
        return False
    year, month, day, hour, minute, second = (
        int(match.group(index)) for index in range(1, 7)
    )
    try:
        datetime(year, month, day)
    except ValueError:
        return False
    if hour > 23 or minute > 59 or second > 59:
        return False
    if match.group(8):
        offset_hour, offset_minute = int(match.group(9)), int(match.group(10))
        if offset_hour > 23 or offset_minute > 59:
            return False
    return True


NEXT_STEP_AT = re.compile(r"^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2})?$")


def flow_map(text):
    """Read a one-line flow map ``{key: value, key: "value"}`` as the Go store does.

    Returns ``(pairs, rest)``: ``rest`` is whatever follows the closing brace, which may
    only be an inline comment. Raises MetaError for anything else.
    """
    text = text.lstrip(" \t")
    if not text.startswith("{"):
        raise MetaError("invalid flow map: expected {")
    pairs = {}
    i = 1
    n = len(text)

    def skip(i):
        while i < n and text[i] in " \t":
            i += 1
        return i

    while True:
        i = skip(i)
        if i >= n:
            raise MetaError("invalid flow map: missing }")
        if text[i] == "}":
            i += 1
            break
        start = i
        while i < n and text[i] not in ":,}":
            i += 1
        key = text[start:i].strip()
        if i >= n or text[i] != ":" or not KEY.fullmatch(key):
            raise MetaError("invalid flow map: expected key:")
        if key in pairs:
            raise MetaError("invalid flow map: duplicate key " + key)
        i = skip(i + 1)
        if i < n and text[i] == '"':
            end = -1
            j = i + 1
            while j < n:
                if text[j] == "\\":
                    j += 2
                    continue
                if text[j] == '"':
                    end = j + 1
                    break
                j += 1
            if end < 0:
                raise MetaError("invalid flow map: unterminated string")
            value = decode_quoted(text[i:end])
            i = end
        else:
            start = i
            while i < n and text[i] not in ",}":
                i += 1
            value = text[start:i].strip()
            if any(c in value for c in '{["\'\t') or " #" in value:
                raise MetaError("invalid flow map: unquoted value")
        pairs[key] = value
        i = skip(i)
        if i < n and text[i] == ",":
            i += 1
            continue
        if i >= n or text[i] != "}":
            raise MetaError("invalid flow map: expected , or }")
    rest = text[i:]
    tail = rest.lstrip(" \t")
    if tail and not tail.startswith("#"):
        raise MetaError("invalid flow map: text after }")
    return pairs, rest


def valid_next_step_at(value):
    if not NEXT_STEP_AT.fullmatch(value):
        return False
    try:
        datetime.strptime(value, "%Y-%m-%dT%H:%M" if "T" in value else "%Y-%m-%d")
    except ValueError:
        return False
    return True


def parse_moment(value):
    """An aware datetime from a datetime, YYYY-MM-DD or RFC 3339 string."""
    if isinstance(value, datetime):
        return value
    text = str(value).strip()
    if DATE.fullmatch(text):
        return datetime.strptime(text, "%Y-%m-%d").replace(tzinfo=timezone.utc)
    match = RFC3339.fullmatch(text)
    if match is None:
        raise MetaError(f"invalid timestamp {value!r}")
    year, month, day, hour, minute, second = (
        int(match.group(index)) for index in range(1, 7)
    )
    if match.group(7) == "Z":
        zone = timezone.utc
    else:
        offset = timedelta(hours=int(match.group(9)), minutes=int(match.group(10)))
        zone = timezone(-offset if match.group(8) == "-" else offset)
    return datetime(year, month, day, hour, minute, second, tzinfo=zone)
