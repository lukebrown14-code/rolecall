package watch_test

import (
	"reflect"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui/watch/internal/debounce"
)

// The debounce window is closed by hand, so these tests involve no clock.
func TestDebouncerDeliversEachFolderOncePerWindow(t *testing.T) {
	var closeWindow func()
	var batches [][]string
	d := debounce.New(func(f func()) { closeWindow = f }, func(folders []string) { batches = append(batches, folders) })
	for i := 0; i < 50; i++ {
		d.Add("/jobs/SWE/b")
		d.Add("/jobs/SWE/a")
	}
	if len(batches) != 0 {
		t.Fatalf("delivered before the window closed: %v", batches)
	}
	closeWindow()
	if want := [][]string{{"/jobs/SWE/a", "/jobs/SWE/b"}}; !reflect.DeepEqual(batches, want) {
		t.Fatalf("batches = %v, want %v", batches, want)
	}
	closeWindow = nil
	d.Add("/jobs/SWE/a")
	if closeWindow == nil {
		t.Fatal("a change after delivery did not open a new window")
	}
	closeWindow()
	if len(batches) != 2 || !reflect.DeepEqual(batches[1], []string{"/jobs/SWE/a"}) {
		t.Fatalf("second window = %v", batches)
	}
}

func TestDebouncerOpensOneWindowForABurst(t *testing.T) {
	windows := 0
	d := debounce.New(func(func()) { windows++ }, func([]string) {})
	for i := 0; i < 20; i++ {
		d.Add("/jobs/SWE/a")
	}
	if windows != 1 {
		t.Fatalf("opened %d windows for one burst", windows)
	}
}
