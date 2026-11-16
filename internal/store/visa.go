package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The workspace.json keys for the Owner's visa limits and pay conversion.
const (
	keyVisa  = "visa"
	keyPayFX = "pay_fx"
)

// The visa keys of workspace.json.
const (
	VisaNeedsSponsorship = "needs_sponsorship"
	VisaMaxContract      = "max_contract_months"
)

// Visa is workspace.json visa: whether the Owner needs sponsorship, and the longest
// contract the visa allows (0 when unset).
type Visa struct {
	NeedsSponsorship  bool
	MaxContractMonths int
	// Set names the keys visa sets itself.
	Set map[string]bool
}

// decodeVisa reads the visa object; a missing key is no limits.
func decodeVisa(raw json.RawMessage) (Visa, error) {
	out := Visa{Set: map[string]bool{}}
	if raw == nil {
		return out, nil
	}
	var v struct {
		Needs *bool `json:"needs_sponsorship"`
		Max   *int  `json:"max_contract_months"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		return out, err
	}
	if v.Max != nil && *v.Max < 1 {
		return out, errors.New("max_contract_months must be 1 or more")
	}
	if v.Needs != nil {
		out.NeedsSponsorship, out.Set[VisaNeedsSponsorship] = *v.Needs, true
	}
	if v.Max != nil {
		out.MaxContractMonths, out.Set[VisaMaxContract] = *v.Max, true
	}
	return out, nil
}

// decodePayFX reads pay_fx: currency code to its value in AUD.
func decodePayFX(raw json.RawMessage) (map[string]float64, error) {
	if raw == nil {
		return nil, nil
	}
	var fx map[string]float64
	if err := json.Unmarshal(raw, &fx); err != nil {
		return nil, err
	}
	for code, rate := range fx {
		if !currencyPattern.MatchString(code) || rate <= 0 {
			return nil, fmt.Errorf("%q: want a 3-letter code and a number above 0", code)
		}
	}
	return fx, nil
}

// SetVisa writes one visa key from text as the Owner typed it: "true" or "false" for
// needs_sponsorship, a whole number above 0 for max_contract_months. Empty text clears it.
func (s *Store) SetVisa(key, text string) error {
	text = strings.TrimSpace(text)
	var value json.RawMessage
	switch key {
	case VisaNeedsSponsorship:
		if text != "" && text != "true" && text != "false" {
			return errors.New("choose yes or no")
		}
		if text != "" {
			value = json.RawMessage(text)
		}
	case VisaMaxContract:
		if text != "" {
			n, err := strconv.Atoi(text)
			if err != nil || n < 1 {
				return errors.New("enter a whole number above 0")
			}
			value = json.RawMessage(strconv.Itoa(n))
		}
	default:
		return fmt.Errorf("unknown visa key %q", key)
	}
	w, err := readWorkspace(s.root)
	if err != nil {
		return err
	}
	if w.ParseErr != nil {
		return fmt.Errorf("%s is not valid JSON, not writing: %w", w.Path, w.ParseErr)
	}
	obj := []byte("{}")
	if raw, ok := w.Values[keyVisa]; ok {
		obj = raw
	}
	next, err := setObjectKey(obj, key, value)
	if err != nil {
		return fmt.Errorf("visa: %w", err)
	}
	return s.setWorkspaceKey(keyVisa, next)
}
