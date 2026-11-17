package reporting_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Closed Applications are tallied by close_reason, most first; one that has left Rejected
// or Withdrawn, or has no reason, is not a drop-off.
func TestDropOffByCloseReason(t *testing.T) {
	closed := func(status, reason string) store.Application {
		return store.Application{Status: status, CloseReason: reason, StatusLog: []store.StatusEntry{{Status: store.StatusApplied, At: "2026-01-01"}, {Status: status, At: "2026-02-01"}}}
	}
	apps := []store.Application{
		closed(store.StatusRejected, "no-response"), closed(store.StatusRejected, "no-response"),
		closed(store.StatusWithdrawn, "salary"), closed(store.StatusRejected, "after-screen"),
		closed(store.StatusRejected, ""),
		{Status: store.StatusInterviewing, CloseReason: "no-response", StatusLog: []store.StatusEntry{{Status: store.StatusApplied, At: "2026-01-01"}}},
	}
	r := reporting.Build(apps, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	want := []reporting.DropOff{{Reason: "no-response", Count: 2}, {Reason: "after-screen", Count: 1}, {Reason: "salary", Count: 1}}
	if !reflect.DeepEqual(r.DropOff, want) || r.Unrecorded != 1 {
		t.Fatalf("DropOff = %+v (unrecorded %d), want %+v (1)", r.DropOff, r.Unrecorded, want)
	}
	if none := reporting.Build(apps[4:5], time.Now()); none.DropOff != nil {
		t.Fatalf("no reason recorded, yet DropOff = %+v", none.DropOff)
	}
}
