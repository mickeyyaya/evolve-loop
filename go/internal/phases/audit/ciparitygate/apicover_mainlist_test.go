package ciparitygate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mainAndLane(t *testing.T, mainList, laneList string) Request {
	t.Helper()
	mainRoot, mainGo := goWorktree(t)
	laneRoot, laneGo := goWorktree(t)
	if mainList != "" {
		if err := os.WriteFile(filepath.Join(mainGo, ".apicover-enforce"), []byte(mainList), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if laneList != "" {
		if err := os.WriteFile(filepath.Join(laneGo, ".apicover-enforce"), []byte(laneList), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Request{Cycle: 3, ProjectRoot: mainRoot, Worktree: laneRoot}
}

func TestApicoverEnforce_ALaneThatDeletesTheEnforceListFails(t *testing.T) {
	g, events := observed(t, fakeRunFunc(0, "", "", nil), fixedSet("./internal/p/..."))

	off, err := g.ApicoverEnforce(mainAndLane(t, "./internal/p\n", ""))

	if err != nil || len(off) != 1 || !strings.Contains(off[0], "deletes go/.apicover-enforce") {
		t.Fatalf("offenders = %v, err = %v: a lane that deletes the list the main plane holds must not pass the gate", off, err)
	}
	if e := only(t, *events, CodeGateFailed); e.Fields["cause"] != string(causeApicover) {
		t.Errorf("event = %+v, want cause apicover", e)
	}
}

func TestApicoverEnforce_NoListInEitherTreeStillSkips(t *testing.T) {
	g, events := observed(t, fakeRunFunc(0, "", "", nil), fixedSet("./internal/p/..."))
	if off, err := g.ApicoverEnforce(mainAndLane(t, "", "")); off != nil || err != nil || len(*events) != 0 {
		t.Fatalf("(%v, %v) events %v: a project without the list has no apicover gate", off, err, codesOf(*events))
	}
}

func TestEnforceList_TheMainListBindsBesideTheLaneList(t *testing.T) {
	req := mainAndLane(t, "./internal/p\n", "./internal/q\n")
	got, missing := enforceList(req, moduleDir(req.root()))
	if missing || !strings.Contains(string(got), "./internal/p") || !strings.Contains(string(got), "./internal/q") {
		t.Fatalf("list = %q missing = %v: the lane cannot drop ./internal/p from the gate", got, missing)
	}
	if got, missing := enforceList(Request{Cycle: 3, ProjectRoot: req.Worktree}, moduleDir(req.Worktree)); missing || strings.TrimSpace(string(got)) != "./internal/q" {
		t.Errorf("list = %q missing = %v: without a worktree the root list is the one list", got, missing)
	}
}
