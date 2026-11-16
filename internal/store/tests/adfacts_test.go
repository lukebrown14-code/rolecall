package store_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

func TestAdFactKeysAreRead(t *testing.T) {
	s := fixtureProfileStore(t)
	folder := writeLead(t, s, "facts", leadHead+"recruiter: Wattle Talent\nemployment_type: contract\nterm_months: 6\nextension: possible\n"+
		"sponsorship: offered\npay_min: 95.5\npay_max: 110\npay_unit: hour\ncurrency: USD\ncloses_unclear: true\njd_completeness: blurb\n"+
		"added_by: sweep\nsearch_query: \"data engineer\"\nlisting: gone\nlisting_checked: 2026-11-14\nfilter_reason: pulled\n")
	a := rescan(t, s, folder)
	if a.Malformed != nil {
		t.Fatal(a.Malformed)
	}
	f := a.AdFacts
	if f.Recruiter != "Wattle Talent" || f.EmploymentType != "contract" || *f.TermMonths != 6 || f.Extension != "possible" ||
		f.Sponsorship != "offered" || *f.PayMin != 95.5 || *f.PayMax != 110 || f.PayUnit != "hour" || f.Currency != "USD" ||
		!f.ClosesUnclear || f.JDCompleteness != "blurb" || f.AddedBy != "sweep" || f.SearchQuery != "data engineer" ||
		f.Listing != "gone" || f.ListingChecked != "2026-11-14" || a.FilterReason != store.FilterPulled {
		t.Fatalf("facts = %+v", f)
	}
	if got := a.CompanyLabel(); got != "Acme via Wattle Talent" {
		t.Errorf("CompanyLabel = %q", got)
	}
}

func TestBadAdFactKeysAreMalformed(t *testing.T) {
	s := fixtureProfileStore(t)
	for _, line := range []string{"employment_type: gig", "term_months: 0", "extension: maybe", "sponsorship: yes",
		"pay_min: -1", "pay_max: lots", "pay_unit: fortnight", "currency: aud", "currency: AUDX", "closes_unclear: yes",
		"jd_completeness: half", "added_by: robot", "listing: up", "listing_checked: 2026-13-01"} {
		folder := writeLead(t, s, "bad", leadHead+line+"\n")
		if a := rescan(t, s, folder); a.Malformed == nil {
			t.Errorf("%q read as valid", line)
		}
	}
}

func f64(v float64) *float64 { return &v }

func TestEstimatePay(t *testing.T) {
	for _, c := range []struct {
		facts    store.AdFacts
		fx       map[string]float64
		lo, hi   int
		ok       bool
		mislabel string
	}{
		{store.AdFacts{PayMin: f64(900), PayMax: f64(1100), PayUnit: "day", Currency: "AUD"}, nil, 207, 253, true, ""},
		{store.AdFacts{PayMin: f64(140000), PayMax: f64(170000), PayUnit: "year", Currency: "AUD"}, nil, 140, 170, true, ""},
		{store.AdFacts{PayMin: f64(850), PayUnit: "hour", Currency: "AUD"}, nil, 1680, 1680, true, "looks like a day rate"},
		{store.AdFacts{PayMin: f64(6000), PayUnit: "day", Currency: "AUD"}, nil, 1380, 1380, true, "looks like an annual figure"},
		{store.AdFacts{PayMin: f64(90), PayMax: f64(120), PayUnit: "year", Currency: "AUD"}, nil, 0, 0, true, "looks like an hourly or daily rate"},
		{store.AdFacts{PayMin: f64(100000), PayUnit: "year", Currency: "USD"}, nil, 155, 155, true, ""},
		{store.AdFacts{PayMin: f64(100000), PayUnit: "year", Currency: "USD"}, map[string]float64{"USD": 1.5}, 150, 150, true, ""},
		{store.AdFacts{PayMin: f64(100000), PayUnit: "year", Currency: "JPY"}, nil, 0, 0, false, ""},
		{store.AdFacts{PayMin: f64(100000), PayUnit: "year", Currency: "JPY"}, map[string]float64{"JPY": 0.01}, 1, 1, true, ""},
		{store.AdFacts{}, nil, 0, 0, false, ""},
	} {
		e := store.EstimatePay(c.facts, c.fx)
		if e.OK != c.ok || store.RoundThousands(e.MinAUD) != c.lo || store.RoundThousands(e.MaxAUD) != c.hi || e.Mislabel != c.mislabel {
			t.Errorf("%+v: got %+v", c.facts, e)
		}
	}
}

func TestVisaAndPayFXAreReadAndWritten(t *testing.T) {
	s := fixtureProfileStore(t)
	if err := s.SetVisa(store.VisaNeedsSponsorship, "true"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVisa(store.VisaMaxContract, "6"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetVisa(store.VisaMaxContract, "0"); err == nil {
		t.Error("a cap of 0 was accepted")
	}
	ws, err := s.Settings()
	if err != nil || ws.Err != nil {
		t.Fatal(err, ws.Err)
	}
	if !ws.Visa.NeedsSponsorship || ws.Visa.MaxContractMonths != 6 {
		t.Fatalf("visa = %+v", ws.Visa)
	}
	if err := s.SetVisa(store.VisaMaxContract, ""); err != nil {
		t.Fatal(err)
	}
	if ws, _ = s.Settings(); ws.Visa.Set[store.VisaMaxContract] {
		t.Error("clearing the cap left it set")
	}
	for _, bad := range []string{`"visa":{"max_contract_months":0}`, `"visa":{"needs_sponsorship":"yes"}`, `"visa":[]`, `"pay_fx":{"usd":1.5}`, `"pay_fx":{"USD":"x"}`} {
		path := filepath.Join(s.Root(), "workspace.json")
		if err := os.WriteFile(path, []byte(`{"tracks":["SWE"],`+bad+"}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if ws, _ := s.Settings(); ws.Err == nil {
			t.Errorf("%s read as valid", bad)
		}
	}
}

func TestKeysNormalise(t *testing.T) {
	for in, want := range map[string]string{
		"Senior Data Engineer (Contract)":  "senior data engineer",
		"The Level-3 Engineer, a Platform": "3 engineer platform",
		"  Backend   Engineer  ":           "backend engineer",
	} {
		if got := store.TitleKey(in); got != want {
			t.Errorf("TitleKey(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{"Quokka Energy Pty Ltd": "quokka energy", "Numbat Group Australia": "numbat", "A.B.C. Inc": "a b c"} {
		if got := store.CompanyKey(in); got != want {
			t.Errorf("CompanyKey(%q) = %q, want %q", in, got, want)
		}
	}
	if store.JDHash("short ad") != "" {
		t.Error("a short ad was hashed")
	}
	long := strings.Repeat("Build data pipelines. ", 20)
	if store.JDHash(long) == "" || store.JDHash(long) != store.JDHash(strings.ToUpper(strings.ReplaceAll(long, " ", "\n\t "))) {
		t.Error("case and whitespace changed the hash")
	}
}

func TestLikelyDuplicates(t *testing.T) {
	s := fixtureProfileStore(t)
	folders := func(apps []store.Application) []string {
		var out []string
		for _, a := range apps {
			out = append(out, filepath.Base(a.Folder))
		}
		return out
	}
	// Same title and company, however written.
	got, err := s.LikelyDuplicates("Quokka Energy Pty Ltd", "", "Data Engineer (Contract)", "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(folders(got), []string{"lead-contract"}) {
		t.Errorf("by company = %v", folders(got))
	}
	// The recruiter the existing Lead names, given as the new ad's company.
	if got, _ = s.LikelyDuplicates("Wattle Talent", "", "Data Engineer", ""); !reflect.DeepEqual(folders(got), []string{"lead-contract"}) {
		t.Errorf("by recruiter = %v", folders(got))
	}
	if got, _ = s.LikelyDuplicates("Other Co", "", "Data Engineer", ""); len(got) != 0 {
		t.Errorf("another company matched: %v", folders(got))
	}
	// The same ad text, under another title.
	text := strings.Repeat("Run our data platform end to end. ", 10)
	writeLead(t, s, "same-text", leadHead)
	if err := os.WriteFile(filepath.Join(s.Root(), "jobs", "SWE", "same-text", "jd.md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.LikelyDuplicates("Elsewhere", "", "Something else", strings.ToUpper(text)); !reflect.DeepEqual(folders(got), []string{"same-text"}) {
		t.Errorf("by text = %v", folders(got))
	}
}

func TestFitGapsAndCommonGaps(t *testing.T) {
	got := store.FitGaps("# Fit\n\n## Strengths\n- Go\n\n## Gaps (for gaps.md if proceeding)\n- **Terraform** — asked twice\n- `Kafka`: streaming\n" +
		"* On-call - nights\n- terraform\n\n## Next\n- Not a gap\n")
	if want := []string{"terraform", "kafka", "on-call"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FitGaps = %v, want %v", got, want)
	}
	s := fixtureProfileStore(t)
	apps, err := s.Scan()
	if err != nil {
		t.Fatal(err)
	}
	gaps := s.CommonGaps(apps)
	if len(gaps) < 2 || gaps[0] != (store.GapCount{Name: "terraform", Count: 3, Unlocks: 3}) || gaps[1] != (store.GapCount{Name: "kafka", Count: 2, Unlocks: 2}) {
		t.Fatalf("CommonGaps = %+v", gaps)
	}
}
