package dossier

import "testing"

// These tests cover BuildOpts.FinalVerdict — the extension that lets the cycle
// producer (core.RunCycle, ADR-0055) record a cycle's REAL outcome instead of an
// always-PASS skeleton. A FAIL dossier must still satisfy Validate (>=1 defect +
// >=1 carryover), so Build synthesizes a minimal, truthful pair pointing at the
// audit artifacts rather than fabricating a PASS for a failed cycle.

func TestBuild_FinalVerdictDefaultsToPass(t *testing.T) {
	d, err := Build(1, BuildOpts{WorkspacePath: "/w", Goal: "g"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.FinalVerdict != VerdictPass {
		t.Errorf("default FinalVerdict = %q, want %q (back-compat)", d.FinalVerdict, VerdictPass)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("default dossier must be valid: %v", err)
	}
}

func TestBuild_FinalVerdictWarn_PassesValidate(t *testing.T) {
	d, err := Build(2, BuildOpts{WorkspacePath: "/w", Goal: "g", FinalVerdict: VerdictWarn})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.FinalVerdict != VerdictWarn {
		t.Errorf("FinalVerdict = %q, want %q", d.FinalVerdict, VerdictWarn)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("WARN dossier must validate without defects: %v", err)
	}
}

func TestBuild_FinalVerdictFail_PassesValidate(t *testing.T) {
	d, err := Build(3, BuildOpts{WorkspacePath: "/w", Goal: "g", FinalVerdict: VerdictFail})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if d.FinalVerdict != VerdictFail {
		t.Errorf("FinalVerdict = %q, want %q", d.FinalVerdict, VerdictFail)
	}
	if len(d.Defects) == 0 {
		t.Error("FAIL build must synthesize >=1 defect so the failure's reason is recorded")
	}
	if len(d.Carryover) == 0 {
		t.Error("FAIL build must synthesize >=1 carryover so the fix-work is recorded")
	}
	if err := d.Validate(); err != nil {
		t.Errorf("FAIL dossier must satisfy Validate: %v", err)
	}
}

func TestBuild_FinalVerdictUnknown_Errors(t *testing.T) {
	if _, err := Build(4, BuildOpts{WorkspacePath: "/w", Goal: "g", FinalVerdict: "BOGUS"}); err == nil {
		t.Error("Build with an unknown FinalVerdict must error, not silently default")
	}
}
