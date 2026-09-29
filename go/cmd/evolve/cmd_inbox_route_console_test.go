package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type routedFields struct {
	Route        string `json:"route"`
	RoutedReason string `json:"routed_reason"`
	RoutedCycle  int    `json:"routed_cycle"`
}

func readRouted(t *testing.T, path string) routedFields {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f routedFields
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestCmd_InboxRouteConsole_ALaneCanNoLongerClaimTheItem(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"route-console", "task-1", "  protected-surface: go/internal/loopwave/loopwave.go  ", "1757"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	got := readRouted(t, filepath.Join(d, ".evolve", "inbox", "task-1.json"))
	if got != (routedFields{Route: "console-manual", RoutedReason: "protected-surface: go/internal/loopwave/loopwave.go", RoutedCycle: 1757}) {
		t.Errorf("item = %+v; want the console route with the trimmed reason and the cycle", got)
	}
	if !strings.Contains(stdout.String(), "task-1") {
		t.Errorf("stdout names the routed item: %q", stdout.String())
	}
	if rc := runInboxMover([]string{"claim", "task-1", "1758"}, nil, &stdout, &stderr); rc != 3 {
		t.Errorf("claim rc = %d; a console-routed item is refused to every later lane (exit 3)", rc)
	}
}

func TestCmd_InboxRouteConsole_RefusesAMalformedRequest(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"no cycle", []string{"route-console", "task-1", "a reason"}},
		{"a cycle that is not a number", []string{"route-console", "task-1", "a reason", "last"}},
		{"a negative cycle", []string{"route-console", "task-1", "a reason", "-5"}},
		{"a blank reason", []string{"route-console", "task-1", "  ", "1757"}},
		{"a blank id", []string{"route-console", " ", "a reason", "1757"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := setupInbox(t, "task-1")
			t.Setenv("EVOLVE_PROJECT_ROOT", d)
			var stdout, stderr bytes.Buffer

			if rc := runInbox(tc.args, nil, &stdout, &stderr); rc != 10 {
				t.Fatalf("rc = %d, want 10 (usage)\nstderr=%s", rc, stderr.String())
			}
			if got := readRouted(t, filepath.Join(d, ".evolve", "inbox", "task-1.json")); got.Route != "" {
				t.Errorf("a refused request routes nothing: %+v", got)
			}
		})
	}
}

func TestCmd_InboxRouteConsole_RefusesAnItemNoRootHolds(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	claimDir := filepath.Join(d, ".evolve", "inbox", "processing", "cycle-1757")
	if err := os.MkdirAll(claimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claimed := filepath.Join(claimDir, "held.json")
	if err := os.WriteFile(claimed, []byte(`{"id":"held"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"route-console", "held", "a reason", "1757"}, nil, &stdout, &stderr); rc != 1 {
		t.Fatalf("rc = %d; an item under a lane's claim is refused whatever cycle the operator names\nstderr=%s", rc, stderr.String())
	}
	if !strings.Contains(stderr.String(), "held by cycle 1757") {
		t.Errorf("the refusal names the claim: %q", stderr.String())
	}
	if got := readRouted(t, claimed); got.Route != "" {
		t.Errorf("a live claim is never rewritten: %+v", got)
	}
	if rc := runInbox([]string{"route-console", "missing", "a reason", "1757"}, nil, &stdout, &stderr); rc != 1 {
		t.Errorf("an id the inbox does not hold: rc = %d, want 1", rc)
	}
}

func TestCmd_InboxRouteConsole_AFailedRewriteIsExitTwo(t *testing.T) {
	d := setupInbox(t, "task-1")
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	blocker := filepath.Join(d, ".evolve", "inbox", fmt.Sprintf("task-1.json.tmp.%d", os.Getpid()))
	if err := os.Mkdir(blocker, 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"route-console", "task-1", "a reason", "1757"}, nil, &stdout, &stderr); rc != 2 {
		t.Fatalf("rc = %d; a route that could not be written is never reported routed\nstderr=%s", rc, stderr.String())
	}
	if got := readRouted(t, filepath.Join(d, ".evolve", "inbox", "task-1.json")); got.Route != "" {
		t.Errorf("the item is unchanged: %+v", got)
	}
}

func TestCmd_InboxUsageNamesRouteConsole(t *testing.T) {
	var stdout, stderr bytes.Buffer

	runInbox(nil, nil, &stdout, &stderr)

	if !strings.Contains(stderr.String(), "route-console") {
		t.Errorf("usage = %q", stderr.String())
	}
}

func TestCmd_InboxRouteConsole_AnUnreadableInboxIsAFaultNotAnUnknownID(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, ".evolve", "inbox"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer

	if rc := runInbox([]string{"route-console", "task-1", "a reason", "1757"}, nil, &stdout, &stderr); rc != 2 {
		t.Errorf("rc = %d, want 2: an inbox that cannot be read is a fault, not an id it does not hold\nstderr=%s", rc, stderr.String())
	}
}
