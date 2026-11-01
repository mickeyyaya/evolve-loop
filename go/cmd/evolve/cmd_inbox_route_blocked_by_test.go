package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func blockedByInbox(t *testing.T) string {
	t.Helper()
	t.Setenv("EVOLVE_DISPATCH_DEPTH", "")
	d := t.TempDir()
	inbox := filepath.Join(d, ".evolve", "inbox")
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, body := range map[string]string{
		"blocker": `{"id":"blocker","title":"Fix the blocker","kind":"fix","route":"console-manual"}`,
		"held":    `{"id":"held","title":"Held work","kind":"feature","route":"console-manual"}`,
	} {
		if err := os.WriteFile(filepath.Join(inbox, id+".json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return d
}

func itemDeps(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var it struct {
		Deps []string `json:"deps"`
	}
	if err := json.Unmarshal(raw, &it); err != nil {
		t.Fatal(err)
	}
	return it.Deps
}

func statusIDs(t *testing.T, status string) []string {
	t.Helper()
	var stdout, stderr bytes.Buffer
	if rc := runInbox([]string{"list", "--status", status, "--json"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("inbox list --status %s: rc = %d\nstderr=%s", status, rc, stderr.String())
	}
	var rows []struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rows); err != nil {
		t.Fatalf("inbox list --json: %v\n%s", err, stdout.String())
	}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.Item.ID)
	}
	return ids
}

func TestCmd_InboxRouteLane_BlockedByRecordsTheDepsEdgeAndWakesWhenTheBlockerIsConsumed(t *testing.T) {
	d := blockedByInbox(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	held := filepath.Join(d, ".evolve", "inbox", "held.json")
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"route-lane", "held", "the blocker lands first", "--blocked-by", "blocker"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
	}
	if got := itemDeps(t, held); !slices.Equal(got, []string{"blocker"}) {
		t.Errorf("deps = %v, want [blocker]", got)
	}
	if !strings.Contains(stdout.String(), "lane-dispatchable once blocker is consumed") {
		t.Errorf("stdout must say the item waits on its blocker: %q", stdout.String())
	}
	if got := statusIDs(t, "waiting"); !slices.Contains(got, "held") {
		t.Errorf("waiting = %v; a lane item blocked by a pending item must wait", got)
	}
	if rc := runInbox([]string{"consume", filepath.Join(d, ".evolve", "inbox", "blocker.json"), "--cycle", "console"}, nil, &stdout, &stderr); rc != 0 {
		t.Fatalf("consume blocker: rc = %d\nstderr=%s", rc, stderr.String())
	}
	if got := statusIDs(t, "ready"); !slices.Contains(got, "held") {
		t.Errorf("ready = %v; the held item must be ready once its blocker is consumed, with no manual step", got)
	}
}

func TestCmd_InboxRouteConsole_BlockedByRecordsTheDepsEdgeOnce(t *testing.T) {
	d := blockedByInbox(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	held := filepath.Join(d, ".evolve", "inbox", "held.json")
	for range 2 {
		var stdout, stderr bytes.Buffer
		if rc := runInbox([]string{"route-console", "held", "blocked on the fix", "2046", "--blocked-by", "blocker"}, nil, &stdout, &stderr); rc != 0 {
			t.Fatalf("rc = %d, want 0\nstderr=%s", rc, stderr.String())
		}
	}
	if got := itemDeps(t, held); !slices.Equal(got, []string{"blocker"}) {
		t.Errorf("deps = %v, want [blocker] once: a second route with the same blocker adds no second edge", got)
	}
	if got := readRouted(t, held); got.Route != "console-manual" || got.RoutedReason != "blocked on the fix" {
		t.Errorf("item = %+v; want console-manual with the reason", got)
	}
}

func TestCmd_InboxRoute_BlockedByAnUnknownItemIsRefusedWithItsID(t *testing.T) {
	d := blockedByInbox(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	held := filepath.Join(d, ".evolve", "inbox", "held.json")
	before, err := os.ReadFile(held)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"route-lane", "held", "operator", "--blocked-by", "no-such-blocker"},
		{"route-console", "held", "operator", "7", "--blocked-by", "no-such-blocker"},
	} {
		var stdout, stderr bytes.Buffer
		rc := runInbox(args, nil, &stdout, &stderr)
		if rc != 1 || !strings.Contains(stderr.String(), "no-such-blocker") {
			t.Errorf("%s: rc = %d stderr = %q; want 1 naming the unknown blocker id", args[0], rc, stderr.String())
		}
	}
	if after, _ := os.ReadFile(held); !bytes.Equal(before, after) {
		t.Errorf("a refused route must leave the item unchanged:\n%s", after)
	}
}

func TestCmd_InboxRoute_BlockedByNeedsAnIDOtherThanTheItem(t *testing.T) {
	d := blockedByInbox(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	for _, tc := range []struct {
		args []string
		want int
	}{
		{[]string{"route-lane", "held", "operator", "--blocked-by"}, 10},
		{[]string{"route-lane", "held", "operator", "--blocked-by", "  "}, 10},
		{[]string{"route-console", "held", "operator", "7", "--blocked-by"}, 10},
		{[]string{"route-console", "held", "operator", "7", "--blocked-by", "held"}, 1},
		{[]string{"route-lane", "held", "operator", "--blocked-by", "held"}, 1},
	} {
		var stdout, stderr bytes.Buffer
		if rc := runInbox(tc.args, nil, &stdout, &stderr); rc != tc.want {
			t.Errorf("%v: rc = %d, want %d (stderr=%q)", tc.args, rc, tc.want, stderr.String())
		}
	}
}

func TestCmd_InboxRouteLane_BlockedByBeforeThePositionalsAndInTheJSONDetail(t *testing.T) {
	d := blockedByInbox(t)
	t.Setenv("EVOLVE_PROJECT_ROOT", d)
	var stdout, stderr bytes.Buffer

	rc := runInbox([]string{"route-lane", "--blocked-by", "  blocker  ", "held", "the blocker lands first", "--json"}, nil, &stdout, &stderr)

	if rc != 0 {
		t.Fatalf("rc = %d, want 0 (stderr=%s)", rc, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"detail":"lane-dispatchable once blocker is consumed"`) {
		t.Errorf("json stdout = %s; want the detail with the blocker", stdout.String())
	}
	if got := itemDeps(t, filepath.Join(d, ".evolve", "inbox", "held.json")); !slices.Equal(got, []string{"blocker"}) {
		t.Errorf("deps = %v, want [blocker]", got)
	}
}

func TestCmd_InboxRoute_AQueuedRouteKeepsItsBlocker(t *testing.T) {
	for _, args := range [][]string{
		{"route-lane", "held", "lane may take it", "--blocked-by", "blocker"},
		{"route-console", "held", "operator owns this", "1790", "--blocked-by", "blocker"},
	} {
		d := blockedByInbox(t)
		contractUnitWrite(t, d, ".evolve/policy.json", queueEnforcePolicy)
		liveLaneLease(t, d, 0)
		var stdout, stderr bytes.Buffer

		rc := runInbox(append(append([]string{}, args...), "--project-root", d), nil, &stdout, &stderr)

		if rc != cliExitQueued {
			t.Fatalf("%v: rc = %d, want %d (stderr=%s)", args, rc, cliExitQueued, stderr.String())
		}
		reqs := queuedRequests(t, d)
		if len(reqs) != 1 || !slices.Equal(reqs[0].Args, args[1:]) {
			t.Errorf("%v: queue = %+v; the replay must carry --blocked-by", args, reqs)
		}
	}
}
