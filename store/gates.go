package store

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Gate keys in workspace.json, as build.py reads them.
const (
	GateLetterCap = "letterWordCap"
	GateLetterMin = "letterWordMin"
	GateSkillsMin = "keySkillsMin"
	GateSkillsMax = "keySkillsMax"
)

// SetGate checks a document gate value against the other gates and writes it to
// workspace.json. The value is text as typed: numbers only, cap above 0 and above the
// minimum, Key Skills minimum at most the maximum. A refused value returns the reason and
// writes nothing.
func (s *Store) SetGate(key, text string) error {
	ws, err := s.Settings()
	if err != nil {
		return err
	}
	if ws.Err != nil {
		return fmt.Errorf("%s is not valid JSON, not writing", ws.Path)
	}
	n, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil || n < 0 {
		return errors.New("enter a whole number")
	}
	switch key {
	case GateLetterCap:
		if n <= 0 {
			return errors.New("word cap must be above 0")
		}
		if n <= ws.LetterWordMin {
			return fmt.Errorf("word cap must be above the minimum (%d)", ws.LetterWordMin)
		}
	case GateLetterMin:
		if n >= ws.LetterWordCap {
			return fmt.Errorf("minimum must be below the cap (%d)", ws.LetterWordCap)
		}
	case GateSkillsMin:
		if n > ws.KeySkillsMax {
			return fmt.Errorf("Key Skills minimum must be at most the maximum (%d)", ws.KeySkillsMax)
		}
	case GateSkillsMax:
		if n < ws.KeySkillsMin {
			return fmt.Errorf("Key Skills maximum must be at least the minimum (%d)", ws.KeySkillsMin)
		}
	default:
		return fmt.Errorf("unknown gate %q", key)
	}
	return s.setWorkspaceKey(key, []byte(strconv.Itoa(n)))
}
