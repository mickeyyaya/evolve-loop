package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	tSkillsDrift1828 = "skill projection drift: 30 artifact(s) stale vs their SSOTs (SKILL.md phase-facts and/or commands/ stubs) — CI TestSkills_NoDrift would FAIL. Run `evolve skills generate`. Drifted: commands/adversarial-testing.md, commands/audit.md, commands/build.md"
	tConflict1828    = "verdict-conflict: auditor narrative=PASS but 1 deterministic gate(s) forced FAIL [skills-drift] — the gate outranks the narrative (ship policy unchanged); both readings are recorded so the disagreement is weighable. Gate detail is in the error diagnostics beside this one."
	tGofmtDirty      = "gofmt: 1 file(s) are not gofmt -s clean — CI `vet + fmt` would FAIL. Run `gofmt -w -s .` in go/. Offenders: internal/x/x.go"
	tEGPSRed         = "EGPS: red_count=2 [TestRetryEnvelope_Bound TestSeal_Order] (cycle ships only when red_count==0)"
	tCLIWall         = "all CLI families exhausted; systemic infrastructure teardown"
	tInfraDecision   = `{"category":"infra-systemic","level":"system","evidence":"prose-declared systemic infrastructure failure","action":"halt-and-diagnose","fix_type":"pipeline-repair"}`

	tSolutionViolated  = "solution contract: 1 violation(s) in the document deliverable — fix these exactly (self-check: `evolve solution check`): missing section: Decision"
	tGraduationMissing = "apicover new-package graduation: 1 new go/internal/<pkg>(s) changed this cycle are absent from .apicover-enforce — the apicover -enforce gate silently skips them (new-package blind spot). Add each to go/.apicover-enforce + an apicover_named_test.go before ship. Offenders: internal/baz"
	tVetDiskFull       = "go vet ./... reported 1 issue(s) — CI `vet + fmt` would FAIL (e.g. import cycle). Offenders: go: error obtaining buildID for go tool compile: write /var/folders/x/T/go-build1/b001/_pkg_.a: no space left on device"
	tTierRetakeNoRun   = "the integration tier (`go test -tags integration`) reported 3 offender(s) locally. Offenders: --- FAIL: TestFleetSoak (12.00s); FAIL\tgithub.com/mickeyyaya/evolve-loop/go/cmd/evolve\t12.4s; full output: /ws/integration-tier.log"
	tACSDurableRed     = "acs-durable (-tags acs) FAILED 1 check(s) — CI acs-durable gate would FAIL (flag-registry / flag-ceiling / skills-drift). Offenders: flag-ceiling"
	tApicoverUnnamed   = "apicover -enforce flagged 1 line(s) in touched enforced packages — CI `api-coverage enforce` would FAIL (unnamed export). Offenders: internal/bar:12"
	tEGPSHarnessRed    = "EGPS: red_count=2 [TestC9001_001 TestC9001_002] (cycle ships only when red_count==0); 2" + HarnessRedClauseMarker + ", first egps/cycle9001/TestC9001_001: go: command not found (+1 more in acs-verdict.json)"
)

func haltsOnProseInfraSystemic(t *testing.T, cs CycleState) (*SystemFailureSignal, string) {
	t.Helper()
	writeDecision(t, cs.WorkspacePath, tInfraDecision)
	_, _, reason, sig := floorOrchestrator(nil).decideAfterRetro(cs, VerdictFAIL, nil)
	return sig, reason
}

func gateFailDispatch(reasons ...string) dispatchResult {
	diags := make([]Diagnostic, len(reasons))
	for i, reason := range reasons {
		diags[i] = Diagnostic{Severity: "error", Message: reason}
	}
	return dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL, Diagnostics: diags}, attemptCount: 1}
}

func withCodeAuditFailBudget(n int) policy.SystemFailurePolicy {
	p := policy.DefaultSystemFailurePolicy()
	row := p.Categories[policy.CategoryCodeAuditFail]
	row.MaxRetries = n
	p.Categories[policy.CategoryCodeAuditFail] = row
	return p
}

func TestAuditGateFail_SkillsDriftOverPassNarrativeGrantsOneBuildRepairLedByItsRemediation(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow()

	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tSkillsDrift1828, tConflict1828)); err != nil {
		t.Fatalf("recordAndBranch: %v", err)
	}

	if cr.scheduledNext != PhaseBuild || cr.cs.AuditRepairAttempts != 1 {
		t.Fatalf("scheduledNext=%q attempts=%d decline=%q, want one audit-repair round at build: a PASS narrative overridden only by skills-drift has a one-command remedy",
			cr.scheduledNext, cr.cs.AuditRepairAttempts, cr.cs.AuditDeclineReason)
	}
	brief := seedAuditRepairContext(nil, PhaseBuild, cr.cs)[CtxKeyAuditRepairFindings]
	wantLead := "audit defects (the deterministic gates' failure block, class code-audit-fail):\n- regenerate with the worktree's own generator: `EVOLVE_WORKTREE_ROOT=<worktree> go run ./cmd/evolve skills generate` in <worktree>/go (an installed evolve binary renders its own build's templates) — skill projection drift: "
	if !strings.HasPrefix(brief, wantLead) {
		t.Errorf("the repair brief must lead with the gate's remediation:\ngot:\n%s\nwant prefix:\n%s", brief, wantLead)
	}
}

func TestAuditGateFail_IdenticalGateFailsAreBoundedByTheEnvelopeBudget(t *testing.T) {
	shipped := policy.DefaultSystemFailurePolicy().Categories[policy.CategoryCodeAuditFail].MaxRetries
	for _, budget := range []int{1, shipped} {
		t.Run(fmt.Sprintf("max_retries=%d", budget), func(t *testing.T) {
			cr := retroGateHarness(t, phasespec.Catalog{})
			defer cr.releaseShipWindow()
			cr.o.failurePolicy = withCodeAuditFailBudget(budget)

			grants := 0
			for round := 0; round <= budget; round++ {
				cr.scheduledNext = ""
				if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tSkillsDrift1828, tConflict1828)); err != nil {
					t.Fatalf("audit round %d: %v", round+1, err)
				}
				if cr.scheduledNext == PhaseBuild {
					grants++
				}
			}

			if grants != budget {
				t.Errorf("%d identical gate FAILs earned %d repair round(s), want exactly the envelope budget %d (last decline %q)",
					budget+1, grants, budget, cr.cs.AuditDeclineReason)
			}
			want := fmt.Sprintf("retry budget spent for code-audit-fail (%d/%d)", budget, budget)
			if cr.cs.AuditDeclineReason != want {
				t.Errorf("final disposition = %q, want the envelope's own decline %q", cr.cs.AuditDeclineReason, want)
			}
		})
	}
}

func TestAuditGateFail_ProseInfraSystemicRetroDoesNotHaltAGateDiagnosedFail(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow()
	cr.o.failurePolicy = withCodeAuditFailBudget(1)

	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tSkillsDrift1828, tConflict1828)); err != nil {
		t.Fatalf("audit round 1: %v", err)
	}
	granted := cr.scheduledNext
	cr.scheduledNext = ""
	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tSkillsDrift1828, tConflict1828)); err != nil {
		t.Fatalf("audit round 2: %v", err)
	}
	writeDecision(t, cr.cs.WorkspacePath, tInfraDecision)
	cr.current = PhaseRetro
	if _, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}); err != nil {
		t.Fatalf("retro: %v", err)
	}

	if granted != PhaseBuild {
		t.Errorf("first gate FAIL scheduled %q (decline %q), want the build repair round", granted, cr.cs.AuditDeclineReason)
	}
	if cr.result.SystemFailure != nil || strings.Contains(cr.result.RetroDecision, "system-failure-floor") {
		t.Errorf("a FAIL whose every reason is a gate diagnosis halted the loop on the retro's prose: sig=%+v decision=%q",
			cr.result.SystemFailure, cr.result.RetroDecision)
	}
}

func TestDecideAfterRetroFloor_StaticGateDiagnosisRefutesProseInfraSystemic(t *testing.T) {
	cases := []struct {
		name    string
		reasons []string
	}{
		{"skills-drift over a PASS narrative", []string{tSkillsDrift1828, tConflict1828}},
		{"gofmt over a WARN narrative", []string{tGofmtDirty, tConflictWARN}},
		{"every static gate over a PASS narrative", []string{tGofmtDirty, tSolutionViolated, tSkillsDrift1828, tGraduationMissing, tConflict1828}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := CycleState{CycleID: 1828, WorkspacePath: t.TempDir(), AuditFailReasons: tc.reasons}

			sig, reason := haltsOnProseInfraSystemic(t, cs)

			if sig != nil || strings.HasPrefix(reason, "system-failure-floor") {
				t.Fatalf("reason=%q sig=%+v: a FAIL that static gate readings alone forced over a PASS or WARN narrative is task-level whatever the retro's prose claimed", reason, sig)
			}
		})
	}
}

func TestDecideAfterRetroFloor_AGateThatExecutesCodeKeepsTheHalt(t *testing.T) {
	cases := []struct {
		name    string
		reasons []string
	}{
		{"go vet's diagnosis of a full disk", []string{tVetDiskFull, tConflict1828}},
		{"an integration tier whose retake could not run", []string{tTierRetakeNoRun, tConflict1828}},
		{"acs-durable", []string{tACSDurableRed, tConflict1828}},
		{"apicover-enforce", []string{tApicoverUnnamed, tConflict1828}},
		{"an EGPS red beside skills-drift", []string{tEGPSRed, tSkillsDrift1828, tConflict1828}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cs := CycleState{CycleID: 1828, WorkspacePath: t.TempDir(), AuditFailReasons: tc.reasons}

			sig, reason := haltsOnProseInfraSystemic(t, cs)

			if sig == nil || !sig.Halt || sig.Category != policy.CategoryInfraSystemic {
				t.Fatalf("reason=%q sig=%+v, want the infra-systemic halt: a gate that executes code can fail for the host's reasons, so its diagnosis cannot refute a systemic claim", reason, sig)
			}
		})
	}
}

func TestAuditGateFail_AnIntegrationTierRedKeepsItsRepairAndTheProseInfraSystemicHalt(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow()
	cr.o.failurePolicy = withCodeAuditFailBudget(1)

	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tTierRetakeNoRun, tConflict1828)); err != nil {
		t.Fatalf("audit round 1: %v", err)
	}
	granted := cr.scheduledNext
	cr.scheduledNext = ""
	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tTierRetakeNoRun, tConflict1828)); err != nil {
		t.Fatalf("audit round 2: %v", err)
	}
	writeDecision(t, cr.cs.WorkspacePath, tInfraDecision)
	cr.current = PhaseRetro
	_, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1})

	if granted != PhaseBuild {
		t.Errorf("first tier FAIL scheduled %q (decline %q), want the bounded build repair round every gate row keeps", granted, cr.cs.AuditDeclineReason)
	}
	if sig := cr.result.SystemFailure; sig == nil || !sig.Halt || sig.Category != policy.CategoryInfraSystemic {
		t.Errorf("sig=%+v decision=%q err=%v: the integration tier executes code, so its offenders (attempt 1's when the retake could not run) cannot refute the infra-systemic halt",
			sig, cr.result.RetroDecision, err)
	}
}

func TestAuditGateFail_AHarnessRedEGPSEarnsNoRepairAndKeepsTheHalt(t *testing.T) {
	cs := CycleState{CycleID: 9001, WorkspacePath: t.TempDir(), AuditFailReasons: []string{tEGPSHarnessRed, tConflict1828}}

	next, reason, _ := floorOrchestrator(fixedNextStrategy{next: "end"}).decideAfterAuditFail(cs)
	sig, floorReason := haltsOnProseInfraSystemic(t, cs)

	if next != PhaseRetro || reason != auditDeclineReasonPrefix+"audit declared no failure class; nothing to base a retry on" {
		t.Errorf("next=%s reason=%q: reds the harness itself could not run are not the gate's diagnosis, so they derive no class", next, reason)
	}
	if sig == nil || !sig.Halt {
		t.Errorf("reason=%q: an all-harness EGPS red must keep the infra-systemic halt", floorReason)
	}
}

func TestAuditGateFail_AFailNarrativeBesideAGateDiagnosisDerivesNothingAndKeepsTheFloor(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte("# Audit\n\n## Verdict\n**FAIL**\n\nCRITICAL: the retry envelope double-grants (H1).\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := CycleState{CycleID: 9001, WorkspacePath: dir, AuditFailReasons: []string{tGofmtDirty}}

	next, reason, _ := floorOrchestrator(fixedNextStrategy{next: "end"}).decideAfterAuditFail(cs)
	sig, floorReason := haltsOnProseInfraSystemic(t, cs)

	if next != PhaseRetro || reason != auditDeclineReasonPrefix+"audit declared no failure class; nothing to base a retry on" {
		t.Errorf("next=%s reason=%q: with no verdict-conflict record the auditor's own FAIL stands beside the gate, so nothing derives a class", next, reason)
	}
	if sig == nil || !sig.Halt {
		t.Errorf("reason=%q: a FAIL narrative beside a gate diagnosis must keep the infra-systemic halt", floorReason)
	}
}

func TestAuditGateFail_TheDerivedRouteIsRetryAtBuild(t *testing.T) {
	cs := CycleState{CycleID: 1828, WorkspacePath: t.TempDir(), AuditFailReasons: []string{tSkillsDrift1828, tConflict1828}}

	next, reason, sig := floorOrchestrator(fixedNextStrategy{next: "end"}).decideAfterAuditFail(cs)

	if next != PhaseBuild || sig != nil || !strings.HasPrefix(reason, auditRepairReasonPrefix+"retry@build: ") {
		t.Errorf("next=%s sig=%+v reason=%q, want the retry@build action: a gate's remedy is a code build, never an explanation re-author", next, sig, reason)
	}
}

func TestAuditRejectionReasons_AnAuditorDeclaredBlockKeepsTheDerivedGateBlockOut(t *testing.T) {
	dir := t.TempDir()
	writeAuditWithFailure(t, dir, "PASS", policy.CategoryCodeAuditFail, "H1 the auditor rejected this build")
	cs := CycleState{CycleID: 1828, WorkspacePath: dir, AuditFailReasons: []string{tSkillsDrift1828, tConflict1828}}

	got := auditRejectionReasons(cs, nil)

	want := "failed phase: audit\n- " + tSkillsDrift1828 + "\n- " + tConflict1828
	if got != want {
		t.Errorf("brief:\n%s\nwant the gate reasons as recorded: the auditor's declared block owns the class, so no derived block may speak for it:\n%s", got, want)
	}
}

func TestDecideAfterRetroFloor_ProseInfraSystemicWithAnyNonGateReasonStillHalts(t *testing.T) {
	cases := []struct {
		name  string
		audit []string
		ship  []string
	}{
		{"a CLI wall", []string{tCLIWall}, nil},
		{"a gate diagnosis beside a CLI wall", []string{tSkillsDrift1828, tCLIWall, tConflict1828}, nil},
		{"a gate diagnosis beside a ship reason", []string{tSkillsDrift1828, tConflict1828}, []string{"ship: push rejected: remote unreachable"}},
		{"a conflict record with no gate diagnosis", []string{tConflict1828}, nil},
		{"an EGPS ship_eligible anomaly", []string{"EGPS: acs-verdict.json ship_eligible=false — the authoritative acssuite SSOT rejects the ship even though red_count==0; a narrative PASS cannot override it"}, nil},
		{"a host predicate execution failure", []string{"host predicate execution: acssuite run: go: command not found"}, nil},
		{"no persisted reason", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := floorOrchestrator(nil)
			dir := t.TempDir()
			writeDecision(t, dir, tInfraDecision)
			cs := CycleState{CycleID: 1828, WorkspacePath: dir, AuditFailReasons: tc.audit, ShipFailReasons: tc.ship}

			next, _, _, sig := o.decideAfterRetro(cs, VerdictFAIL, nil)

			if next != PhaseEnd || sig == nil || !sig.Halt || sig.Category != policy.CategoryInfraSystemic {
				t.Fatalf("next=%s sig=%+v, want the infra-systemic halt: a reason outside the gate table keeps the floor", next, sig)
			}
		})
	}
}

func TestAuditGateFail_BookkeepingConflictStillTakesTheRegrade(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow()

	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tLedger, tConflictPASS)); err != nil {
		t.Fatalf("audit: %v", err)
	}
	auditNext, attempts := cr.scheduledNext, cr.cs.AuditRepairAttempts
	cr.current = PhaseRetro
	if _, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}); err != nil {
		t.Fatalf("retro: %v", err)
	}

	if auditNext != "" || attempts != 0 {
		t.Errorf("a bookkeeping conflict earned an audit-repair round (next=%q attempts=%d); the regrade owns it", auditNext, attempts)
	}
	if cr.scheduledNext != PhaseAudit || !strings.Contains(cr.result.RetroDecision, BookkeepingRegradeReasonPrefix) {
		t.Errorf("scheduledNext=%q decision=%q, want the once-per-cycle bookkeeping regrade", cr.scheduledNext, cr.result.RetroDecision)
	}
}

func TestAuditGateFail_GateBesideBookkeepingEarnsNeitherRoute(t *testing.T) {
	cr := retroGateHarness(t, phasespec.Catalog{})
	defer cr.releaseShipWindow()

	if _, err := cr.recordAndBranch(PhaseAudit, gateFailDispatch(tSkillsDrift1828, tLedger, tConflictPASS)); err != nil {
		t.Fatalf("audit: %v", err)
	}
	auditNext := cr.scheduledNext
	cr.current = PhaseRetro
	if _, err := cr.recordAndBranch(PhaseRetro, dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}, attemptCount: 1}); err != nil {
		t.Fatalf("retro: %v", err)
	}

	if auditNext != "" || cr.cs.AuditRepairAttempts != 0 {
		t.Errorf("a gate diagnosis beside a bookkeeping reason earned a repair round (next=%q); only an all-gate FAIL derives a class", auditNext)
	}
	if strings.Contains(cr.result.RetroDecision, BookkeepingRegradeReasonPrefix) {
		t.Errorf("decision=%q: a FAIL with a non-bookkeeping reason must not take the regrade", cr.result.RetroDecision)
	}
}

func TestAuditFailEnvelope_AnAuditorDeclaredClassKeepsItsFullEnvelope(t *testing.T) {
	o := floorOrchestrator(fixedNextStrategy{next: "end"})
	cs := CycleState{CycleID: 1828, WorkspacePath: auditFailFixture(t, policy.CategoryCodeAuditFail, "H1 the auditor rejected this build"),
		AuditFailReasons: []string{tGofmtDirty, tConflictWARN}}

	next, reason, _ := o.decideAfterAuditFail(cs)

	if next != PhaseTDD {
		t.Errorf("next=%s (%s): the auditor's own failure block outranks the gate-derived one, so its envelope keeps tdd first", next, reason)
	}
}

func TestAuditGateRemedies_EveryGateDeclaresARepairableClassAndARemediation(t *testing.T) {
	t.Parallel()
	if len(auditGateRemedies) == 0 {
		t.Fatal("the gate table is empty: no deterministic gate FAIL can declare a class")
	}
	env := computeRetryEnvelope(retryEnvelopeInput{DeclaredClass: string(gateDerivedClass), Policy: policy.DefaultSystemFailurePolicy()})
	if !slices.Contains(env.Legal, retryActionRetryBuild) {
		t.Errorf("class %q earns %v (%s); a gate's forced FAIL must be repairable at build", gateDerivedClass, env.Legal, env.Reason)
	}
	if got := AuditGateRemedy("no such gate"); got != "" {
		t.Errorf("an unknown gate renders remedy %q, want none", got)
	}
	gates := map[string]bool{}
	for _, row := range auditGateRemedies {
		if gates[row.gate] {
			t.Errorf("gate %q has two rows; one gate, one remediation", row.gate)
		}
		gates[row.gate] = true
		t.Run(row.gate, func(t *testing.T) {
			if strings.TrimSpace(row.remediation) == "" {
				t.Error("no remediation: the repair brief has nothing to lead with")
			}
			if got := AuditGateRemedy(row.gate); got != row.remediation {
				t.Errorf("the producers render remedy %q for gate %q, want its own row's %q", got, row.gate, row.remediation)
			}
			if got := AuditGateOf(row.reasonPrefix + "1 offender(s)"); got != row.gate {
				t.Errorf("a %q reason classifies as gate %q; an earlier row's prefix shadows this one", row.gate, got)
			}
			if BookkeepingMetaAuditReason(row.reasonPrefix) || BookkeepingConflictAuditReason(row.reasonPrefix) {
				t.Errorf("prefix %q is also a bookkeeping reason; the regrade and the gate repair must never claim one FAIL together", row.reasonPrefix)
			}
		})
	}
}

func TestGateFailureBlock_DerivesTheClassOnlyWhenGatesAloneForcedTheFail(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		reasons     []string
		wantDefects []string
	}{
		{"skills-drift over a PASS narrative", []string{tSkillsDrift1828, tConflict1828},
			[]string{"regenerate with the worktree's own generator: `EVOLVE_WORKTREE_ROOT=<worktree> go run ./cmd/evolve skills generate` in <worktree>/go (an installed evolve binary renders its own build's templates) — " + tSkillsDrift1828}},
		{"two gates over a WARN narrative keep their order", []string{tGofmtDirty, tEGPSRed, tConflictWARN},
			[]string{"run `gofmt -w -s .` in go/ — " + tGofmtDirty, "turn every red ACS predicate green — " + tEGPSRed}},
		{"a gate diagnosis with no conflict record", []string{tGofmtDirty}, nil},
		{"an EGPS red the harness could not run", []string{tEGPSHarnessRed, tConflict1828}, nil},
		{"a conflict record alone", []string{tConflict1828}, nil},
		{"no reasons", nil, nil},
		{"a gate beside a bookkeeping reason", []string{tSkillsDrift1828, tLedger, tConflictPASS}, nil},
		{"a gate beside a CLI wall", []string{tGofmtDirty, tCLIWall, tConflictWARN}, nil},
		{"a host failure quoting a gate's words", []string{"host predicate execution: acssuite run: gofmt: signal: killed", tConflictWARN}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fb, ok := gateFailureBlock(tc.reasons)

			if ok != (tc.wantDefects != nil) {
				t.Fatalf("derived=%v for %v, want %v", ok, tc.reasons, tc.wantDefects != nil)
			}
			if !ok {
				return
			}
			if fb.Class != "code-audit-fail" || !slices.Equal(fb.Defects, tc.wantDefects) {
				t.Errorf("block = %+v, want class code-audit-fail and defects %q", fb, tc.wantDefects)
			}
		})
	}
}

type gateForcedAuditRunner struct{}

func (gateForcedAuditRunner) Name() string { return string(PhaseAudit) }

func (gateForcedAuditRunner) Run(_ context.Context, req PhaseRequest) (PhaseResponse, error) {
	return PhaseResponse{Phase: string(PhaseAudit), Verdict: VerdictFAIL, ArtifactsDir: req.Workspace,
		Diagnostics: []Diagnostic{{Severity: "error", Message: tSkillsDrift1828}, {Severity: "error", Message: tConflict1828}}}, nil
}

func TestResumePath_GateForcedAuditFailRepairsAtBuildWithinTheBudget(t *testing.T) {
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: 1828, WorkspacePath: t.TempDir()}}
	runners := buildRunners(map[Phase]string{PhaseAudit: VerdictFAIL, PhaseRetro: VerdictFAIL})
	runners[PhaseAudit] = gateForcedAuditRunner{}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	res, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: t.TempDir()},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: 1828})
	if err != nil {
		t.Fatalf("resume cycle: %v", err)
	}

	budget := policy.DefaultSystemFailurePolicy().Categories[policy.CategoryCodeAuditFail].MaxRetries
	builds, tdds := 0, 0
	for _, p := range res.PhasesRun {
		switch p {
		case PhaseBuild:
			builds++
		case PhaseTDD:
			tdds++
		}
	}
	if builds != budget || tdds != 0 {
		t.Errorf("phases=%v: a resumed gate-forced FAIL must re-enter build exactly %d time(s) and never tdd", res.PhasesRun, budget)
	}
}
