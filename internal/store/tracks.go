package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SetTracks writes tracks to workspace.json. Only the tracks key changes: every other key
// keeps its value and its place, and the file is re-indented with 2 spaces. A missing file
// is created; an invalid one is never written over.
func (s *Store) SetTracks(tracks []string) error {
	if tracks == nil {
		tracks = []string{}
	}
	value, err := json.Marshal(tracks)
	if err != nil {
		return err
	}
	return s.setWorkspaceKey("tracks", value)
}

// setWorkspaceKey replaces one top-level key of workspace.json, appending it when absent.
func (s *Store) setWorkspaceKey(key string, value json.RawMessage) error {
	path := filepath.Join(s.root, "workspace.json")
	mode := os.FileMode(privateFileMode)
	var keys []string
	values := map[string]json.RawMessage{}
	switch b, err := os.ReadFile(path); {
	case os.IsNotExist(err):
	case err != nil:
		return err
	default:
		if info, e := os.Stat(path); e == nil {
			mode = info.Mode().Perm()
		}
		if keys, values, err = orderedObject(b); err != nil {
			return fmt.Errorf("%s is not valid JSON, not writing: %w", path, err)
		}
	}
	// A schema this build does not know is never written over: the Owner may be on a
	// newer rolecall, and rewriting would drop keys it added.
	if raw, ok := values["schema"]; ok {
		if _, err := workspaceSchemaVersion(path, raw); err != nil {
			return err
		}
	}
	if _, ok := values[key]; !ok {
		keys = append(keys, key)
	}
	values[key] = value
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
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, out.Bytes(), "", "  "); err != nil {
		return err
	}
	pretty.WriteByte('\n')
	return s.atomicWrite(path, pretty.Bytes(), mode)
}

// orderedObject parses a JSON object keeping the order of its keys.
func orderedObject(b []byte) ([]string, map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		if err == nil {
			err = errors.New("top level is not an object")
		}
		return nil, nil, err
	}
	var keys []string
	values := map[string]json.RawMessage{}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		k, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, nil, err
		}
		if _, dup := values[k]; !dup {
			keys = append(keys, k)
		}
		values[k] = v
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, nil, errors.New("trailing data")
	}
	return keys, values, nil
}
