package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// closesUnclearText is a close date the ad did not let the Sweep read.
const closesUnclearText = "date unclear"

// adFactRow is one detail row of what the ad says, with an optional warning under it.
type adFactRow struct{ label, value, warn string }

// adFactRows are the detail rows of the ad's own facts, each only when the ad says.
func (m model) adFactRows(a store.Application) []adFactRow {
	var rows []adFactRow
	if a.Recruiter != "" {
		rows = append(rows, adFactRow{label: "RECRUITER", value: a.Recruiter + " · for " + a.Company})
	}
	if t := termText(a.AdFacts); t != "" {
		rows = append(rows, adFactRow{label: "TYPE", value: t})
	}
	if pay, warn := payText(a.AdFacts, m.payFX); pay != "" || warn != "" {
		rows = append(rows, adFactRow{label: "PAY", value: pay, warn: warn})
	}
	if a.Sponsorship != "" {
		rows = append(rows, adFactRow{label: "SPONSORSHIP", value: a.Sponsorship})
	}
	if a.JDCompleteness != "" {
		v := a.JDCompleteness
		if v == store.JDBlurb {
			v = badgeBlurb
		}
		rows = append(rows, adFactRow{label: "AD", value: v})
	}
	if a.AddedBy != "" || a.SearchQuery != "" {
		parts := []string{}
		if a.AddedBy != "" {
			parts = append(parts, a.AddedBy)
		}
		if a.SearchQuery != "" {
			parts = append(parts, strconv.Quote(a.SearchQuery))
		}
		rows = append(rows, adFactRow{label: "ADDED BY", value: strings.Join(parts, " · ")})
	}
	if a.Listing != "" || a.ListingChecked != "" {
		parts := []string{}
		if a.Listing != "" {
			parts = append(parts, a.Listing)
		}
		if a.ListingChecked != "" {
			parts = append(parts, "checked "+a.ListingChecked)
		}
		rows = append(rows, adFactRow{label: "LISTING", value: strings.Join(parts, " · ")})
	}
	return rows
}

// termText is the ad's terms: "contract · 6 months · extension possible".
func termText(f store.AdFacts) string {
	var parts []string
	if f.EmploymentType != "" {
		parts = append(parts, f.EmploymentType)
	}
	if f.TermMonths != nil {
		parts = append(parts, plural(*f.TermMonths, "month", "months"))
	}
	if f.Extension != "" {
		parts = append(parts, "extension "+f.Extension)
	}
	return strings.Join(parts, " · ")
}

// payText is the ad's pay as stated, then the yearly AUD estimate:
// "$900–1,100/day AUD · ~A$207k–253k/yr", and a warning when the unit looks wrong.
func payText(f store.AdFacts, fx map[string]float64) (text, warn string) {
	if f.PayMin == nil {
		return "", ""
	}
	stated := "$" + money(*f.PayMin)
	if f.PayMax != nil && *f.PayMax != *f.PayMin {
		stated += "–" + money(*f.PayMax)
	}
	if f.PayUnit != "" {
		stated += "/" + f.PayUnit
	}
	if f.Currency != "" {
		stated += " " + f.Currency
	}
	est := store.EstimatePay(f, fx)
	if est.OK {
		lo, hi := store.RoundThousands(est.MinAUD), store.RoundThousands(est.MaxAUD)
		yr := fmt.Sprintf("~A$%dk", lo)
		if hi != lo {
			yr += fmt.Sprintf("–%dk", hi)
		}
		stated += " · " + yr + "/yr"
	}
	return stated, est.Mislabel
}

// digitGroup is the digits between thousands separators; centEpsilon is the least
// fraction money shows as cents.
const (
	digitGroup  = 3
	centEpsilon = 0.0001
)

// money is an amount with thousands separators, keeping cents only when it has them.
func money(v float64) string {
	whole := int64(v)
	s := strconv.FormatInt(whole, 10)
	for i := len(s) - digitGroup; i > 0; i -= digitGroup {
		s = s[:i] + "," + s[i:]
	}
	if frac := v - float64(whole); frac > centEpsilon {
		s += strings.TrimPrefix(strconv.FormatFloat(frac, 'f', 2, 64), "0")
	}
	return s
}

// Row badges: what the ad says that matters against the Owner's visa. This is the one
// place their words live; the detail's AD row reuses badgeBlurb.
const (
	badgeBlurb       = "ad summary only"
	badgeNoSponsor   = "no sponsorship"
	badgeSponsors    = "sponsors visa"
	badgeContractFit = "contract fits visa cap"
)

// badgeContractOver is a contract longer than the visa allows: "12-mo contract > 6-mo visa cap".
func badgeContractOver(months, limit int) string {
	return fmt.Sprintf("%d-mo contract > %d-mo visa cap", months, limit)
}

// badgeTone is how a badge reads: a blocker against the visa, a fit with it, or a
// neutral note about the ad. A row draws a blocker with ⚠ and in bold, so it reads
// without colour; amber and red stay for time.
type badgeTone int

const (
	toneBlock badgeTone = iota
	toneOK
	toneNote
)

type badge struct {
	text string
	tone badgeTone
}

// adBadges are a Lead's or Draft's badges, blockers first, then what fits the visa, then
// notes about the ad: a narrow row cuts from the right, so it drops the least important.
func (m model) adBadges(a store.Application) []badge {
	if !a.Unsent() {
		return nil
	}
	var out []badge
	if limit := m.visa.MaxContractMonths; limit > 0 && a.TermMonths != nil {
		if *a.TermMonths <= limit {
			out = append(out, badge{badgeContractFit, toneOK})
		} else {
			out = append(out, badge{badgeContractOver(*a.TermMonths, limit), toneBlock})
		}
	}
	if m.visa.NeedsSponsorship {
		switch a.Sponsorship {
		case store.SponsorshipNotOffered:
			out = append(out, badge{badgeNoSponsor, toneBlock})
		case store.SponsorshipOffered:
			out = append(out, badge{badgeSponsors, toneOK})
		}
	}
	if a.JDCompleteness == store.JDBlurb {
		out = append(out, badge{badgeBlurb, toneNote})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].tone < out[j].tone })
	return out
}
