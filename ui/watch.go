package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
)

// defaultDebounce is the window over which a burst of writes is batched.
const defaultDebounce = 200 * time.Millisecond

// archiveDir is the track-level folder the watcher never looks into.
const archiveDir = "_archive"

// foldersChanged carries the Application folders touched since the last batch, and the
// documents written in them (writingKey paths), for the live writing marks.
type foldersChanged struct {
	folders []string
	written []string
}

// debouncer groups changed folders into windows. The first change opens a window
// and everything changed before it closes is delivered as one batch, each folder
// once. How a window is timed is injected so tests can close it by hand.
type debouncer struct {
	mu       sync.Mutex
	pending  map[string]struct{}
	armed    bool
	schedule func(closeWindow func())
	deliver  func(folders []string)
}

func newDebouncer(window time.Duration, deliver func([]string)) *debouncer {
	return &debouncer{schedule: func(f func()) { time.AfterFunc(window, f) }, deliver: deliver}
}

func (d *debouncer) add(folder string) {
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

func (d *debouncer) closeWindow() {
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

// watcher turns file system events under jobs/ into batches of changed Application
// folders. It watches jobs/, each track folder, each Application folder and its
// out/, adds watches as folders appear, and ignores _archive/.
type watcher struct {
	jobs    string
	fs      *fsnotify.Watcher
	batches chan []string
	done    chan struct{}
	once    sync.Once
	deb     *debouncer
	mu      sync.Mutex
	dirs    map[string]bool // directories currently watched
	written []string        // documents written since the last batch, as writingKey paths
}

func startWatcher(jobs string, window time.Duration) (*watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &watcher{jobs: jobs, fs: fsw, batches: make(chan []string, batchBuffer), done: make(chan struct{}), dirs: map[string]bool{}}
	w.deb = newDebouncer(window, w.deliver)
	if _, err := os.Stat(jobs); err == nil {
		w.addTree(jobs, false)
	}
	go w.loop()
	return w, nil
}

func (w *watcher) deliver(folders []string) {
	select {
	case w.batches <- folders:
	case <-w.done:
	}
}

func (w *watcher) close() {
	if w == nil {
		return
	}
	w.once.Do(func() {
		close(w.done)
		w.fs.Close()
	})
}

// next waits for the next batch of changed folders.
func (w *watcher) next() tea.Cmd {
	return func() tea.Msg {
		select {
		case b := <-w.batches:
			w.mu.Lock()
			written := w.written
			w.written = nil
			w.mu.Unlock()
			return foldersChanged{b, written}
		case <-w.done:
			return nil
		}
	}
}

func (w *watcher) loop() {
	for {
		select {
		case ev, ok := <-w.fs.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fs.Errors:
			if !ok {
				return
			}
		case <-w.done:
			return
		}
	}
}

// parts splits a path under jobs/ into its components, or reports false for paths
// that are outside jobs/ or under _archive/.
func (w *watcher) parts(path string) ([]string, bool) {
	rel, err := filepath.Rel(w.jobs, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return nil, false
	}
	p := strings.Split(rel, string(filepath.Separator))
	return p, p[0] != archiveDir
}

// Path depths under jobs/ (number of segments below it).
const (
	depthJobs        = 0
	depthTrack       = 1
	depthApplication = 2
	depthOut         = 3

	batchBuffer = 16
)

// wanted reports whether a directory at this depth under jobs/ is watched: track
// folders and Application folders always, out/ inside an Application folder.
func wanted(parts []string) bool {
	switch len(parts) {
	case depthJobs, depthTrack, depthApplication:
		return true
	case depthOut:
		return parts[2] == "out"
	}
	return false
}

// addTree watches dir and every wanted folder below it. With mark set, each
// Application folder found is reported as changed, since files may have landed
// before its watch existed.
func (w *watcher) addTree(dir string, mark bool) {
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		var p []string
		if path != w.jobs {
			var ok bool
			if p, ok = w.parts(path); !ok {
				return filepath.SkipDir
			}
		}
		if !wanted(p) {
			return filepath.SkipDir
		}
		w.mu.Lock()
		known := w.dirs[path]
		w.dirs[path] = true
		w.mu.Unlock()
		if !known {
			w.fs.Add(path)
		}
		if mark && len(p) == 2 {
			w.deb.add(path)
		}
		return nil
	})
}

func (w *watcher) handle(ev fsnotify.Event) {
	if ev.Op == fsnotify.Chmod {
		return
	}
	p, ok := w.parts(ev.Name)
	if !ok {
		return
	}
	created := ev.Has(fsnotify.Create)
	isDir := false
	if created {
		if info, err := os.Stat(ev.Name); err == nil {
			isDir = info.IsDir()
		}
	}
	if created && isDir && wanted(p) {
		w.addTree(ev.Name, true)
	}
	switch len(p) {
	case 1:
		// A track folder appeared or went away: its Applications are reported by
		// addTree on creation, and as their own removals otherwise.
		if !created && w.forget(ev.Name) {
			w.deb.add(ev.Name)
		}
	case depthApplication:
		if created && !isDir {
			return // loose file beside Application folders, e.g. TRACKER.md
		}
		if !created && !w.forget(ev.Name) {
			return
		}
		w.deb.add(ev.Name)
	default:
		if !created {
			w.forget(ev.Name)
		}
		folder := filepath.Join(w.jobs, p[0], p[1])
		if f := chipFile(p[depthApplication:]); f != "" && (ev.Has(fsnotify.Write) || created) {
			w.mu.Lock()
			w.written = append(w.written, writingKey(folder, f))
			w.mu.Unlock()
		}
		w.deb.add(folder)
	}
}

// forget drops a removed directory (and anything below it) from the watched set and
// reports whether it was one.
func (w *watcher) forget(path string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	was := w.dirs[path]
	for d := range w.dirs {
		if d == path || strings.HasPrefix(d, path+string(filepath.Separator)) {
			delete(w.dirs, d)
		}
	}
	return was
}
