package phasecontract

import "testing"

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

func TestArtifactName_ResolvesFromRegistryAndSkipsNoArtifact(t *testing.T) {
	for _, phase := range []string{"scout", "build", "audit", "tdd", "triage", "intent"} {
		c, ok := For(phase)
		if !ok {
			t.Fatalf("premise broken: phase %q is not registered", phase)
		}
		if got := ArtifactName(phase); got != c.ArtifactName {
			t.Errorf("ArtifactName(%q) = %q, want the registry value %q", phase, got, c.ArtifactName)
		}
		if ArtifactName(phase) == "" {
			t.Errorf("ArtifactName(%q) is empty — consumers would join a bare directory path", phase)
		}
	}

	if ArtifactName("advisor") != ArtifactName("router") {
		t.Errorf("ArtifactName(\"advisor\") = %q, want the canonical router value %q",
			ArtifactName("advisor"), ArtifactName("router"))
	}

	if got := ArtifactName("ship"); got != "" {
		t.Errorf("ArtifactName(\"ship\") = %q, want \"\" — ship is NoArtifact, so a caller must fall back rather than write a file", got)
	}
	if got := ArtifactName("no-such-phase-cycle1145"); got != "" {
		t.Errorf("ArtifactName(unregistered) = %q, want \"\"", got)
	}
}
