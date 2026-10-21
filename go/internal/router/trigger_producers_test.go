package router

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestHarnessProduces_TypedAndFailureSentinelFieldsOnly(t *testing.T) {
	t.Parallel()
	for field, want := range map[string]bool{
		"scout.goal_type":         true,
		"cycle_size":              true,
		"build.files_touched":     true,
		"audit.verdict":           true,
		"ship.class":              true,
		"run_dir.artifact_bytes":  true,
		"audit.failure_class":     true,
		"tdd.defect_count":        true,
		".failure_class":          false,
		"campaign.mode":           false,
		"scout.security_relevant": false,
		"amplify.tests_added":     false,
	} {
		if got := HarnessProduces(field); got != want {
			t.Errorf("HarnessProduces(%q) = %v, want %v", field, got, want)
		}
	}
}

func TestDigest_ACompletedShipIsTheCycleClass(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	before, err := Digest(ws, []string{"scout", "build", "audit"})
	if err != nil {
		t.Fatal(err)
	}
	after, err := Digest(ws, []string{"scout", "build", "audit", "ship"})
	if err != nil {
		t.Fatal(err)
	}
	if before.Ship != (ShipSignals{}) || after.Ship != (ShipSignals{Class: ShipClassCycle}) {
		t.Fatalf("Ship.Class before ship = %q, after = %q; want \"\" then %q", before.Ship.Class, after.Ship.Class, ShipClassCycle)
	}
	if !evalCondition(after, config.Condition{Field: "ship.class", Op: "==", Value: "cycle"}) {
		t.Fatalf("ship.class == cycle must hold once ship completed: %+v", after.Ship)
	}
}

func TestDigest_RunDirArtifactBytesSumsEveryFile(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFile(t, ws, "scout-report.md", "12345")
	if err := os.MkdirAll(filepath.Join(ws, "acs"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ws, "acs"), "verdict.json", "1234567")
	sig, err := Digest(ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if sig.RunDir.ArtifactBytes != 12 {
		t.Fatalf("RunDir.ArtifactBytes = %d, want 12", sig.RunDir.ArtifactBytes)
	}
	if !evalCondition(sig, config.Condition{Field: "run_dir.artifact_bytes", Op: "gt", Value: float64(11)}) {
		t.Fatal("run_dir.artifact_bytes gt 11 must hold for a 12-byte run dir")
	}
}

func TestDigest_AnUnreadableRunDirEntryDegradesTheDigest(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 directory, so the walk cannot fail")
	}
	ws := t.TempDir()
	locked := filepath.Join(ws, "locked")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, locked, "a.md", "abc")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })
	sig, err := Digest(ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !containsPrefix(sig.DigestDegraded, "run_dir: artifact bytes: ") {
		t.Fatalf("DigestDegraded = %q, want a run_dir artifact-bytes entry", sig.DigestDegraded)
	}
}

func TestDigest_FoldsTheHandoffSignalsOfAnyCompletedPhase(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFile(t, ws, "handoff-test-amplification.json", `{"payload":{"signals":{"amplify.tests_added":3,"ratio":0.5}}}`)
	writeFile(t, ws, "handoff-mutation-gate.json", `{"payload":{"signals":{"mutation.score":9}}}`)
	sig, err := Digest(ws, []string{"test-amplification"})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := sig.GenericValue("amplify.tests_added"); v != float64(3) {
		t.Errorf("amplify.tests_added = %v, want 3 from the completed phase's handoff", v)
	}
	if v, _ := sig.GenericValue("test-amplification.ratio"); v != 0.5 {
		t.Errorf("test-amplification.ratio = %v, want 0.5 (a bare key is phase-prefixed)", v)
	}
	if _, ok := sig.GenericValue("mutation.score"); ok {
		t.Error("a phase that has not completed contributes no signal")
	}
}

func TestRoute_ChangelogSyncIsAdmittedAtAdvisoryOnceShipProducesItsSignal(t *testing.T) {
	t.Parallel()
	shipped := []string{"scout", "triage", "build", "audit", "ship"}
	for _, tc := range []struct {
		name      string
		completed []string
		want      bool
	}{
		{"ship completed", shipped, true},
		{"ship absent", shipped[:4], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sig, err := Digest(t.TempDir(), tc.completed)
			if err != nil {
				t.Fatal(err)
			}
			in := RouteInput{
				Signals: sig,
				Cfg: config.RoutingConfig{
					Stage:         config.StageAdvisory,
					MaxInsertions: 10,
					Triggers: map[string]config.RoutingBlock{"changelog-sync": {InsertWhen: []config.Condition{
						{Field: "ship.class", Op: "==", Value: "cycle"},
					}}},
				},
				Plan: &PhasePlan{Entries: []PhasePlanEntry{{Phase: "changelog-sync", Run: true}}},
			}
			runs, _, clamp := shouldRunFromPlan(in, "changelog-sync", config.EnableContent, 0)
			if runs != tc.want {
				t.Fatalf("changelog-sync runs = %v (clamp %+v), want %v", runs, clamp, tc.want)
			}
		})
	}
}

func containsPrefix(xs []string, prefix string) bool {
	for _, x := range xs {
		if strings.HasPrefix(x, prefix) {
			return true
		}
	}
	return false
}

func TestDigest_AHandoffCannotForgeASignalTheHarnessOrAnotherPhaseOwns(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFile(t, ws, "handoff-code-review.json", `{"signals":{"audit.failure_class":"code-audit-fail","tdd.failure_class":"code-build-fail","scout.goal_type":"security","code-review.failure_class":"forged","code-review.findings":2,"review.depth":"deep"}}`)

	sig, err := Digest(ws, []string{"scout", "tdd", "code-review", "audit"})
	if err != nil {
		t.Fatal(err)
	}

	for _, forged := range []string{"audit.failure_class", "tdd.failure_class", "scout.goal_type", "code-review.failure_class"} {
		if _, inGeneric := sig.Generic[forged]; inGeneric {
			t.Errorf("%s from handoff-code-review.json landed in the generic plane", forged)
		}
		if _, _, _, present := resolveField(sig, forged); present {
			t.Errorf("%s from handoff-code-review.json reached routing: a handoff writes only its own signals", forged)
		}
	}
	for field, want := range map[string]any{"code-review.findings": float64(2), "review.depth": "deep"} {
		if v, ok := sig.GenericValue(field); !ok || v != want {
			t.Errorf("%s = %v (%v), want %v: a phase keeps its own and unowned signals", field, v, ok, want)
		}
	}
}

func TestDigest_TheHarnessFailureBlockFoldsAfterEveryHandoff(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md", "# Audit\n<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"FAIL\",\"schema_version\":2,\"failure\":{\"class\":\"code-audit-fail\",\"defects\":[\"d1\"]}} -->\n")
	writeFile(t, ws, "handoff-memo.json", `{"signals":{"audit.failure_class":"none","audit.defect_count":0}}`)

	sig, err := Digest(ws, []string{"audit", "memo"})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := sig.GenericValue("audit.failure_class"); v != "code-audit-fail" {
		t.Errorf("audit.failure_class = %v, want the harness value code-audit-fail", v)
	}
	if v, _ := sig.GenericValue("audit.defect_count"); v != float64(1) {
		t.Errorf("audit.defect_count = %v, want 1 from the harness", v)
	}
}
