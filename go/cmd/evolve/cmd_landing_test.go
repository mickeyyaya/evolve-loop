package main

import (
	"bytes"
	"errors"
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
	return landingRootAudited(t, status, heldproposal.AuditPass, "claude")
}

func landingRootAudited(t *testing.T, status, verdict, auditFamily string) (root, report string) {
	t.Helper()
	holdLandingCredential(t, true)
	root = t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if status != "" {
		rec := heldproposal.Record{Cycle: 9, Status: status, Commit: "c", Tree: "tree9", Reasons: []string{"kind:pipeline-repair"}, At: time.Now(), DiffSHA256: "diff9", BuilderFamilies: []string{"gemini"}}
		if err := heldproposal.WriteRecord(evolveDir, rec); err != nil {
			t.Fatal(err)
		}
	}
	if verdict != "" {
		audit := heldproposal.AuditReport{Cycle: 9, Tree: "tree9", DiffSHA256: "diff9", Family: auditFamily, CLI: auditFamily + "-tmux", Tier: "deep", Verdict: verdict}
		if err := heldproposal.WriteAuditReport(evolveDir, audit, []byte("# audit\n")); err != nil {
			t.Fatal(err)
		}
	}
	return root, heldproposal.AuditReportPath(evolveDir, 9)
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
	if err != nil || !found || a.Decision != heldproposal.DecisionApprove || a.Tree != "tree9" || a.Audit != heldproposal.AuditReportPath(filepath.Join(root, ".evolve"), 9) {
		t.Errorf("approval = %+v, %v, %v", a, found, err)
	}
	if rec, _, _ := heldproposal.ReadRecord(filepath.Join(root, ".evolve"), 9); rec.Status != heldproposal.StatusApproved {
		t.Errorf("record status = %q, want approved, so wave status stops listing it as held", rec.Status)
	}
	if rc, _, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--audit-report", report, "--operator", "--project-root", root); rc != 1 || !strings.Contains(errs, "no held-for-operator record") {
		t.Errorf("a second approval: rc %d err %q, want refused", rc, errs)
	}
}

func checkApprovalNeedsAPassingAuditFromAnotherModelFamily(t *testing.T) {
	t.Helper()
	absent, _ := landingRootAudited(t, heldproposal.StatusHeld, "", "")
	failed, _ := landingRootAudited(t, heldproposal.StatusHeld, heldproposal.AuditFail, "claude")
	waiting, _ := landingRootAudited(t, heldproposal.StatusHeld, heldproposal.AuditWait, "")
	builder, _ := landingRootAudited(t, heldproposal.StatusHeld, heldproposal.AuditPass, "gemini")
	cases := []struct {
		name, root, want string
		sentinel         error
	}{
		{"an absent report", absent, "no adversarial audit report", heldproposal.ErrAuditMissing},
		{"a FAIL report", failed, "did not pass", heldproposal.ErrAuditNotPass},
		{"a WAIT report", waiting, "did not pass", heldproposal.ErrAuditNotPass},
		{"a report of the builder family", builder, "built the change", heldproposal.ErrAuditBuilderFamily},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rc, _, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--operator", "--project-root", tc.root)
			if rc != 1 || !strings.Contains(errs, tc.want) || !strings.Contains(errs, "refused") {
				t.Errorf("rc %d err %q, want a refusal that names %q", rc, errs, tc.want)
			}
			if _, found, _ := heldproposal.ReadApproval(filepath.Join(tc.root, ".evolve"), 9); found {
				t.Error("a refused approval wrote an approval")
			}
			evolveDir := filepath.Join(tc.root, ".evolve")
			rec, _, _ := heldproposal.ReadRecord(evolveDir, 9)
			report, body, found, _ := heldproposal.ReadAuditReport(evolveDir, 9)
			if err := heldproposal.VerifyAudit(rec, report, found, body); !errors.Is(err, tc.sentinel) {
				t.Errorf("the shared verifier says %v, want %v", err, tc.sentinel)
			}
		})
	}
}

func checkAnAuditReportPathOtherThanTheHoldsOwnIsRefused(t *testing.T) {
	t.Helper()
	corrupt, _ := landingRoot(t, heldproposal.StatusHeld)
	if err := os.WriteFile(heldproposal.AuditReportPath(filepath.Join(corrupt, ".evolve"), 9), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rc, _, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--operator", "--project-root", corrupt); rc != 1 || !strings.Contains(errs, "parse") {
		t.Errorf("a corrupt report: rc %d err %q, want a refusal", rc, errs)
	}
	if rc, _, errs := runLandingArgs("audit", "--cycle", "9", "--project-root", corrupt); rc != 2 || !strings.Contains(errs, "parse") {
		t.Errorf("audit over a corrupt report: rc %d err %q, want a dispatch error", rc, errs)
	}
	root, _ := landingRoot(t, heldproposal.StatusHeld)
	other := filepath.Join(root, "audit.md")
	if err := os.WriteFile(other, []byte("Verdict: PASS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rc, _, errs := runLandingArgs("approve", "--cycle", "9", "--tree", "tree9", "--audit-report", other, "--operator", "--project-root", root)
	if rc != 1 || !strings.Contains(errs, "other than the audit report of the hold") {
		t.Errorf("rc %d err %q, want a refusal of a foreign report path", rc, errs)
	}
}

func checkTheAuditVerbIsGatedByTheCredentialAndTheHold(t *testing.T) {
	t.Helper()
	root, _ := landingRootAudited(t, heldproposal.StatusHeld, "", "")
	none, _ := landingRootAudited(t, "", "", "")
	if rc, _, errs := runLandingArgs("audit", "--cycle", "9", "--project-root", none); rc != 1 || !strings.Contains(errs, "no held-for-operator record") {
		t.Errorf("no hold: rc %d err %q", rc, errs)
	}
	if rc, _, _ := runLandingArgs("audit", "--project-root", root); rc != 10 {
		t.Errorf("no cycle: rc %d, want usage 10", rc)
	}
	rc, _, errs := runLandingArgs("audit", "--cycle", "9", "--project-root", root)
	if rc != 0 {
		t.Fatalf("audit of a hold with no builder attribution: rc %d err %q, want the wait recorded", rc, errs)
	}
	report, _, found, _ := heldproposal.ReadAuditReport(filepath.Join(root, ".evolve"), 9)
	if !found || report.Verdict != heldproposal.AuditWait {
		t.Errorf("report = %+v found %v, want a WAIT report: a hold whose diff file is absent never launches", report, found)
	}
	holdLandingCredential(t, false)
	if rc, _, errs := runLandingArgs("audit", "--cycle", "9", "--project-root", root); rc != 1 || !strings.Contains(errs, "dispatch credential") {
		t.Errorf("no credential: rc %d err %q", rc, errs)
	}
}

func TestLanding_AProcessWithoutTheDispatchCredentialCannotDecide(t *testing.T) {
	t.Run("the audit verb", checkTheAuditVerbIsGatedByTheCredentialAndTheHold)
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
	t.Run("the audit of the hold", checkApprovalNeedsAPassingAuditFromAnotherModelFamily)
	t.Run("a foreign audit report path", checkAnAuditReportPathOtherThanTheHoldsOwnIsRefused)
	held, report := landingRoot(t, heldproposal.StatusHeld)
	shadow, _ := landingRoot(t, heldproposal.StatusWouldHold)
	none, _ := landingRoot(t, "")
	cases := []struct {
		name, root, want string
		args             []string
	}{
		{"no --operator", held, "needs --operator", []string{"approve", "--cycle", "9", "--tree", "tree9", "--audit-report", report}},
		{"a foreign audit report", held, "other than the audit report", []string{"approve", "--cycle", "9", "--tree", "tree9", "--audit-report", filepath.Join(held, "absent.md"), "--operator"}},
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
	for _, args := range [][]string{{}, {"merge"}, {"approve", "--cycle", "0", "--tree", "x"}, {"reject", "--cycle", "9"}, {"list", "--bogus"}, {"approve", "--bogus"}, {"audit"}, {"audit", "--cycle", "9", "--tree", "x"}, {"audit", "--cycle", "9", "--operator"}, {"audit", "--cycle", "9", "extra"}} {
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
