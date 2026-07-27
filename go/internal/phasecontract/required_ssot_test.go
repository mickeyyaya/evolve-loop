package phasecontract

import "testing"

// required_ssot_test.go pins the "what counts as a complete cycle" SSOT that
// cyclehealth, redteamcheck and ledgerverify now read instead of each carrying
// their own `[]string{"scout", "builder", "auditor"}` literal (cycle-1140,
// phasecontract-role-artifact-ssot). The accessors must DERIVE from the
// registry — a hand-typed return here would recreate the very drift they exist
// to remove.

func TestRequiredRoles_DerivesFromRegistryAgentNames(t *testing.T) {
	got := RequiredRoles()
	want := []string{"scout", "builder", "auditor"}
	if len(got) != len(want) {
		t.Fatalf("RequiredRoles() = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("RequiredRoles()[%d] = %q, want %q", i, got[i], w)
		}
	}
	// DERIVATION (anti-hardcode): each returned role must be the AgentName of
	// its registry contract. A literal return passes the equality check above
	// but drifts silently the moment a contract's AgentName changes.
	for i, phase := range requiredPhases {
		c, ok := For(phase)
		if !ok {
			t.Fatalf("requiredPhases[%d]=%q has no registered contract", i, phase)
		}
		if got[i] != c.AgentName {
			t.Errorf("RequiredRoles()[%d] = %q, but For(%q).AgentName = %q — the accessor is "+
				"not sourced from the registry", i, got[i], phase, c.AgentName)
		}
	}
}

func TestRequiredArtifacts_DerivesFromRegistryArtifactNames(t *testing.T) {
	got := RequiredArtifacts()
	want := []string{"scout-report.md", "build-report.md", "audit-report.md"}
	if len(got) != len(want) {
		t.Fatalf("RequiredArtifacts() = %v, want %v", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("RequiredArtifacts()[%d] = %q, want %q", i, got[i], w)
		}
	}
	for i, phase := range requiredPhases {
		c, _ := For(phase)
		if got[i] != c.ArtifactName {
			t.Errorf("RequiredArtifacts()[%d] = %q, but For(%q).ArtifactName = %q — not "+
				"registry-sourced", i, got[i], phase, c.ArtifactName)
		}
	}
}

// TestRequiredAccessors_ReturnCopies guards the shared-slice hazard: three
// packages assign these to package-level vars, so a returned backing array
// aliased across callers would let one consumer's append corrupt another's
// vocabulary.
func TestRequiredAccessors_ReturnCopies(t *testing.T) {
	a := RequiredRoles()
	a[0] = "mutated"
	if b := RequiredRoles(); b[0] != "scout" {
		t.Errorf("RequiredRoles() returned aliased state: second call = %v", b)
	}
	c := RequiredArtifacts()
	c[0] = "mutated"
	if d := RequiredArtifacts(); d[0] != "scout-report.md" {
		t.Errorf("RequiredArtifacts() returned aliased state: second call = %v", d)
	}
}

// TestRetroAndBuildPlanner_RegisteredWithRuntimeTruthNames pins the artifact
// half: both phases write real files whose names lived only in
// backfill.phaseHeaders and core.backfillArtifactPath before this registration.
func TestRetroAndBuildPlanner_RegisteredWithRuntimeTruthNames(t *testing.T) {
	cases := []struct{ phase, artifact, agent string }{
		{"retro", "retrospective-report.md", "retrospective"},
		{"build-planner", "build-plan.md", "build-planner"},
	}
	for _, tc := range cases {
		c, ok := For(tc.phase)
		if !ok {
			t.Errorf("For(%q): not registered", tc.phase)
			continue
		}
		if c.ArtifactName != tc.artifact {
			t.Errorf("For(%q).ArtifactName = %q, want %q", tc.phase, c.ArtifactName, tc.artifact)
		}
		if c.AgentName != tc.agent {
			t.Errorf("For(%q).AgentName = %q, want %q (must match core.phaseAgentName)",
				tc.phase, c.AgentName, tc.agent)
		}
		if c.Kind != KindMarkdown || c.WriteTarget != TargetWorkspace {
			t.Errorf("For(%q) = kind %v target %q, want markdown/workspace",
				tc.phase, c.Kind, c.WriteTarget)
		}
	}
}
