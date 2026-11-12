package store_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The Application JSON shape is the contract between the Go store and the Python
// reader. Both sides assert the same fixture, so a field added on one side without the
// other is caught here.
func TestApplicationKeysMatchTheSharedContract(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "application-keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Data, GoOnly, ScanOnly []string
	}
	if err := json.Unmarshal(b, &contract); err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(store.Application{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(fields))
	for key := range fields {
		got = append(got, key)
	}

	want := append(append([]string{}, contract.Data...), contract.GoOnly...)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Application JSON keys = %v, want the shared contract %v", got, want)
	}
}
