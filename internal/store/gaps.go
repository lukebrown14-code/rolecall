package store

import (
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// GapCount is one gap fit.md names, across Applications: how many name it, and how many
// of those are not Dismissed (the Applications closing it would help).
type GapCount struct {
	Name    string
	Count   int
	Unlocks int
}

// FitGaps is the gaps one fit.md lists: each bullet under a "## " heading that starts
// with "Gaps" (any case), up to the next heading. A bullet's name is its text before the
// first " — ", " - " or ":", with ** and backticks stripped, lower-cased and trimmed;
// each name once.
func FitGaps(text string) []string {
	var out []string
	in := false
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") {
			head, ok := strings.CutPrefix(t, "## ")
			in = ok && strings.HasPrefix(strings.ToLower(strings.TrimSpace(head)), "gaps")
			continue
		}
		if !in {
			continue
		}
		item, ok := strings.CutPrefix(t, "- ")
		if !ok {
			item, ok = strings.CutPrefix(t, "* ")
		}
		if !ok {
			continue
		}
		for _, sep := range []string{" — ", " - ", ":"} {
			item, _, _ = strings.Cut(item, sep)
		}
		item = strings.NewReplacer("**", "", "`", "").Replace(item)
		if name := strings.ToLower(strings.TrimSpace(item)); name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	return out
}

// CommonGaps counts the gaps every given Application's fit.md lists, most named first
// (then by name). An Application without a fit.md, or one that does not read, adds none.
func (s *Store) CommonGaps(apps []Application) []GapCount {
	counts := map[string]*GapCount{}
	for _, a := range apps {
		if a.Malformed != nil || !a.HasFit {
			continue
		}
		b, err := os.ReadFile(filepath.Join(a.Folder, "fit.md"))
		if err != nil {
			continue
		}
		for _, g := range FitGaps(DocumentText(string(b))) {
			c := counts[g]
			if c == nil {
				c = &GapCount{Name: g}
				counts[g] = c
			}
			c.Count++
			if a.Status != StatusDismissed {
				c.Unlocks++
			}
		}
	}
	out := make([]GapCount, 0, len(counts))
	for _, c := range counts {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}
