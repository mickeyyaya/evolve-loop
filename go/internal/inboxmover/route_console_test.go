package inboxmover

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

func TestRouteConsole_NamesTheFacadeAndRoutesThroughOptions(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbox, "2026-07-30T13-04-00Z-poison.json")
	if err := os.WriteFile(path, []byte(`{"id":"poison"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	c := signalcenter.New()
	var codes []signalcenter.Code
	c.Subscribe(func(e signalcenter.Event) { codes = append(codes, e.Code) })
	res, err := RouteConsole(Options{ProjectRoot: root, Stderr: io.Discard, Signals: c}, "poison", "triage refused", 7)
	if err != nil || res.Path != path {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	var _ RouteResult = res
	body, _ := os.ReadFile(path)
	var item map[string]any
	if err := json.Unmarshal(body, &item); err != nil || item["route"] != "console-manual" || item["routed_reason"] != "triage refused" {
		t.Errorf("item = %v (%v)", item, err)
	}
	if len(codes) != 1 || codes[0] != "INBOX_ITEM_ROUTED_CONSOLE" {
		t.Errorf("codes = %v", codes)
	}
	if _, err := RouteConsole(Options{ProjectRoot: root, Stderr: io.Discard}, "ghost", "r", 7); err == nil {
		t.Error("a missing id surfaces as an error through the facade")
	}
}

func TestRouteRequests_CarryTheBlockerThroughTheFacade(t *testing.T) {
	root := t.TempDir()
	inbox := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]string{"blocker": `{"id":"blocker"}`, "a": `{"id":"a"}`, "b": `{"id":"b"}`} {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	opts := Options{ProjectRoot: root, Stderr: io.Discard}

	_, cerr := RouteConsoleRequest(opts, RouteRequest{TaskID: "a", Reason: "r", Cycle: 7, BlockedBy: "blocker"})
	_, lerr := RouteLaneRequest(opts, RouteRequest{TaskID: "b", Reason: "r", BlockedBy: "blocker"})

	if cerr != nil || lerr != nil {
		t.Fatalf("console err = %v, lane err = %v", cerr, lerr)
	}
	for _, id := range []string{"a", "b"} {
		if d := PendingDispatchability(opts, readDeps(t, filepath.Join(inbox, id+".json"))); d.Dispatchable {
			t.Errorf("%s: dispatchable while its blocker is pending; deps must name the blocker", id)
		}
	}
}

func readDeps(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var it struct {
		Deps []string `json:"deps"`
	}
	if err := json.Unmarshal(body, &it); err != nil {
		t.Fatal(err)
	}
	return it.Deps
}
