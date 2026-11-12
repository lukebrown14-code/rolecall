package ui

import "testing"

// Every row action is well-formed: it has keys, both gates and a run, and the key the help
// shows is one the dispatcher can reach — so the help never advertises a key that does
// nothing. The help (actionKeys) and the dispatcher (runListAction) both read listActions,
// so the two cannot drift apart.
func TestListActionsAreSelfConsistent(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range listActions {
		if len(a.keys) == 0 {
			t.Fatalf("%s has no keys", a.keyLabel)
		}
		if a.listed == nil || a.enabled == nil || a.run == nil {
			t.Fatalf("%s is missing a gate or a run", a.keyLabel)
		}
		if press := pressable(a.keyLabel); press != "" {
			reachable := false
			for _, k := range a.keys {
				if k == press {
					reachable = true
				}
			}
			if !reachable {
				t.Fatalf("%s: the help's key %q is not among the dispatcher's %v", a.keyLabel, press, a.keys)
			}
		}
		for _, k := range a.keys {
			if seen[k] {
				t.Fatalf("key %q is claimed by two actions", k)
			}
			seen[k] = true
		}
	}
}
