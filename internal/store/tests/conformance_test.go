package store_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// knownDrift lists the shared conformance vectors the Go store does not meet yet, by the
// audit finding that tracks each (reports/code-audit-2026-11-18). A listed vector is
// skipped while it fails and fails once it passes, so whoever fixes the finding deletes
// its entry here. plugin/tests/test_conformance.py keeps the Python list.
var knownDrift = map[string]struct {
	reason  string
	vectors []string
}{}

// driftFor is the finding that lists vector, or "".
func driftFor(vector string) string {
	for id, d := range knownDrift {
		for _, v := range d.vectors {
			if v == vector {
				return id
			}
		}
	}
	return ""
}

// checkVector runs one shared vector as a subtest: problems are what it got wrong. A
// known-drift vector is skipped while wrong and fails once right, so its entry is removed.
func checkVector(t *testing.T, section, name string, problems func() []string) {
	t.Helper()
	vector := section + "/" + name
	t.Run(name, func(t *testing.T) {
		got := problems()
		id := driftFor(vector)
		switch {
		case id != "" && len(got) == 0:
			t.Errorf("%s now conforms: remove it from knownDrift[%s]", vector, id)
		case id != "":
			t.Skipf("known drift %s (%s): %s", id, knownDrift[id].reason, strings.Join(got, "; "))
		default:
			for _, p := range got {
				t.Error(p)
			}
		}
	})
}

type conformance struct {
	Base  string
	Lines []struct {
		Line  string
		Valid bool
	}
	Enums      map[string][]string
	PayPerYear map[string]float64
	PayFX      map[string]float64
	Malformed  []struct {
		Name   string
		Text   string
		Valid  bool
		Line   int
		Reason string
	}
	Slugs []struct {
		Name, Company, Title, Date, Want string
	}
}

func readConformance(t *testing.T) conformance {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "conformance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c conformance
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

// Every drift entry names a vector the fixture still has, so a renamed vector cannot
// leave a stale skip behind.
func TestKnownDriftNamesSharedVectors(t *testing.T) {
	c := readConformance(t)
	names := map[string]bool{}
	for _, m := range c.Malformed {
		names["malformed/"+m.Name] = true
	}
	for _, s := range c.Slugs {
		names["slugs/"+s.Name] = true
	}
	for id, d := range knownDrift {
		for _, v := range d.vectors {
			if !names[v] {
				t.Errorf("knownDrift[%s] names %q, which is not a store vector in fixtures/conformance.json", id, v)
			}
		}
	}
}

// The Go store and the Python reader give every fixture line the same verdict:
// plugin/tests/test_conformance.py asserts the same file.
func TestMetaLinesMatchTheSharedConformanceFixture(t *testing.T) {
	c := readConformance(t)
	s := fixtureProfileStore(t)
	for i, l := range c.Lines {
		folder := writeLead(t, s, fmt.Sprintf("conformance-%d", i), c.Base+l.Line+"\n")
		a, err := s.Rescan(folder)
		if err != nil {
			t.Fatal(err)
		}
		if got := a.Malformed == nil; got != l.Valid {
			t.Errorf("%q: valid = %v, want %v (%v)", l.Line, got, l.Valid, a.Malformed)
		}
	}
}

// Whole meta.yml texts are malformed, on the line and for the reason the fixture names, on
// both sides.
func TestMalformedTextsMatchTheSharedConformanceFixture(t *testing.T) {
	c := readConformance(t)
	s := fixtureProfileStore(t)
	for i, m := range c.Malformed {
		checkVector(t, "malformed", m.Name, func() []string {
			folder := writeLead(t, s, fmt.Sprintf("malformed-%d", i), m.Text)
			a, err := s.Rescan(folder)
			if err != nil {
				return []string{err.Error()}
			}
			if got := a.Malformed == nil; got != m.Valid {
				return []string{fmt.Sprintf("valid = %v, want %v (%v)", got, m.Valid, a.Malformed)}
			}
			if m.Valid {
				return nil
			}
			var problems []string
			var pe *store.ParseError
			if !errors.As(a.Malformed, &pe) || pe.Line != m.Line {
				problems = append(problems, fmt.Sprintf("error %v, want line %d", a.Malformed, m.Line))
			}
			if !strings.Contains(a.Malformed.Error(), m.Reason) {
				problems = append(problems, fmt.Sprintf("error %v, want it to say %q", a.Malformed, m.Reason))
			}
			return problems
		})
	}
}

// Create names a folder by SCHEMA's slug rule, as Python's unique_folder must.
func TestFolderSlugsMatchTheSharedConformanceFixture(t *testing.T) {
	c := readConformance(t)
	s := fixtureProfileStore(t)
	for _, v := range c.Slugs {
		checkVector(t, "slugs", v.Name, func() []string {
			day, err := time.Parse(time.DateOnly, v.Date)
			if err != nil {
				return []string{err.Error()}
			}
			folder, err := s.Create(store.CreateFields{Company: v.Company, Title: v.Title, Track: "SWE"}, day.Add(12*time.Hour))
			if err != nil {
				return []string{err.Error()}
			}
			defer func() { _ = os.RemoveAll(folder) }()
			if got := filepath.Base(folder); got != v.Want {
				return []string{fmt.Sprintf("folder %q, want %q", got, v.Want)}
			}
			return nil
		})
	}
}

func TestAdFactTablesMatchTheSharedConformanceFixture(t *testing.T) {
	c := readConformance(t)
	enums := map[string][]string{
		"employment_type": store.EmploymentTypes, "extension": store.ExtensionValues,
		"sponsorship": store.SponsorshipValues, "pay_unit": store.PayUnits,
		"jd_completeness": store.JDCompleteness, "added_by": store.AddedByValues,
		"listing": store.ListingValues,
	}
	if !reflect.DeepEqual(enums, c.Enums) {
		t.Errorf("enums = %v, want %v", enums, c.Enums)
	}
	one := 1.0
	for unit, want := range c.PayPerYear {
		if got := store.EstimatePay(store.AdFacts{PayMin: &one, PayUnit: unit}, nil); !got.OK || got.MinAUD != want {
			t.Errorf("one %s a year = %+v, want %v", unit, got, want)
		}
	}
	for _, unit := range store.PayUnits {
		if _, ok := c.PayPerYear[unit]; !ok {
			t.Errorf("pay unit %s missing from payPerYear", unit)
		}
	}
	if !reflect.DeepEqual(store.DefaultPayFX, c.PayFX) {
		t.Errorf("DefaultPayFX = %v, want %v", store.DefaultPayFX, c.PayFX)
	}
}
