// Package debounce groups folder changes into scheduled batches.
package debounce

import (
	"sort"
	"sync"
)

// Debouncer groups changed folders into windows. The first change opens a window
// and everything changed before it closes is delivered as one batch, each folder
// once. How a window is timed is injected so tests can close it by hand.
type Debouncer struct {
	mu       sync.Mutex
	pending  map[string]struct{}
	armed    bool
	schedule func(closeWindow func())
	deliver  func(folders []string)
}

// New groups additions using the supplied window scheduler and batch receiver.
func New(schedule func(func()), deliver func([]string)) *Debouncer {
	return &Debouncer{schedule: schedule, deliver: deliver}
}

// Add records a folder and opens a window if needed.
func (d *Debouncer) Add(folder string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.pending == nil {
		d.pending = map[string]struct{}{}
	}
	d.pending[folder] = struct{}{}
	if !d.armed {
		d.armed = true
		d.schedule(d.closeWindow)
	}
}

func (d *Debouncer) closeWindow() {
	d.mu.Lock()
	folders := make([]string, 0, len(d.pending))
	for f := range d.pending {
		folders = append(folders, f)
	}
	d.pending, d.armed = nil, false
	d.mu.Unlock()
	sort.Strings(folders)
	d.deliver(folders)
}
