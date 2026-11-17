package ui

import (
	"errors"
	"strings"
	"testing"
)

// failedFetch is a failed new-job.py --fetch-only run: its stderr as the script prints it
// (the WARNING block, then one JSON line) and a non-zero exit.
func failedFetch(stderr string) CapturedResult {
	return CapturedResult{Stderr: stderr, Err: errors.New("exit status 2")}
}

// These lines are what plugin/scripts/new-job.py prints; plugin/tests/test_fetch_failures.py
// drives the real script for each kind and pins the line format.
func TestParseFetchShowsAMessageForEachFailureKind(t *testing.T) {
	cases := []struct {
		kind, line, want string
	}{
		{"login_wall", `{"error": "www.seek.com.au serves job ads behind a login, so fetching it will not work.", "kind": "login_wall"}`, "needs a login"},
		{"blocked", `{"error": "Could not fetch https://x.test/a -- HTTP Error 403: Forbidden", "kind": "blocked"}`, "refused the request"},
		{"not_found", `{"error": "Could not fetch https://x.test/a -- HTTP Error 404: Not Found", "kind": "not_found"}`, "was not found"},
		{"timeout", `{"error": "Could not fetch https://x.test/a -- timed out after 20s", "kind": "timeout"}`, "too long to answer"},
		{"bad_url", `{"error": "Could not fetch example.com -- not a web address; it must start with http:// or https://", "kind": "bad_url"}`, "not a web address"},
		{"non_html", `{"error": "Could not fetch https://x.test/a.pdf -- is application/pdf, not a web page", "kind": "non_html"}`, "a file, not a web page"},
		{"unreachable", `{"error": "Could not fetch https://x.test/a -- connection refused", "kind": "unreachable"}`, "Could not reach the site"},
		{"thin_page", `{"error": "Fetched https://x.test/a but only got 12 characters of text.", "kind": "thin_page"}`, "built in the browser"},
		{"too_short", `{"error": "Job description is only 4 characters -- that looks wrong.", "kind": "too_short"}`, "too little text"},
	}
	for _, c := range cases {
		t.Run(c.kind, func(t *testing.T) {
			stderr := "\n  WARNING\n  The first human line.\n  Paste instead.\n\n" + c.line + "\n"
			got := parseFetch("https://x.test/a", failedFetch(stderr))
			if !strings.Contains(got.failed, c.want) {
				t.Fatalf("failed = %q, want it to contain %q", got.failed, c.want)
			}
			if strings.Contains(got.failed, "{") || strings.Contains(got.failed, "WARNING") {
				t.Fatalf("failed = %q shows raw script output", got.failed)
			}
		})
	}
}

func TestParseFetchUnknownKindShowsItsErrorText(t *testing.T) {
	stderr := "\n  WARNING\n  ignored\n\n" + `{"error": "Something new went wrong.", "kind": "from_the_future"}` + "\n"
	if got := parseFetch("u", failedFetch(stderr)); got.failed != "Something new went wrong." {
		t.Fatalf("failed = %q", got.failed)
	}
}

// A failure without the JSON line (an older plugin, a crash) still shows its text.
func TestParseFetchWithoutJSONFallsBackToTheFirstLine(t *testing.T) {
	stderr := "\n  WARNING\n  example.test serves job ads behind a login.\n  more\n"
	if got := parseFetch("u", failedFetch(stderr)); got.failed != "example.test serves job ads behind a login." {
		t.Fatalf("failed = %q", got.failed)
	}
	if got := parseFetch("u", failedFetch("")); got.failed != fetchGenericReason {
		t.Fatalf("failed = %q", got.failed)
	}
}

func TestParseFetchSuccessIgnoresFailureParsing(t *testing.T) {
	out := `{"jd": "Some ad", "signals": [], "meta": {"role": "R", "company": "C", "closes": ""}}`
	got := parseFetch("u", CapturedResult{Stdout: out})
	if got.failed != "" || got.jd != "Some ad" {
		t.Fatalf("got %+v", got)
	}
}
