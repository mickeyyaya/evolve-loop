package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/heldproposal"
)

func holdLandingCredential(t *testing.T, held bool) {
	t.Helper()
	saved := landingCredential
	t.Cleanup(func() { landingCredential = saved })
	dir := filepath.Join(t.TempDir(), "dispatch")
	landingCredential = func() (dispatchcred.Store, error) { return dispatchcred.Store{Dir: dir}, nil }
	if !held {
		landingCredential = func() (dispatchcred.Store, error) { return dispatchcred.Store{}, os.ErrPermission }
	}
}

func landingRoot(t *testing.T, status string) (root, report string) {
	t.Helper()
	holdLandingCredential(t, true)
	root = t.TempDir()
	if status != "" {
		rec := heldproposal.Record{Cycle: 9, Status: status, Commit: "c", Tree: "tree9", Reasons: []string{"kind:pipeline-repair"}, At: time.Now()}
		if err := heldproposal.WriteRecord(filepath.Join(root, ".evolve"), rec); err != nil {
			t.Fatal(err)
		}
	}
	report = filepath.Join(root, "audit.md")
	if err := os.WriteFile(report, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, report
}

func runLandingArgs(args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	rc := runLanding(args, nil, &out, &errb)
	return rc, out.String(), errb.String()
}

func TestLanding_TheOperatorApprovesTheHeldTree(t *testing.T) {
	root, report := landingRoot(t, heldproposal.StatusHeld)

	rc, out, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--audit-report", report, "--operator", "--project-root", root)

	if rc != 0 || !strings.Contains(out, "approve recorded") {
		t.Fatalf("rc %d out %q err %q", rc, out, errs)
	}
	a, found, err := heldproposal.ReadApproval(filepath.Join(root, ".evolve"), 9)
	if err != nil || !found || a.Decision != heldproposal.DecisionApprove || a.Tree != "tree9" || a.Audit != report {
		t.Errorf("approval = %+v, %v, %v", a, found, err)
	}
	if rec, _, _ := heldproposal.ReadRecord(filepath.Join(root, ".evolve"), 9); rec.Status != heldproposal.StatusApproved {
		t.Errorf("record status = %q, want approved, so wave status stops listing it as held", rec.Status)
	}
	if rc, _, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--audit-report", report, "--operator", "--project-root", root); rc != 1 || !strings.Contains(errs, "no held-for-operator record") {
		t.Errorf("a second approval: rc %d err %q, want refused", rc, errs)
	}
}

func TestLanding_AProcessWithoutTheDispatchCredentialCannotDecide(t *testing.T) {
	root, report := landingRoot(t, heldproposal.StatusHeld)
	holdLandingCredential(t, false)

	rc, _, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--audit-report", report, "--operator", "--project-root", root)

	if rc != 1 || !strings.Contains(errs, "dispatch credential") {
		t.Errorf("rc %d err %q, want a refusal that names the credential", rc, errs)
	}
	if _, found, _ := heldproposal.ReadApproval(filepath.Join(root, ".evolve"), 9); found {
		t.Error("a process with no credential wrote an approval")
	}
}

func TestLanding_TheOperatorRejectsTheHeldTree(t *testing.T) {
	root, _ := landingRoot(t, heldproposal.StatusHeld)

	rc, _, errs := runLandingArgs("reject", "--cycle", "9", "--tree", "tree9", "--operator", "--project-root", root)

	a, _, _ := heldproposal.ReadApproval(filepath.Join(root, ".evolve"), 9)
	if rec, _, _ := heldproposal.ReadRecord(filepath.Join(root, ".evolve"), 9); rec.Status != heldproposal.StatusRejected {
		t.Errorf("record status = %q, want rejected", rec.Status)
	}
	if rc != 0 || a.Decision != heldproposal.DecisionReject {
		t.Errorf("rc %d err %q approval %+v, want a rejection", rc, errs, a)
	}
}

func TestLanding_EachRefusalWritesNoApproval(t *testing.T) {
	held, report := landingRoot(t, heldproposal.StatusHeld)
	shadow, _ := landingRoot(t, heldproposal.StatusWouldHold)
	none, _ := landingRoot(t, "")
	cases := []struct {
		name, root, want string
		args             []string
	}{
		{"no --operator", held, "needs --operator", []string{"approve", "--cycle", "9", "--tree", "tree9", "--audit-report", report}},
		{"no audit report", held, "needs --audit-report", []string{"approve", "--cycle", "9", "--tree", "tree9", "--operator"}},
		{"an absent audit report", held, "the audit report", []string{"approve", "--cycle", "9", "--tree", "tree9", "--audit-report", filepath.Join(held, "absent.md"), "--operator"}},
		{"an audit report that is a directory", held, "is a directory", []string{"approve", "--cycle", "9", "--tree", "tree9", "--audit-report", held, "--operator"}},
		{"a later tree", held, "tree differs", []string{"approve", "--cycle", "9", "--tree", "tree10", "--audit-report", report, "--operator"}},
		{"a shadow record", shadow, "no held-for-operator record", []string{"reject", "--cycle", "9", "--tree", "tree9", "--operator"}},
		{"no record", none, "no held-for-operator record", []string{"reject", "--cycle", "9", "--tree", "tree9", "--operator"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc, _, errs := runLandingArgs(append(tc.args, "--project-root", tc.root)...)
			if rc != 1 || !strings.Contains(errs, tc.want) {
				t.Errorf("rc %d err %q, want 1 and %q", rc, errs, tc.want)
			}
			if _, found, _ := heldproposal.ReadApproval(filepath.Join(tc.root, ".evolve"), 9); found {
				t.Error("a refused decision wrote an approval")
			}
		})
	}
}

func TestLanding_ABrokenRecordAndAFailedWriteAreErrors(t *testing.T) {
	holdLandingCredential(t, true)
	broken := t.TempDir()
	if err := os.MkdirAll(heldproposal.HeldDir(filepath.Join(broken, ".evolve")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(heldproposal.HeldDir(filepath.Join(broken, ".evolve")), "cycle-9.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, errs := runLandingArgs("reject", "--cycle", "9", "--tree", "tree9", "--operator", "--project-root", broken); rc != 1 || !strings.Contains(errs, "parse") {
		t.Errorf("broken record: rc %d err %q", rc, errs)
	}
	if rc, _, errs := runLandingArgs("list", "--project-root", broken); rc != 2 || !strings.Contains(errs, "parse") {
		t.Errorf("list over a broken record: rc %d err %q", rc, errs)
	}
	blocked, _ := landingRoot(t, heldproposal.StatusHeld)
	if err := os.MkdirAll(filepath.Join(heldproposal.ApprovalsDir(filepath.Join(blocked, ".evolve")), "cycle-9.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if rc, _, errs := runLandingArgs("reject", "--cycle", "9", "--tree", "tree9", "--operator", "--project-root", blocked); rc != 2 || !strings.Contains(errs, "write") {
		t.Errorf("blocked write: rc %d err %q", rc, errs)
	}
}

func TestLanding_ListShowsEachHeldCycle(t *testing.T) {
	root, _ := landingRoot(t, heldproposal.StatusHeld)
	rc, out, _ := runLandingArgs("list", "--project-root", root)
	if rc != 0 || !strings.Contains(out, "cycle 9  held-for-operator  tree tree9  kind:pipeline-repair") {
		t.Errorf("rc %d out %q", rc, out)
	}
	empty, _ := landingRoot(t, "")
	if rc, out, _ := runLandingArgs("list", "--project-root", empty); rc != 0 || !strings.Contains(out, "no held proposals") {
		t.Errorf("empty list: rc %d out %q", rc, out)
	}
}

func TestLanding_TheProjectRootDefaultsToTheEnvironmentRoot(t *testing.T) {
	root, _ := landingRoot(t, heldproposal.StatusHeld)
	t.Setenv("EVOLVE_PROJECT_ROOT", root)
	if rc, out, _ := runLandingArgs("list"); rc != 0 || !strings.Contains(out, "cycle 9") {
		t.Errorf("rc %d out %q", rc, out)
	}
}

func TestLanding_UsageErrors(t *testing.T) {
	t.Parallel()
	for _, args := range [][]string{{}, {"merge"}, {"approve", "--cycle", "0", "--tree", "x"}, {"reject", "--cycle", "9"}, {"list", "--bogus"}, {"approve", "--bogus"}} {
		if rc, _, errs := runLandingArgs(args...); rc != 10 || errs == "" {
			t.Errorf("args %v: rc %d err %q, want usage 10", args, rc, errs)
		}
	}
}

func TestLanding_TheApprovalsLiveOnTheFence(t *testing.T) {
	t.Parallel()
	for _, p := range []string{".evolve/landing/approvals/cycle-9.json", ".evolve/landing/held/cycle-9.json", "go/internal/heldproposal/heldproposal.go", "go/cmd/evolve/cmd_landing.go", "go/internal/phases/ship/held_proposal_gate.go"} {
		if !guards.IsProtectedSurface(p) {
			t.Errorf("%s is not on the fence; a lane could write or weaken the approval of its own held proposal", p)
		}
	}
}
