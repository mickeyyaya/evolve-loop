package triagecap

import "testing"

func TestAmplifiedCommittedFloorPackagesDeclarationFiltersCandidatesAndOverridesProse(t *testing.T) {
	artifact := "## top_n\n" +
		"- prose-only-core: raise core coverage to >=95% -- priority=H, evidence=scout, source=scout\n"
	comp := writeCompanion(t, t.TempDir(), []string{"ledger", "unknown-package", "clihealth"})

	got := CommittedFloorPackages(artifact, comp, knownPkgsFixture)
	want := map[string]bool{"ledger": true, "clihealth": true}
	if len(got) != len(want) {
		t.Fatalf("CommittedFloorPackages = %v, want exactly ledger and clihealth", got)
	}
	for _, pkg := range got {
		if !want[pkg] {
			t.Fatalf("CommittedFloorPackages included %q from %v, want only known declared packages", pkg, got)
		}
		delete(want, pkg)
	}
	for missing := range want {
		t.Fatalf("CommittedFloorPackages missing declared known package %q from %v", missing, got)
	}
}
