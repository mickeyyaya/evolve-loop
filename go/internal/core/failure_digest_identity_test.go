package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// identityWorkspace materializes the minimal artifacts the distinguisher
// consults: an audit-report.md carrying a schema-v2 verdict sentinel with a
// defects list, and a triage-decision.json committing taskID.
func identityWorkspace(t *testing.T, taskID string, defects ...string) string {
	t.Helper()
	ws := t.TempDir()
	sentinel := map[string]any{
		"phase": "audit", "verdict": "FAIL", "schema_version": 2,
		"failure": map[string]any{"class": "code-audit-fail", "defects": defects},
	}
	sj, err := json.Marshal(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	report := "# Audit Report\n\nprose\n\n<!-- evolve-verdict: " + string(sj) + " -->\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	if taskID != "" {
		td, err := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": taskID}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), td, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

// digestFor runs the REAL dispatch-composed reason through the REAL assembler
// — the same two calls cyclerun_dispatch.go makes — so the test covers the
// production path, not a reimplementation.
func digestFor(t *testing.T, ws string) FailureDigest {
	t.Helper()
	reason := agentGradedFailReason("audit", ws)
	b, err := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit", Reasons: []string{reason}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestFailureDigest_SameTaskDistinctDefectsGetDistinctFingerprints(t *testing.T) {
	heads := []string{
		"CRITICAL: the gc.mode=enforce apply path (cmd_gc.go:147-157) has zero covering tests at any level",
		"CRITICAL: gc.Policy.Worktrees grace is never defaulted (withDefaults skips it), so KeepRecent=0 ships",
		"cmd_gc.go:106-127 gcWorkspaceSweep has no `case \"off\": return` — an explicit gc.mode=off is ignored",
	}
	seen := map[string]string{}
	for _, h := range heads {
		d := digestFor(t, identityWorkspace(t, "workspace-hygiene-s5-wiring-shadow-default", h))
		if prev, dup := seen[d.Fingerprint]; dup {
			t.Fatalf("distinct defects share fingerprint %s:\n  %q\n  %q\n— the 1137/1139/1143 false-identity that halted batch-14", d.Fingerprint, prev, h)
		}
		seen[d.Fingerprint] = h
		if d.Unexplained {
			t.Errorf("defect-bearing digest marked Unexplained: %q", h)
		}
	}
}

func TestFailureDigest_SameDefectAcrossRetryCyclesCollides(t *testing.T) {
	a := digestFor(t, identityWorkspace(t, "task-x",
		"acs/cycle1141 predicates never compile; TestC1141_004 drives the enforce path (cycle-1141)"))
	b := digestFor(t, identityWorkspace(t, "task-x",
		"acs/cycle1142 predicates never compile; TestC1142_004 drives the enforce path (cycle-1142)"))
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("the SAME defect on consecutive retry cycles minted different fingerprints:\n  %s\n  %s\n— cycle-numbered tokens in defect text must normalize out or the breaker never catches real recurrence", a.Fingerprint, b.Fingerprint)
	}
}

func TestVerdictFailDistinguisher_SentinelDefectsOutrankTaskIDs(t *testing.T) {
	ws := identityWorkspace(t, "task-x", "CRITICAL: the one true defect")
	got := verdictFailDistinguisher("audit", ws)
	if !strings.Contains(got, "defect=") || !strings.Contains(got, "one true defect") {
		t.Fatalf("distinguisher = %q, want the sentinel defect head", got)
	}
	if strings.Contains(got, "tasks=") {
		t.Errorf("distinguisher %q leads with task identity — it cannot separate same-task retries", got)
	}
}

func TestVerdictFailDistinguisher_FallsBackTasksWhenNoDefects(t *testing.T) {
	ws := t.TempDir()
	td, _ := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": "task-y"}}})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), td, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := verdictFailDistinguisher("audit", ws); got != "tasks=task-y" {
		t.Fatalf("distinguisher = %q, want tasks=task-y", got)
	}
}

func TestVerdictFailDistinguisher_ClasslessSentinelFallsSoftToBullets(t *testing.T) {
	ws := t.TempDir()
	report := "# Audit Report\n\n- D1 CRITICAL: the bullet-layer defect\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":2,"failure":{"defects":["classless sentinel defect"]}} -->` + "\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	got := verdictFailDistinguisher("audit", ws)
	if strings.Contains(got, "classless sentinel defect") {
		t.Fatalf("distinguisher = %q — a class-less sentinel is not authoritative and must not source identity", got)
	}
	if !strings.Contains(got, "bullet-layer defect") {
		t.Fatalf("distinguisher = %q, want the bullet-layer fallback", got)
	}
}

func TestAssembleFailureDigest_BoilerplateOnlyReasonIsUnexplained(t *testing.T) {
	ws := t.TempDir()
	b, _ := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit",
		Reasons: []string{agentGradedRouterReason("audit")}})
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Unexplained {
		t.Fatalf("boilerplate-only digest not marked Unexplained (fingerprint %s) — three of these false-tripped the identical-fingerprint breaker on batch-14", d.Fingerprint)
	}
	b, _ = json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit",
		Reasons: []string{agentGradedRouterReason("audit"), "EGPS: red_count=1 [GCHookRunsAfterFinalize]"}})
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Unexplained {
		t.Fatalf("a mixed reason set (boilerplate + real gate detail) marked Unexplained — its real reason IS a defect identity the breaker must keep counting")
	}
}

func TestEvaluateBlockerBreaker_UnexplainedDigestsNeverAssertIdentity(t *testing.T) {
	shared := []FailureDigest{
		{Cycle: 1, Fingerprint: "audit|verdict-fail|deadbeef0000", PreClass: "verdict-fail", Unexplained: true},
		{Cycle: 2, Fingerprint: "audit|verdict-fail|deadbeef0000", PreClass: "verdict-fail", Unexplained: true},
		{Cycle: 3, Fingerprint: "audit|verdict-fail|deadbeef0000", PreClass: "verdict-fail", Unexplained: true},
	}
	v := EvaluateBlockerBreaker(shared, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3})
	if v.Halt {
		t.Fatalf("identical-fingerprint rule asserted identity over UNEXPLAINED digests: %+v — distinct failures collapse into the content-free bucket by construction", v)
	}
	v = EvaluateBlockerBreaker(shared, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3, UnexplainedCeiling: 3})
	if !v.Halt || v.Rule != "unexplained-failures" {
		t.Fatalf("unexplained rule did not claim the content-free digests: %+v — the diagnosability breakdown must stay visible under its honest name", v)
	}
}

func TestAgentGradedRouterReason_MatchesBoilerplateDetector(t *testing.T) {
	for _, phase := range []string{"audit", "adversarial-review"} {
		for _, r := range []string{agentGradedRouterReason(phase), abnormalEpilogueReason(phase)} {
			if !isBoilerplateRouterReason(r) {
				t.Fatalf("the fallback line %q does not match its own boilerplate detector", r)
			}
			if isBoilerplateRouterReason(r + " defect=something real") {
				t.Fatalf("a distinguisher-bearing reason must NOT read as boilerplate: %q", r)
			}
		}
	}
	if isBoilerplateRouterReason("bridge: launch exit=81: core: bridge artifact timeout") {
		t.Fatal("a real error string must never read as boilerplate")
	}
}

func TestAbnormalEpilogue_DigestIsUnexplained(t *testing.T) {
	cr, _ := epilogueRun(t, false)
	cr.abnormalEpilogue(nil)
	raw, err := os.ReadFile(filepath.Join(cr.cs.WorkspacePath, "failure-digest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d FailureDigest
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if !d.Unexplained {
		t.Fatalf("abnormal-epilogue digest not marked Unexplained: %+v — its template is constant per phase, so identical fingerprints across distinct aborts are guaranteed", d)
	}
}

func TestAbnormalEpilogue_CauseBecomesDistinguisher(t *testing.T) {
	digestFor := func(t *testing.T, cause error) FailureDigest {
		t.Helper()
		cr, _ := epilogueRun(t, false)
		cr.abnormalEpilogue(cause)
		raw, err := os.ReadFile(filepath.Join(cr.cs.WorkspacePath, "failure-digest.json"))
		if err != nil {
			t.Fatal(err)
		}
		var d FailureDigest
		if err := json.Unmarshal(raw, &d); err != nil {
			t.Fatal(err)
		}
		return d
	}
	timeout := digestFor(t, fmt.Errorf("build: bridge: launch exit=81: [claude-tmux] session killed: core: bridge artifact timeout"))
	if timeout.Unexplained {
		t.Fatalf("an abort WITH a cause must be content-bearing, got Unexplained: %+v", timeout)
	}
	compile := digestFor(t, fmt.Errorf("build: worktree self-check: go vet ./...: exit 1"))
	if compile.Unexplained || compile.Fingerprint == timeout.Fingerprint {
		t.Fatalf("distinct causes must yield distinct fingerprints: %q vs %q", compile.Fingerprint, timeout.Fingerprint)
	}
	// Same cause twice ⇒ same fingerprint (the breaker's recurrence signal).
	if again := digestFor(t, fmt.Errorf("build: bridge: launch exit=81: [claude-tmux] session killed: core: bridge artifact timeout")); again.Fingerprint != timeout.Fingerprint {
		t.Fatalf("identical causes must share a fingerprint: %q vs %q", again.Fingerprint, timeout.Fingerprint)
	}
	// The same cause on consecutive RETRY cycles carries cycle-numbered tokens
	// that must normalize out — otherwise recurrence never accumulates and the
	// breaker goes blind (the raw-cause degenerate implementation fails here).
	c1 := digestFor(t, fmt.Errorf("build: acs/cycle1197 worktree: bridge artifact timeout"))
	c2 := digestFor(t, fmt.Errorf("build: acs/cycle1207 worktree: bridge artifact timeout"))
	if c1.Fingerprint != c2.Fingerprint {
		t.Fatalf("same cause across retry cycles must share a fingerprint: %q vs %q", c1.Fingerprint, c2.Fingerprint)
	}
	// A multi-line cause must flatten — a raw newline inside the reason breaks
	// downstream line-oriented consumers.
	multi := digestFor(t, fmt.Errorf("build: vet failed:\n./pkg/x.go:10: undefined: y\n"))
	if multi.Unexplained {
		t.Fatalf("multi-line cause must still be content-bearing: %+v", multi)
	}
}

func TestAbnormalEpilogue_TeardownMarkedAndTailKept(t *testing.T) {
	cr, _ := epilogueRun(t, false)
	cr.abnormalEpilogue(fmt.Errorf("build: bridge: %w", ErrArtifactTimeout))
	raw, err := os.ReadFile(filepath.Join(cr.cs.WorkspacePath, "audit-fail-reason.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "teardown=") {
		t.Fatalf("teardown-shaped cause must carry the teardown marker: %s", raw)
	}
	longPrefix := strings.Repeat("shared/prefix/segment/", 12)
	a := causeHead("phase build: " + longPrefix + ": root cause alpha")
	b := causeHead("phase build: " + longPrefix + ": root cause beta")
	if a == b {
		t.Fatalf("tail-kept truncation must keep distinct roots distinct under a long shared prefix: %q", a)
	}
	if !strings.Contains(a, "root cause alpha") {
		t.Fatalf("the root cause (chain tail) must survive truncation: %q", a)
	}
}

func TestFailureDigest_DurationTokensNormalizeOut(t *testing.T) {
	a := fingerprint("audit", "gate-block", []string{
		"--- FAIL: TestEveryGateShapedFileIsProtectedSurface (0.02s); file: x_guard_test.go; FAIL\tgo/acs/regression/protectedsurface\t1.478s"})
	b := fingerprint("audit", "gate-block", []string{
		"--- FAIL: TestEveryGateShapedFileIsProtectedSurface (0.03s); file: x_guard_test.go; FAIL\tgo/acs/regression/protectedsurface\t1.495s"})
	if a != b {
		t.Fatalf("reasons differing ONLY in test durations minted different fingerprints:\n  %s\n  %s", a, b)
	}
	c := fingerprint("audit", "gate-block", []string{
		"--- FAIL: TestEveryGateShapedFileIsProtectedSurface (0.02s); file: DIFFERENT_guard_test.go; FAIL\tgo/acs/regression/protectedsurface\t1.478s"})
	if a == c {
		t.Fatalf("different offending files collapsed to one fingerprint — normalization over-folded")
	}
	d := fingerprint("audit", "gate-block", []string{"go test -timeout 300s failed"})
	e := fingerprint("audit", "gate-block", []string{"go test -timeout 600s failed"})
	if d == e {
		t.Fatalf("integer-second config tokens (-timeout 300s vs 600s) folded — the duration rule must require a decimal")
	}
}
