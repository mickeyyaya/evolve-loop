package lifecycle

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func TestIsMoverWritten_IsTheLoopStampsOnly(t *testing.T) {
	for _, key := range inboxbatch.FieldsOwnedBy(inboxbatch.LoopStampOwned) {
		if !IsMoverWritten(key) {
			t.Errorf("IsMoverWritten(%q) = false, want true: the loop's mover writes it", key)
		}
	}
	if !IsMoverWritten(RouteField) {
		t.Errorf("IsMoverWritten(%q) = false", RouteField)
	}
	for _, key := range append(inboxbatch.FieldsOwnedBy(inboxbatch.OperatorStampOwned), "weight", "summary", "acceptance", "id", "kind", "routed_note") {
		if IsMoverWritten(key) {
			t.Errorf("IsMoverWritten(%q) = true, want false: the loop never writes it", key)
		}
	}
}

func keysChanged(before, after map[string]json.RawMessage) []string {
	var changed []string
	for _, key := range slices.Sorted(maps.Keys(after)) {
		if string(before[key]) != string(after[key]) {
			changed = append(changed, key)
		}
	}
	for key := range before {
		if _, kept := after[key]; !kept {
			changed = append(changed, key)
		}
	}
	return changed
}

func readFields(t *testing.T, path string) map[string]json.RawMessage {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestFieldOwners_EveryKeyALifecycleWriterWritesIsAStampOfItsOwner(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	stamped := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	m := New(inbox, nil, WithNow(func() time.Time { return stamped }), WithMainHead(mainAt(verifiedSHA)))
	for name, tc := range map[string]struct {
		write func() error
		owner inboxbatch.FieldOwner
	}{
		"RouteConsole":     {func() error { _, err := m.RouteConsole("x", "found", 3); return err }, inboxbatch.LoopStampOwned},
		"RouteLane":        {func() error { _, err := m.RouteLane("x", "lane work"); return err }, inboxbatch.LoopStampOwned},
		"BumpFailureCount": {func() error { _, err := BumpFailureCount(path, "red"); return err }, inboxbatch.LoopStampOwned},
		"VerifyPremise":    {func() error { _, err := m.VerifyPremise("x", "checked"); return err }, inboxbatch.OperatorStampOwned},
		"bump and shed":    {func() error { _, err := bumpWith(path, "red", func(int) bool { return true }); return err }, inboxbatch.LoopStampOwned},
	} {
		t.Run(name, func(t *testing.T) {
			writeItem(t, path, `{"id":"x","kind":"bug","weight":0.5,"continuation":{"snapshot_sha":"abc"}}`)
			before := readFields(t, path)

			if err := tc.write(); err != nil {
				t.Fatal(err)
			}

			for _, key := range keysChanged(before, readFields(t, path)) {
				if got := inboxbatch.RoleOf(key).Owner; got != tc.owner {
					t.Errorf("%s writes %q, owned by %s in the table, want %s", name, key, got, tc.owner)
				}
			}
		})
	}
}

func TestFieldOwners_AnUnbackedRecordCarriesOnlyLoopStamps(t *testing.T) {
	inbox := newInbox(t)

	path, err := New(inbox, nil).RetireUnbacked("ghost", "processed", PromoteOpts{Cycle: "9", CommitSHA: "abcdef1234"}, "shipped under another id")

	if err != nil {
		t.Fatal(err)
	}
	for key := range readFields(t, path) {
		if key != "id" && inboxbatch.RoleOf(key).Owner != inboxbatch.LoopStampOwned {
			t.Errorf("RetireUnbacked writes %q, which the table does not own as a loop stamp", key)
		}
	}
}
