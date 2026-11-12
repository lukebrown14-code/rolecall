package store

import (
	"encoding/json"
	"fmt"
)

// The Remote rules of search_area.
const (
	RemoteInPlaces = "in-places"
	RemoteAnywhere = "anywhere"
	RemoteNo       = "no"
)

// SetSearchArea writes the search_area key of workspace.json: where roles must be. Every
// other key keeps its value and place; an invalid file is never written over. remote is
// one of in-places, anywhere or no.
func (s *Store) SetSearchArea(places []string, remote string) error {
	switch remote {
	case RemoteInPlaces, RemoteAnywhere, RemoteNo:
	default:
		return fmt.Errorf("remote must be in-places, anywhere or no, not %q", remote)
	}
	if places == nil {
		places = []string{}
	}
	value, err := json.Marshal(struct {
		Places []string `json:"places"`
		Remote string   `json:"remote"`
	}{places, remote})
	if err != nil {
		return err
	}
	return s.setWorkspaceKey("search_area", value)
}
