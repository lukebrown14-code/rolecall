package ui

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// A terminal that never answers the colour request releases the start-up after the cap,
// not after lipgloss's two seconds.
func TestBackgroundQueryGivesUpQuickly(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	start := time.Now()
	err = queryTerminal(r, io.Discard, backgroundQueryWait, func(string, *ansi.Parser) bool { return true }, "q")
	if err == nil {
		t.Fatal("silent terminal: want a cancelled-read error")
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("waited %v for a terminal that never answers; cap is %v", took, backgroundQueryWait)
	}
}

// A terminal that answers gets its colour read: the reply ends at the DA1 answer.
func TestBackgroundQueryReadsAnAnswer(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	if _, err := w.WriteString("\x1b]11;rgb:ffff/ffff/ffff\x1b\\\x1b[?62;c"); err != nil {
		t.Fatal(err)
	}
	var got string
	err = queryTerminal(r, io.Discard, backgroundQueryWait, func(seq string, pa *ansi.Parser) bool {
		switch {
		case ansi.HasOscPrefix(seq) && pa.Command() == 11:
			got = string(pa.Data())
		case ansi.HasCsiPrefix(seq) && pa.Command() == ansi.Command('?', 0, 'c'):
			return false
		}
		return true
	}, "q")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "ffff/ffff/ffff") {
		t.Fatalf("colour reply = %q", got)
	}
}
