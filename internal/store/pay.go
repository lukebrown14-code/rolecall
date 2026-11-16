package store

import "math"

// payPerYear is how many of each pay unit make a year, for the annual estimate.
//
//nolint:mnd // the contract's fixed multipliers, mirrored by the Python side
var payPerYear = map[string]float64{PayHour: 1976, PayDay: 230, PayWeek: 52, PayMonth: 12, PayYear: 1}

// DefaultPayFX is each currency's value in AUD, used unless workspace.json pay_fx sets it.
//
//nolint:mnd // the contract's default rates, mirrored by the Python side
var DefaultPayFX = map[string]float64{"AUD": 1, "USD": 1.55, "GBP": 1.95, "EUR": 1.65, "NZD": 0.92, "CAD": 1.12, "SGD": 1.15}

// PayEstimate is an ad's pay as a yearly AUD range, and a warning when the stated unit
// looks wrong for the figure.
type PayEstimate struct {
	MinAUD, MaxAUD float64
	OK             bool
	Mislabel       string
}

// EstimatePay turns the ad's pay into a yearly AUD range. fx overrides and extends
// DefaultPayFX. No pay, no unit or an unknown currency is no estimate; a currency left
// unset is AUD. The mislabel warning is set even when there is no estimate.
func EstimatePay(f AdFacts, fx map[string]float64) PayEstimate {
	var out PayEstimate
	if f.PayMin == nil {
		return out
	}
	lo, hi := *f.PayMin, *f.PayMin
	if f.PayMax != nil {
		hi = *f.PayMax
	}
	switch {
	case f.PayUnit == PayHour && lo >= 300:
		out.Mislabel = "looks like a day rate"
	case f.PayUnit == PayDay && lo >= 5000:
		out.Mislabel = "looks like an annual figure"
	case f.PayUnit == PayYear && hi < 1000:
		out.Mislabel = "looks like an hourly or daily rate"
	}
	mult, ok := payPerYear[f.PayUnit]
	if !ok {
		return out
	}
	cur := f.Currency
	if cur == "" {
		cur = "AUD"
	}
	rate, ok := fx[cur]
	if !ok {
		rate, ok = DefaultPayFX[cur]
	}
	if !ok {
		return out
	}
	out.MinAUD, out.MaxAUD, out.OK = lo*mult*rate, hi*mult*rate, true
	return out
}

// RoundThousands is an amount to the nearest thousand, in thousands: 207480 is 207.
func RoundThousands(v float64) int { return int(math.Round(v / thousand)) }

const thousand = 1000
