package lifecycle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func neverProtected(string) bool { return false }

func TestMover_RouteLane_OpensAPipelineRepairItemToLanes(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "2026-09-30T00-00-00Z-grep.json")
	writeItem(t, path, `{"id":"grep","kind":"pipeline-repair","files":["go/internal/cycleclassify/classify.go"],"route":"console-manual","routed_cycle":1778}`)
	rec := &recordingAppender{}
	var stderr strings.Builder
	fixed := time.Date(2026, 9, 30, 8, 0, 0, 0, time.UTC)
	m := New(inbox, rec, WithStderr(&stderr), WithNow(func() time.Time { return fixed }))

	res, err := m.RouteLane("grep", "operator: batch pipeline repairs through the loop")

	if err != nil || res.Path != path {
		t.Fatalf("RouteLane = (%+v, %v)", res, err)
	}
	item := readItem(t, path)
	if item["route"] != "lane" || item["routed_reason"] != "operator: batch pipeline repairs through the loop" ||
		item["routed_at"] != "2026-09-30T08:00:00Z" || item["kind"] != "pipeline-repair" {
		t.Errorf("item = %v", item)
	}
	if _, stale := item["routed_cycle"]; stale {
		t.Errorf("item = %v: a console route's cycle must not survive the lane route", item)
	}
	loaded, _, err := inboxbatch.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if routed, why := inboxbatch.ConsoleRouted(loaded, neverProtected); routed {
		t.Errorf("ConsoleRouted = (true, %q), want the loop's lanes to take the item", why)
	}
	if len(rec.records) != 1 || rec.records[0].Action != "route-lane" || rec.records[0].TaskID != "grep" ||
		!strings.Contains(rec.records[0].Message, "batch pipeline repairs") {
		t.Errorf("ledger = %+v", rec.records)
	}
}

func TestMover_RouteLane_RefusesWhatALaneMayNotTake(t *testing.T) {
	for name, tc := range map[string]struct {
		body, why   string
		isProtected func(string) bool
	}{
		"a declared protected file": {
			`{"id":"x","kind":"bug","files":["go/internal/guards/phase.go"]}`,
			"protected fix surface: go/internal/guards/phase.go",
			func(p string) bool { return p == "go/internal/guards/phase.go" },
		},
		"an agent-autofiled pipeline item": {
			`{"id":"x","kind":"pipeline-repair","injected_by":"loop-halt"}`,
			"agent-autofiled",
			neverProtected,
		},
		"a protected file past the loader's field cut": {
			`{"id":"x","kind":"pipeline-repair","files":["` + strings.Repeat("go/internal/cycleclassify/classify.go ", 5) + `go/internal/guards/phase.go"]}`,
			"protected fix surface: go/internal/guards/phase.go",
			func(p string) bool { return p == "go/internal/guards/phase.go" },
		},
	} {
		t.Run(name, func(t *testing.T) {
			inbox := newInbox(t)
			path := filepath.Join(inbox, "2026-09-30T00-00-00Z-x.json")
			writeItem(t, path, tc.body)
			before, _ := os.ReadFile(path)
			rec := &recordingAppender{}

			_, err := New(inbox, rec, WithProtectedPath(tc.isProtected)).RouteLane("x", "operator")

			if !errors.Is(err, ErrConsoleRouted) || !strings.Contains(err.Error(), tc.why) {
				t.Errorf("err = %v, want ErrConsoleRouted naming %q", err, tc.why)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
				t.Errorf("a refused route rewrote the item:\n%s", after)
			}
			if len(rec.records) != 0 {
				t.Errorf("ledger = %+v, want nothing recorded for a refusal", rec.records)
			}
		})
	}
}

func TestMover_RouteLane_LeavesAClaimedOrMissingItemAlone(t *testing.T) {
	inbox := newInbox(t)
	claimed := procPath(inbox, 1780, "held.json")
	writeItem(t, claimed, `{"id":"held","kind":"pipeline-repair"}`)
	m := New(inbox, nil)

	if _, err := m.RouteLane("held", "operator"); !errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), "1780") {
		t.Errorf("claimed: err = %v, want ErrNotFound naming the holding cycle", err)
	}
	if item := readItem(t, claimed); item["route"] != nil {
		t.Errorf("claimed item = %v, want it untouched", item)
	}
	if _, err := m.RouteLane("absent", "operator"); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing: err = %v, want ErrNotFound", err)
	}
	if _, err := m.RouteLane("", "operator"); !errors.Is(err, ErrBadArgs) {
		t.Errorf("empty id: err = %v, want ErrBadArgs", err)
	}
}

func TestMover_RouteLane_AnUnreadableInboxIsNotAMissingItem(t *testing.T) {
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "x.json"), `{"id":"x","kind":"pipeline-repair"}`)
	if err := os.Chmod(inbox, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, nil).RouteLane("x", "operator")

	if err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want the read fault itself, not ErrNotFound", err)
	}
}

func TestMover_RouteLane_RefusesAnItemItCannotJudge(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x","kind":"pipeline-repair","files":"go/internal/guards/phase.go"}`)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{}

	_, err := New(inbox, rec, WithProtectedPath(func(p string) bool { return p == "go/internal/guards/phase.go" })).RouteLane("x", "operator")

	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConsoleRouted) {
		t.Errorf("err = %v, want a malformed-item fault: an unjudged item must not be opened to lanes", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("an unjudged item was rewritten or recorded:\n%s\nledger=%+v", after, rec.records)
	}
}

func TestMover_RouteLane_ARewriteFaultLeavesTheItemAndTheLedgerAlone(t *testing.T) {
	inbox := newInbox(t)
	path := filepath.Join(inbox, "x.json")
	writeItem(t, path, `{"id":"x","kind":"pipeline-repair"}`)
	before, _ := os.ReadFile(path)
	rec := &recordingAppender{}
	if err := os.Chmod(inbox, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(inbox, 0o755) })

	_, err := New(inbox, rec).RouteLane("x", "operator")

	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrConsoleRouted) {
		t.Errorf("err = %v, want the rewrite fault itself", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("a failed rewrite changed the item or recorded a route:\n%s\nledger=%+v", after, rec.records)
	}
}
