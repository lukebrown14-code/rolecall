package ui

import (
	"fmt"
	"image/color"
	"io"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

// backgroundQueryWait is how long the start-up waits for a terminal to answer the
// background-colour (OSC 11) and device-attributes (DA1) request. A terminal that
// supports them answers within a few milliseconds; one that does not never will, and
// lipgloss's own wait of 2 s, twice, left a blank screen. The attempt is made on stdin
// and then stdout, each waiting this long at most.
const backgroundQueryWait = 200 * time.Millisecond

// oscBackground is the OSC command a terminal answers the background-colour request with,
// and oscReplyParts the parts of its data ("11;rgb:…").
const (
	oscBackground = 11
	oscReplyParts = 2
)

// terminalBackground asks the terminal for its background colour, as
// lipgloss.BackgroundColor does but giving up after backgroundQueryWait on each file.
// One of in and out must be a terminal.
func terminalBackground(in, out term.File) (color.Color, error) {
	if !term.IsTerminal(in.Fd()) || !term.IsTerminal(out.Fd()) {
		return nil, fmt.Errorf("input/output is not a terminal")
	}
	var (
		bg  color.Color
		err error
	)
	for _, f := range []term.File{in, out} {
		if bg, err = queryBackground(f, backgroundQueryWait); err == nil {
			return bg, nil
		}
	}
	return bg, err
}

// queryBackground puts f in raw mode and asks for the background colour and the device
// attributes: the second answer ends the wait, so a terminal that lacks the colour
// request but has attributes answers at once.
func queryBackground(f term.File, wait time.Duration) (color.Color, error) {
	state, err := term.MakeRaw(f.Fd())
	if err != nil {
		return nil, fmt.Errorf("error setting raw state to detect background color: %w", err)
	}
	defer term.Restore(f.Fd(), state) //nolint:errcheck
	var c color.Color
	err = queryTerminal(f, f, wait, func(seq string, pa *ansi.Parser) bool {
		switch {
		case ansi.HasOscPrefix(seq):
			if pa.Command() == oscBackground {
				if parts := strings.Split(string(pa.Data()), ";"); len(parts) == oscReplyParts {
					c = ansi.XParseColor(parts[1])
				}
			}
		case ansi.HasCsiPrefix(seq):
			if pa.Command() == ansi.Command('?', 0, 'c') { // DA1
				return false
			}
		}
		return true
	}, ansi.RequestBackgroundColor+ansi.RequestPrimaryDeviceAttributes)
	return c, err
}

// queryTerminal writes query and feeds each reply sequence to filter until it returns
// false or the wait runs out.
func queryTerminal(in io.Reader, out io.Writer, wait time.Duration, filter func(string, *ansi.Parser) bool, query string) error {
	rd, err := uv.NewCancelReader(in)
	if err != nil {
		return fmt.Errorf("could not create cancel reader: %w", err)
	}
	defer rd.Close() //nolint:errcheck

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-done:
		case <-time.After(wait):
			rd.Cancel()
		}
	}()
	if _, err := io.WriteString(out, query); err != nil {
		return fmt.Errorf("could not write query: %w", err)
	}
	pa := ansi.GetParser()
	defer ansi.PutParser(pa)
	var acc []byte
	var buf [256]byte
	var state byte
	for {
		n, err := rd.Read(buf[:])
		if err != nil {
			return fmt.Errorf("could not read from input: %w", err)
		}
		p := buf[:]
		for n > 0 {
			seq, _, read, newState := ansi.DecodeSequence(p[:n], state, pa)
			acc = append(acc, seq...)
			if newState == ansi.NormalState {
				if !filter(string(acc), pa) {
					return nil
				}
				acc = acc[:0]
			}
			state = newState
			n -= read
			p = p[read:]
		}
	}
}
