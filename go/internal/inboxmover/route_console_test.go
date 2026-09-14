package inboxmover

// route_console_test.go — the host facade over lifecycle.RouteConsole
// (apicover): the FAIL closeout reaches the breaker through the same Options
// every other lifecycle call resolves (ProjectRoot → inbox dir, Signals →
// the leaf's accessor, Ledger → the chained appender).

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
	var _ RouteResult = res // the host re-exports the leaf's receipt type
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
