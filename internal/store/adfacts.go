package store

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The meta.yml keys that record what the ad itself says: who posted it, its terms and
// pay, how complete its text is, how it was found and whether the listing is still up.
const (
	keyRecruiter      = "recruiter"
	keyEmploymentType = "employment_type"
	keyTermMonths     = "term_months"
	keyExtension      = "extension"
	keySponsorship    = "sponsorship"
	keyPayMin         = "pay_min"
	keyPayMax         = "pay_max"
	keyPayUnit        = "pay_unit"
	keyCurrency       = "currency"
	keyClosesUnclear  = "closes_unclear"
	keyJDCompleteness = "jd_completeness"
	keyAddedBy        = "added_by"
	keySearchQuery    = "search_query"
	keyListing        = "listing"
	keyListingChecked = "listing_checked"
)

// The values the enumerated ad-fact keys take.
var (
	EmploymentTypes = []string{"permanent", "contract", "fixed-term", "casual", "part-time"}
	ExtensionValues = []string{"yes", "no", "possible"}
	// Sponsorship: whether the ad offers visa sponsorship.
	SponsorshipValues = []string{SponsorshipOffered, SponsorshipNotOffered, "unknown"}
	PayUnits          = []string{PayHour, PayDay, PayWeek, PayMonth, PayYear}
	JDCompleteness    = []string{"full", "summary", JDBlurb}
	AddedByValues     = []string{"sweep", "agent", AddedByOwner}
	ListingValues     = []string{"live", "gone", "closed"}
)

// Named values the code refers to.
const (
	SponsorshipOffered    = "offered"
	SponsorshipNotOffered = "not-offered"
	JDBlurb               = "blurb"
	AddedByOwner          = "owner"
	PayHour               = "hour"
	PayDay                = "day"
	PayWeek               = "week"
	PayMonth              = "month"
	PayYear               = "year"
)

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

// AdFacts are the optional meta.yml keys about the ad. Company is always the employer;
// Recruiter is the agency that posted the ad for it, if one did. Empty means unknown.
type AdFacts struct {
	Recruiter      string
	EmploymentType string
	TermMonths     *int
	Extension      string
	Sponsorship    string
	PayMin, PayMax *float64
	PayUnit        string
	Currency       string
	// ClosesUnclear marks an ad whose close date could not be read; Closes stays empty.
	ClosesUnclear  bool
	JDCompleteness string
	AddedBy        string
	SearchQuery    string
	Listing        string
	ListingChecked string
}

// readAdFact parses one ad-fact key into a. handled is false for any other key.
func (a *Application) readAdFact(k, v string) (handled bool, err error) {
	enum := func(dst *string, allowed []string) (bool, error) {
		if v != "" && !slices.Contains(allowed, v) {
			return true, fmt.Errorf("invalid %s", k)
		}
		*dst = v
		return true, nil
	}
	number := func(dst **float64) (bool, error) {
		if v == "" {
			return true, nil
		}
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 || f != f {
			return true, fmt.Errorf("invalid %s", k)
		}
		*dst = &f
		return true, nil
	}
	f := &a.AdFacts
	switch k {
	case keyRecruiter:
		f.Recruiter = strings.TrimSpace(DisplayText(v))
	case keySearchQuery:
		f.SearchQuery = DisplayText(v)
	case keyEmploymentType:
		return enum(&f.EmploymentType, EmploymentTypes)
	case keyExtension:
		return enum(&f.Extension, ExtensionValues)
	case keySponsorship:
		return enum(&f.Sponsorship, SponsorshipValues)
	case keyPayUnit:
		return enum(&f.PayUnit, PayUnits)
	case keyJDCompleteness:
		return enum(&f.JDCompleteness, JDCompleteness)
	case keyAddedBy:
		return enum(&f.AddedBy, AddedByValues)
	case keyListing:
		return enum(&f.Listing, ListingValues)
	case keyTermMonths:
		if v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return true, fmt.Errorf("invalid term_months")
			}
			f.TermMonths = &n
		}
	case keyPayMin:
		return number(&f.PayMin)
	case keyPayMax:
		return number(&f.PayMax)
	case keyCurrency:
		if v != "" && !currencyPattern.MatchString(v) {
			return true, fmt.Errorf("invalid currency")
		}
		f.Currency = v
	case keyListingChecked:
		if v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return true, fmt.Errorf("invalid listing_checked date")
			}
		}
		f.ListingChecked = v
	default:
		return false, nil
	}
	return true, nil
}

// CompanyLabel is the company as rows show it: "AGL via Chandler Macleod" when a
// recruiter posted the ad.
func (a Application) CompanyLabel() string {
	if a.Recruiter == "" || strings.EqualFold(a.Recruiter, a.Company) {
		return a.Company
	}
	return a.Company + " via " + a.Recruiter
}
