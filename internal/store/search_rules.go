package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// DefaultExpiryWarningDays is how close to max_posting_age_days a Lead is before it is
// flagged, when search_rules sets no expiry_warning_days.
const DefaultExpiryWarningDays = 5

// The search_rules keys of workspace.json.
const (
	RuleMaxPostingAge   = "max_posting_age_days"
	RuleExpiryWarning   = "expiry_warning_days"
	RuleAmbiguousLabels = "ambiguous_age_labels"
	RuleMaxApplicants   = "max_applicants"
	RuleMaxContract     = "max_contract_months"
)

// What an ambiguous age label does to a Lead that shows it.
const (
	LabelDiscard = "discard"
	LabelKeep    = "keep"
)

// SearchRules are the search_rules of workspace.json: the Search rules the Sweep applies.
// A zero int is an unset rule; ExpiryWarningDays is the effective value, default 5.
type SearchRules struct {
	MaxPostingAgeDays int
	ExpiryWarningDays int
	// AmbiguousAgeLabels maps a label ("1mo ago") to discard or keep.
	AmbiguousAgeLabels map[string]string
	// MaxApplicants maps a Source name to the most applicants a Lead from it may have.
	MaxApplicants     map[string]int
	MaxContractMonths int
	// Set names the keys search_rules sets itself.
	Set map[string]bool
}

// The Source types a sources entry can name.
const (
	SourceAPI     = "api"
	SourceBrowser = "browser"
	SourceManual  = "manual"
)

// SourceTypes are the valid Source types, and SourcePlatforms the platforms an api Source reads.
var (
	SourceTypes     = []string{SourceAPI, SourceBrowser, SourceManual}
	SourcePlatforms = []string{"greenhouse", "ashby", "lever", "smartrecruiters", "michaelpage"}
	// SourceAccess is what a Source's last test found: whether the Sweep can read it.
	SourceAccess = []string{"works", "403", "login wall", "cloudflare", "unknown"}
)

// AccessUnknown is the access of a Source never tested.
const AccessUnknown = "unknown"

// Source is one entry of the sources registry: where the Sweep looks for Leads.
type Source struct {
	Name             string         `json:"name"`
	Type             string         `json:"type"`
	Platform         string         `json:"platform,omitempty"`
	Targets          []string       `json:"targets,omitempty"`
	URL              string         `json:"url,omitempty"`
	Access           string         `json:"access,omitempty"`
	Enabled          *bool          `json:"enabled,omitempty"`
	AgeFormat        string         `json:"age_format,omitempty"`
	ApplicantsFormat string         `json:"applicants_format,omitempty"`
	Notes            string         `json:"notes,omitempty"`
	Rules            map[string]int `json:"rules,omitempty"`
}

// On reports whether the Source is enabled: true unless enabled is false.
func (s Source) On() bool { return s.Enabled == nil || *s.Enabled }

// AccessText is the Source's access, unknown when unset.
func (s Source) AccessText() string {
	if s.Access == "" {
		return AccessUnknown
	}
	return s.Access
}

// decodeSearchRules reads the search_rules object; a missing key is all defaults.
func decodeSearchRules(raw json.RawMessage) (SearchRules, error) {
	out := SearchRules{ExpiryWarningDays: DefaultExpiryWarningDays, Set: map[string]bool{}}
	if raw == nil {
		return out, nil
	}
	var r struct {
		MaxPostingAge   *int              `json:"max_posting_age_days"`
		ExpiryWarning   *int              `json:"expiry_warning_days"`
		AmbiguousLabels map[string]string `json:"ambiguous_age_labels"`
		MaxApplicants   map[string]int    `json:"max_applicants"`
		MaxContract     *int              `json:"max_contract_months"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return out, err
	}
	set := func(key string, src *int, dst *int) {
		if src != nil {
			*dst = *src
			out.Set[key] = true
		}
	}
	set(RuleMaxPostingAge, r.MaxPostingAge, &out.MaxPostingAgeDays)
	set(RuleExpiryWarning, r.ExpiryWarning, &out.ExpiryWarningDays)
	set(RuleMaxContract, r.MaxContract, &out.MaxContractMonths)
	if r.AmbiguousLabels != nil {
		out.AmbiguousAgeLabels, out.Set[RuleAmbiguousLabels] = r.AmbiguousLabels, true
	}
	if r.MaxApplicants != nil {
		out.MaxApplicants, out.Set[RuleMaxApplicants] = r.MaxApplicants, true
	}
	return out, nil
}

// decodeSources reads sources in either form: the legacy object (platform → targets),
// read as one enabled api Source per platform, or the list of entries. An entry that is
// not an object is skipped.
func decodeSources(raw json.RawMessage) []Source {
	if raw == nil {
		return nil
	}
	if keys, values, err := orderedObject(raw); err == nil {
		out := make([]Source, 0, len(keys))
		for _, k := range keys {
			var targets []string
			_ = json.Unmarshal(values[k], &targets)
			out = append(out, Source{Name: k, Type: SourceAPI, Platform: k, Targets: targets})
		}
		return out
	}
	var list []json.RawMessage
	if json.Unmarshal(raw, &list) != nil {
		return nil
	}
	var out []Source
	for _, item := range list {
		var src Source
		if json.Unmarshal(item, &src) == nil && src.Name != "" {
			out = append(out, src)
		}
	}
	return out
}

// SetSearchRule writes one search_rules key from text as the Owner typed it. Empty text
// clears the key. The day and month rules take a whole number (max_posting_age_days and
// max_contract_months above 0, expiry_warning_days 0 or more); max_applicants takes
// "linkedin 100, seek 50" and ambiguous_age_labels "1mo ago = discard, 4w ago = keep".
// Every other key keeps its value and place; a refused value writes nothing.
func (s *Store) SetSearchRule(key, text string) error {
	text = strings.TrimSpace(text)
	var value json.RawMessage
	if text != "" {
		var err error
		if value, err = ruleValue(key, text); err != nil {
			return err
		}
	} else if !slices.Contains([]string{RuleMaxPostingAge, RuleExpiryWarning, RuleAmbiguousLabels, RuleMaxApplicants, RuleMaxContract}, key) {
		return fmt.Errorf("unknown Search rule %q", key)
	}
	w, err := readWorkspace(s.root)
	if err != nil {
		return err
	}
	if w.ParseErr != nil {
		return fmt.Errorf("%s is not valid JSON, not writing: %w", w.Path, w.ParseErr)
	}
	obj := []byte("{}")
	if raw, ok := w.Values["search_rules"]; ok {
		obj = raw
	}
	next, err := setObjectKey(obj, key, value)
	if err != nil {
		return fmt.Errorf("search_rules: %w", err)
	}
	return s.setWorkspaceKey("search_rules", next)
}

// ruleValue is the JSON a Search rule's typed text stands for, or why it is refused.
func ruleValue(key, text string) (json.RawMessage, error) {
	switch key {
	case RuleMaxPostingAge, RuleMaxContract, RuleExpiryWarning:
		n, err := strconv.Atoi(text)
		if err != nil || n < 0 || (n == 0 && key != RuleExpiryWarning) {
			if key == RuleExpiryWarning {
				return nil, errors.New("enter whole days, 0 or more")
			}
			return nil, errors.New("enter a whole number above 0")
		}
		return json.Marshal(n)
	case RuleMaxApplicants:
		caps, err := ParseApplicantCaps(text)
		if err != nil {
			return nil, err
		}
		return json.Marshal(caps)
	case RuleAmbiguousLabels:
		labels, err := ParseAgeLabels(text)
		if err != nil {
			return nil, err
		}
		return json.Marshal(labels)
	}
	return nil, fmt.Errorf("unknown Search rule %q", key)
}

// ParseApplicantCaps reads "linkedin 100, seek 50": a Source name then a whole number,
// comma-separated.
func ParseApplicantCaps(text string) (map[string]int, error) {
	out := map[string]int{}
	for _, part := range strings.Split(text, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		i := strings.LastIndexAny(part, " \t=:")
		n, err := strconv.Atoi(strings.TrimSpace(part[i+1:]))
		name := strings.TrimSpace(strings.TrimRight(part[:max(i, 0)], " \t=:"))
		if i < 0 || err != nil || n < 0 || name == "" {
			return nil, fmt.Errorf("write each cap as a Source then a number, like linkedin 100, not %q", part)
		}
		out[name] = n
	}
	return out, nil
}

// ParseAgeLabels reads "1mo ago = discard, 4w ago = keep".
func ParseAgeLabels(text string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(text, ",") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		label, action, ok := strings.Cut(part, "=")
		label, action = strings.Join(strings.Fields(label), " "), strings.ToLower(strings.TrimSpace(action))
		if !ok || label == "" || (action != LabelDiscard && action != LabelKeep) {
			return nil, fmt.Errorf("write each label as label = discard or label = keep, not %q", part)
		}
		out[label] = action
	}
	return out, nil
}

// FormatApplicantCaps is max_applicants as the Owner types it: "linkedin 100, seek 50".
func FormatApplicantCaps(caps map[string]int) string {
	names := sortedKeys(caps)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n + " " + strconv.Itoa(caps[n])
	}
	return strings.Join(parts, ", ")
}

// FormatAgeLabels is ambiguous_age_labels as the Owner types it: "1mo ago = discard".
func FormatAgeLabels(labels map[string]string) string {
	names := sortedKeys(labels)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n + " = " + labels[n]
	}
	return strings.Join(parts, ", ")
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// SetSourceEnabled turns the named Source on or off. A legacy sources object is
// rewritten as the list, keeping every platform and target.
func (s *Store) SetSourceEnabled(name string, on bool) error {
	return s.changeSources(func(list []json.RawMessage) ([]json.RawMessage, error) {
		for i, item := range list {
			var src Source
			if json.Unmarshal(item, &src) == nil && src.Name == name {
				enabled, _ := json.Marshal(on)
				next, err := setObjectKey(item, "enabled", enabled)
				if err != nil {
					return nil, err
				}
				list[i] = next
				return list, nil
			}
		}
		return nil, fmt.Errorf("no Source named %q", name)
	})
}

// AddSource appends a Source to the registry. The name must be new, the type api,
// browser or manual, and an api Source names one of SourcePlatforms. A legacy sources
// object is rewritten as the list, keeping every platform and target.
func (s *Store) AddSource(src Source) error {
	src.Name = strings.TrimSpace(src.Name)
	switch {
	case src.Name == "":
		return errors.New("a Source needs a name")
	case !slices.Contains(SourceTypes, src.Type):
		return errors.New("type must be api, browser or manual")
	case src.Type == SourceAPI && !slices.Contains(SourcePlatforms, src.Platform):
		return errors.New("an api Source needs a platform: " + strings.Join(SourcePlatforms, ", "))
	case src.Access != "" && !slices.Contains(SourceAccess, src.Access):
		return errors.New("access must be one of " + strings.Join(SourceAccess, ", "))
	}
	if src.Type != SourceAPI {
		src.Platform, src.Targets = "", nil
	}
	return s.changeSources(func(list []json.RawMessage) ([]json.RawMessage, error) {
		for _, item := range list {
			var other Source
			if json.Unmarshal(item, &other) == nil && strings.EqualFold(other.Name, src.Name) {
				return nil, fmt.Errorf("a Source named %q is already listed", other.Name)
			}
		}
		raw, err := json.Marshal(src)
		if err != nil {
			return nil, err
		}
		return append(list, raw), nil
	})
}

// RemoveSource takes the named Source off the registry, as the undo of AddSource does.
func (s *Store) RemoveSource(name string) error {
	return s.changeSources(func(list []json.RawMessage) ([]json.RawMessage, error) {
		return slices.DeleteFunc(list, func(item json.RawMessage) bool {
			var src Source
			return json.Unmarshal(item, &src) == nil && src.Name == name
		}), nil
	})
}

// changeSources rewrites sources as a list through change. The list form keeps every
// entry as written; the legacy object becomes one api entry per platform.
func (s *Store) changeSources(change func([]json.RawMessage) ([]json.RawMessage, error)) error {
	w, err := readWorkspace(s.root)
	if err != nil {
		return err
	}
	if w.ParseErr != nil {
		return fmt.Errorf("%s is not valid JSON, not writing: %w", w.Path, w.ParseErr)
	}
	raw := w.Values["sources"]
	var list []json.RawMessage
	if raw != nil && json.Unmarshal(raw, &list) != nil {
		if _, _, err := orderedObject(raw); err != nil {
			return fmt.Errorf("%s: sources is neither a list nor an object", w.Path)
		}
		for _, src := range decodeSources(raw) {
			item, err := json.Marshal(src)
			if err != nil {
				return err
			}
			list = append(list, item)
		}
	}
	list, err = change(list)
	if err != nil {
		return err
	}
	if list == nil {
		list = []json.RawMessage{}
	}
	value, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return s.setWorkspaceKey("sources", value)
}

// setObjectKey sets key in a JSON object, keeping the other keys and their order; a nil
// value removes it.
func setObjectKey(obj json.RawMessage, key string, value json.RawMessage) (json.RawMessage, error) {
	keys, values, err := orderedObject(obj)
	if err != nil {
		return nil, err
	}
	if value == nil {
		keys = slices.DeleteFunc(keys, func(k string) bool { return k == key })
		delete(values, key)
	} else {
		if _, ok := values[key]; !ok {
			keys = append(keys, key)
		}
		values[key] = value
	}
	var out bytes.Buffer
	out.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			out.WriteByte(',')
		}
		name, _ := json.Marshal(k)
		out.Write(name)
		out.WriteByte(':')
		out.Write(values[k])
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
