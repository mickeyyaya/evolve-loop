package core

// repair_tier_escalation_test.go — the producer half of `audit_retry_2plus`.
//
// builder.json and tdd-engineer.json have declared
// `model_tier_overrides.audit_retry_2plus: "deep"` since the override table
// landed, and the only tests guarding it checked that the VALUE is a canonical
// tier. Nothing produced the situation: cycles 1595–1605 re-dispatched every
// repair round at the identical tier (balanced, balanced, balanced) and ship
// probability by audit-round count ran 100 % → 50 % → 17 % → 0 %. The
// orchestrator now raises the tdd/build re-dispatch tier to the profile's
// declared override while CycleState.AuditRepairActive is set — the same
// persisted flag the repair brief derives from — through the same envelope
// clamp the ADR-0076 D floor uses.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeRepairProfile(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const repairBuilderProfile = `{"name":"builder","role":"builder","model_tier_default":"balanced",` +
	`"model_tier_envelope":{"min":"balanced","default":"balanced","max":"deep"},` +
	`"model_tier_overrides":{"audit_retry_2plus":"deep"}}`

func TestRepairRoundTier_RaisesToDeclaredOverrideForSeededPhases(t *testing.T) {
	root := t.TempDir()
	writeRepairProfile(t, root, "builder", repairBuilderProfile)
	writeRepairProfile(t, root, "tdd-engineer", `{"name":"tdd-engineer","model_tier_default":"balanced","model_tier_overrides":{"audit_retry_2plus":"deep"}}`)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	active := CycleState{CycleID: 1605, AuditRepairActive: true, AuditRepairAttempts: 1}

	for _, phase := range []Phase{PhaseBuild, PhaseTDD} {
		tier, raised := o.repairRoundTier(root, phase, active, "")
		if !raised || tier != "deep" {
			t.Fatalf("%s in a repair round must raise to the declared override, got (%q,%v)", phase, tier, raised)
		}
	}
	if _, raised := o.repairRoundTier(root, PhaseAudit, active, ""); raised {
		t.Fatal("audit is not repair-seeded and must never be escalated by this rule")
	}
	if _, raised := o.repairRoundTier(root, PhaseBuild, CycleState{CycleID: 1605}, ""); raised {
		t.Fatal("first dispatch (no repair in flight) must not raise")
	}
	if _, raised := o.repairRoundTier(root, PhaseBuild, CycleState{CycleID: 1605, AuditRepairAttempts: 2}, ""); raised {
		t.Fatal("a FINISHED repair (counter > 0, flag cleared) must not leak the raise into a later dispatch")
	}
}

func TestRepairRoundTier_RaiseOnlyAndDeclarationRequired(t *testing.T) {
	root := t.TempDir()
	writeRepairProfile(t, root, "builder", repairBuilderProfile)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	active := CycleState{CycleID: 1, AuditRepairActive: true, AuditRepairAttempts: 1}
	if _, raised := o.repairRoundTier(root, PhaseBuild, active, "deep"); raised {
		t.Fatal("already at the declared tier — no raise")
	}
	if _, raised := o.repairRoundTier(root, PhaseBuild, active, "top"); raised {
		t.Fatal("a higher proposal must never be lowered")
	}
	writeRepairProfile(t, root, "builder", `{"name":"builder","model_tier_default":"balanced"}`)
	if _, raised := o.repairRoundTier(root, PhaseBuild, active, ""); raised {
		t.Fatal("no declared audit_retry_2plus key ⇒ the rule is inert (config decides, not Go)")
	}
}

func TestRepairRoundTier_EnvelopeMaxClampsThroughRealGuardrail(t *testing.T) {
	root := t.TempDir()
	writeRepairProfile(t, root, "builder", `{"name":"builder","model_tier_default":"fast",`+
		`"model_tier_envelope":{"min":"fast","default":"fast","max":"balanced"},"model_tier_overrides":{"audit_retry_2plus":"deep"}}`)
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	active := CycleState{CycleID: 9, AuditRepairActive: true, AuditRepairAttempts: 1}
	tier, raised := o.repairRoundTier(root, PhaseBuild, active, "fast")
	if !raised || tier != "balanced" {
		t.Fatalf("envelope max must clamp deep→balanced through ClampPlanModelRouting, got (%q,%v)", tier, raised)
	}
	if _, raised := o.repairRoundTier(root, PhaseBuild, active, "balanced"); raised {
		t.Fatal("clamped to no gain ⇒ no raise")
	}
}

// TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop is the wiring proof
// (I2 invariant): driven through RunCycle with the real dispatch seam, the
// FIRST build runs at the profile default (no overlay) and the build
// re-dispatched inside the repair round granted after audit round 1's FAIL
// carries ModelRoutingTier=deep — the declared override, live. A unit-green
// repairRoundTier that nothing calls would leave this red.
func TestRepairRoundDispatch_RaisesBuildTierThroughLiveLoop(t *testing.T) {
	root := t.TempDir()
	writeRepairProfile(t, root, "builder", repairBuilderProfile)
	st := &fakeStorage{state: State{LastCycleNumber: 0}}
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	ar := &verdictStagingAuditRunner{t: t}
	runners[PhaseAudit] = ar
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: root}); err != nil {
		t.Fatalf("RunCycle: %v", err)
	}
	if ar.runs < 2 {
		t.Fatalf("audit ran %d time(s); no repair round happened", ar.runs)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) < 2 {
		t.Fatalf("build dispatched %d time(s); the repair round never re-dispatched build", len(fr.requests))
	}
	if got := fr.requests[0].ModelRoutingTier; got != "" {
		t.Errorf("first build dispatch tier = %q, want the profile default (no overlay)", got)
	}
	if got := fr.requests[len(fr.requests)-1].ModelRoutingTier; got != "deep" {
		t.Errorf("repair-round build dispatch tier = %q, want \"deep\" — the declared audit_retry_2plus override must reach the dispatched request", got)
	}
}

// TestRepairRoundResume_RaisesBuildTierAndArchivesPromptThroughResumePath is
// the resume-surface twin of the live-loop proof: a cycle RESUMED at audit
// whose audit FAILs is granted a repair round by resume.go's own audit-FAIL
// branch, and the build it re-dispatches must carry ModelRoutingTier=deep and
// must have retired the previous attempt's prompt — through resume.go's
// request builder, not cyclerun_dispatch.go's. A raise wired only on the live
// loop would leave this red (the crash-resume divergence class).
func TestRepairRoundResume_RaisesBuildTierAndArchivesPromptThroughResumePath(t *testing.T) {
	root := t.TempDir()
	writeRepairProfile(t, root, "builder", repairBuilderProfile)
	const cycle = 1605
	ws := RunWorkspacePath(root, cycle)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	// The previous attempt's dispatched prompt, as the bridge leaves it.
	if err := os.WriteFile(filepath.Join(ws, "build-prompt.txt"), []byte("round-1 brief"), 0o644); err != nil {
		t.Fatal(err)
	}
	runners := buildRunners(map[Phase]string{PhaseRetro: VerdictFAIL})
	ar := &verdictStagingAuditRunner{t: t}
	runners[PhaseAudit] = ar
	// The persisted cycle state a crash-resume reads back: the workspace is the
	// one the previous attempt dispatched into.
	st := &fakeStorage{state: State{LastCycleNumber: 0}, cycleState: CycleState{CycleID: cycle, Phase: string(PhaseAudit), WorkspacePath: ws}}
	o := NewOrchestrator(st, &fakeLedger{}, runners)

	if _, err := o.RunCycleFromPhase(context.Background(), CycleRequest{ProjectRoot: root},
		&ResumePoint{Phase: string(PhaseAudit), CycleID: cycle}); err != nil {
		t.Fatalf("RunCycleFromPhase: %v", err)
	}
	if ar.runs < 2 {
		t.Fatalf("audit ran %d time(s) on the resume path; no repair round happened", ar.runs)
	}
	fr := runners[PhaseBuild].(*fakeRunner)
	if len(fr.requests) == 0 {
		t.Fatal("the resumed repair round never re-dispatched build")
	}
	if got := fr.requests[len(fr.requests)-1].ModelRoutingTier; got != "deep" {
		t.Errorf("resumed repair-round build tier = %q, want \"deep\" — resume.go must apply repairRoundTier like the live loop", got)
	}
	if _, err := os.Stat(filepath.Join(ws, "build-prompt.round1.txt")); err != nil {
		t.Errorf("resume path must archive the previous attempt's prompt as build-prompt.round1.txt: %v", err)
	}
}
