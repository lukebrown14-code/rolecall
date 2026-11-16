package store_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

type conformance struct {
	Base  string
	Lines []struct {
		Line  string
		Valid bool
	}
	Enums      map[string][]string
	PayPerYear map[string]float64
	PayFX      map[string]float64
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
