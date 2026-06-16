package skillcheck

import (
	"strings"
	"testing"
)

// TestRun_WriteMode_NoDrift: write=true with a clean tree must exit 0 and must
// NOT emit any error output. Write mode differs from check mode here: it does
// not necessarily emit "check OK" (check mode does; write mode may omit it when
// nothing was written). The critical contract is exit 0 and no DRIFT: on stderr.
func TestRun_WriteMode_NoDrift(t *testing.T) {
	tmp := prepareSkillsTree(t)
	var stdout, stderr strings.Builder
	code := Run(tmp, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("Run write no-drift: exit %d, want 0; stdout=%q stderr=%q",
			code, stdout.String(), stderr.String())
	}
	// Write mode with no drift must not produce any error signal on stderr.
	if strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("Run write no-drift: DRIFT: must not appear on stderr when tree is clean; got %q", stderr.String())
	}
}

// TestRun_CheckMode_Drift_OutputIsolation: when drift is detected, DRIFT: must
// appear ONLY on stderr and NOT bleed onto stdout. Callers that redirect stdout
// to a log must not receive error noise inline with any status output.
func TestRun_CheckMode_Drift_OutputIsolation(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	var stdout, stderr strings.Builder
	code := Run(tmp, false, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("Run check drift: exit %d, want 2", code)
	}
	// DRIFT: must be on stderr.
	if !strings.Contains(stderr.String(), "DRIFT:") {
		t.Errorf("want 'DRIFT:' on stderr; got stderr=%q", stderr.String())
	}
	// DRIFT: must NOT leak to stdout.
	if strings.Contains(stdout.String(), "DRIFT:") {
		t.Errorf("DRIFT: must not appear on stdout; got stdout=%q", stdout.String())
	}
}

// TestRun_WriteMode_Idempotent: after write mode repairs drift, a subsequent
// check-mode run must see a clean tree (exit 0). This tests that write mode
// leaves the tree in a state consistent with future no-drift checks.
func TestRun_WriteMode_Idempotent(t *testing.T) {
	tmp := prepareSkillsTree(t)
	mutateBuildSkill(t, tmp)

	// First pass: write=true should repair the drift.
	var out1, err1 strings.Builder
	code1 := Run(tmp, true, &out1, &err1)
	if code1 != 0 {
		t.Fatalf("write mode (first pass): exit %d; stdout=%q stderr=%q",
			code1, out1.String(), err1.String())
	}

	// Second pass: write=false should find the tree clean.
	var out2, err2 strings.Builder
	code2 := Run(tmp, false, &out2, &err2)
	if code2 != 0 {
		t.Fatalf("check mode (after write repair): exit %d, want 0 (tree should be clean); stdout=%q stderr=%q",
			code2, out2.String(), err2.String())
	}
}
