package lifecycle

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func blockedInbox(t *testing.T) (string, string, *recordingAppender, *Mover) {
	t.Helper()
	inbox := newInbox(t)
	writeItem(t, filepath.Join(inbox, "blocker.json"), `{"id":"blocker","kind":"fix"}`)
	held := filepath.Join(inbox, "held.json")
	writeItem(t, held, `{"id":"held","kind":"feature","deps":["earlier"]}`)
	writeItem(t, filepath.Join(inbox, "consumed", "earlier.json"), `{"id":"earlier","kind":"fix"}`)
	rec := &recordingAppender{}
	return inbox, held, rec, New(inbox, rec)
}

func depsOf(t *testing.T, path string) []any {
	t.Helper()
	deps, _ := readItem(t, path)["deps"].([]any)
	return deps
}

func TestMover_RouteRequest_BlockedByAddsTheDepsEdgeInTheRouteRewrite(t *testing.T) {
	for name, route := range map[string]func(*Mover, RouteRequest) (RouteResult, error){
		"console": (*Mover).RouteConsoleRequest,
		"lane":    (*Mover).RouteLaneRequest,
	} {
		t.Run(name, func(t *testing.T) {
			_, held, rec, m := blockedInbox(t)

			for range 2 {
				if _, err := route(m, RouteRequest{TaskID: "held", Reason: "waits on blocker", BlockedBy: "blocker"}); err != nil {
					t.Fatalf("route: %v", err)
				}
			}

			if got, want := depsOf(t, held), []any{"earlier", "blocker"}; !reflect.DeepEqual(got, want) {
				t.Errorf("deps = %v, want %v: the edge is added once and earlier deps stay", got, want)
			}
			if len(rec.records) != 2 || rec.records[0].Action != "route-"+name {
				t.Errorf("ledger = %+v; each route writes one route line and no separate edit line", rec.records)
			}
		})
	}
}

func TestMover_RouteRequest_RefusesABadBlockerAndLeavesTheItemUnchanged(t *testing.T) {
	for _, blocker := range []string{"no-such-item", "held"} {
		for name, route := range map[string]func(*Mover, RouteRequest) (RouteResult, error){
			"console": (*Mover).RouteConsoleRequest,
			"lane":    (*Mover).RouteLaneRequest,
		} {
			_, held, rec, m := blockedInbox(t)
			before, _ := os.ReadFile(held)

			_, err := route(m, RouteRequest{TaskID: "held", Reason: "r", BlockedBy: blocker})

			if !errors.Is(err, ErrInvalidItem) || !bytes.Contains([]byte(err.Error()), []byte(`"`+blocker+`"`)) {
				t.Errorf("%s --blocked-by %s: err = %v, want ErrInvalidItem naming the id", name, blocker, err)
			}
			if after, _ := os.ReadFile(held); !bytes.Equal(before, after) || len(rec.records) != 0 {
				t.Errorf("%s --blocked-by %s: a refused route changed the item or the ledger:\n%s\n%+v", name, blocker, after, rec.records)
			}
		}
	}
}

func TestMover_RouteLaneRequest_ARefusedAdmissionWritesNoEdge(t *testing.T) {
	inbox, _, rec, _ := blockedInbox(t)
	auto := filepath.Join(inbox, "auto.json")
	writeItem(t, auto, `{"id":"auto","kind":"pipeline-repair","injected_by":"loop-halt"}`)
	before, _ := os.ReadFile(auto)
	m := New(inbox, rec, WithProtectedPath(neverProtected))

	_, err := m.RouteLaneRequest(RouteRequest{TaskID: "auto", Reason: "r", BlockedBy: "blocker"})

	if !errors.Is(err, ErrConsoleRouted) {
		t.Fatalf("err = %v, want ErrConsoleRouted", err)
	}
	if after, _ := os.ReadFile(auto); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("a refused lane route must write no deps edge:\n%s", after)
	}
}

func TestMover_RouteConsoleRequest_AMalformedDepsListIsRefused(t *testing.T) {
	inbox, _, rec, _ := blockedInbox(t)
	var stderr bytes.Buffer
	m := New(inbox, rec, WithStderr(&stderr))
	odd := filepath.Join(inbox, "odd.json")
	writeItem(t, odd, `{"id":"odd","kind":"fix","deps":"blocker"}`)

	_, err := m.RouteConsoleRequest(RouteRequest{TaskID: "odd", Reason: "r", BlockedBy: "blocker"})

	if !errors.Is(err, ErrInvalidItem) || len(rec.records) != 0 || stderr.Len() != 0 {
		t.Errorf("err = %v ledger = %+v stderr = %q; a deps value that is not a list must refuse the edge with no rewrite-fault warning", err, rec.records, stderr.String())
	}
}

func TestMover_RouteRequest_AScanFaultIsNotARefusal(t *testing.T) {
	inbox, held, rec, m := blockedInbox(t)
	locked := filepath.Join(inbox, "consumed")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	before, _ := os.ReadFile(held)

	_, err := m.RouteLaneRequest(RouteRequest{TaskID: "held", Reason: "r", BlockedBy: "blocker"})

	if err == nil || errors.Is(err, ErrInvalidItem) || !bytes.Contains([]byte(err.Error()), []byte("blocked-by: scan")) {
		t.Errorf("err = %v; an unreadable inbox tree is a scan fault, not a refusal of the blocker", err)
	}
	if after, _ := os.ReadFile(held); !bytes.Equal(before, after) || len(rec.records) != 0 {
		t.Errorf("a scan fault must change nothing:\n%s", after)
	}
}
