//go:build integration

package ship

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestShipError_AuditBindingHeadMoved(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	seedAudit(t, repo, "PASS", map[string]string{
		"head": "0000000000000000000000000000000000000000",
	})
	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: head moved"})

	se := wantShipErr(t, err, core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition, "HEAD has moved")
	if se.Stage != core.StageVerifyClass {
		t.Errorf("want Stage=verify-class, got %s", se.Stage)
	}
	if se.Debug["audited"] == "" || se.Debug["current"] == "" {
		t.Errorf("Debug must carry audited+current HEADs; got %v", se.Debug)
	}
	if res.ExitCode != ExitFailure {
		t.Errorf("precondition class → ExitFailure; got %d", res.ExitCode)
	}
}

func TestShipError_EGPSRedCount(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\negps red change\n")
	seedAudit(t, repo, "PASS")
	acsPath := filepath.Join(repo, ".evolve", "runs", "cycle-1", "acs-verdict.json")
	raw, marshalErr := json.Marshal(predicateVerdictFixture(1, 3, 1, 0))
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	mustWrite(t, acsPath, string(raw))

	res, err := runShip(t, repo, Options{Class: ClassCycle, CommitMessage: "feat: egps red"})

	se := wantShipErr(t, err, core.CodeEGPSRedCount, core.ShipClassPrecondition, "RED predicate")
	if !strings.Contains(se.Debug["red_ids"], "predicate-3") {
		t.Errorf("Debug.red_ids must name the failing predicate; got %v", se.Debug)
	}
	if res.ExitCode != ExitFailure {
		t.Errorf("EGPS red is precondition → ExitFailure; got %d", res.ExitCode)
	}
}

func TestShipError_GitPushRejected(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	mustWrite(t, filepath.Join(repo, "fixture.txt"), "fixture line 1\npush reject change\n")
	seedAudit(t, repo, "PASS")
	res, err := runShip(t, repo, Options{
		Class:         ClassCycle,
		CommitMessage: "feat: push rejected",
		Runner:        faultRunner("git push", 1, nil),
	})

	se := wantShipErr(t, err, core.CodeGitPushRejected, core.ShipClassTransient, "push failed")
	if se.Stage != core.StageAtomicShip {
		t.Errorf("want Stage=atomic-ship, got %s", se.Stage)
	}
	if se.Debug["git_rc"] != "1" {
		t.Errorf("Debug.git_rc must record the push exit code; got %v", se.Debug)
	}
	if res.ExitCode != ExitFailure {
		t.Errorf("transient class → ExitFailure; got %d", res.ExitCode)
	}
}

func TestShipError_DebugStringDeterministic(t *testing.T) {
	t.Parallel()
	se := core.NewShipError(core.CodeAuditBindingHeadMoved, core.ShipClassPrecondition,
		core.StageVerifyClass, "msg", "current", "abc", "audited", "def")
	got := se.DebugString()
	if got != "audited=def; current=abc" {
		t.Errorf("DebugString not deterministic/sorted; got %q", got)
	}
}
