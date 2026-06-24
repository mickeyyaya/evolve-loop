package ship

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// ADR-0050 §3.10 Slice 6: at enforce the ship gate's verdict parse is sentinel-first
// — the single-valued evolve-verdict sentinel is authoritative and the prose regex
// (which can match multiple verdict words and trip the dual-verdict guard) is gated
// off. Below enforce parseVerdicts stays prose-only — byte-identical.

// The sentinel wins over a conflicting prose verdict at enforce; below enforce the
// prose verdict is what's read (the sentinel JSON does not trip the prose regex).
func TestShipGate_EnforceSentinelFirst(t *testing.T) {
	body := "## Verdict\n**FAIL**\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"

	pass, _, fail := parseVerdicts(body, config.StageOff)
	if pass || !fail {
		t.Errorf("off: prose parse must read FAIL (not the sentinel), got pass=%v fail=%v", pass, fail)
	}
	pass, _, fail = parseVerdicts(body, config.StageEnforce)
	if !pass || fail {
		t.Errorf("enforce: sentinel PASS must win over prose FAIL, got pass=%v fail=%v", pass, fail)
	}
}

// A dual prose verdict (both PASS and FAIL declared) is collapsed to the single
// sentinel verdict at enforce, so the (fail && pass) state the ship gate rejects
// (CodeAuditBindingDualVerdict) becomes structurally impossible.
func TestShipGate_DualVerdictImpossibleAtEnforce(t *testing.T) {
	body := "## Verdict\n**FAIL**\n\nVerdict: PASS\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n"

	pass, _, fail := parseVerdicts(body, config.StageOff)
	if !(pass && fail) {
		t.Fatalf("precondition: off prose parse must see the dual verdict, got pass=%v fail=%v", pass, fail)
	}
	pass, _, fail = parseVerdicts(body, config.StageEnforce)
	if pass && fail {
		t.Errorf("enforce: dual verdict must be impossible (sentinel is single-valued), got pass=%v fail=%v", pass, fail)
	}
	if !pass {
		t.Errorf("enforce: the sentinel PASS must be read, got pass=%v", pass)
	}
}

// No sentinel at enforce → no verdict found (all false), which verifyAuditBinding
// maps to CodeAuditBindingMalformed. This is the sentinel-mandatory enforcement.
func TestShipGate_EnforceNoSentinelIsMalformed(t *testing.T) {
	body := "## Verdict\n**PASS**\n" // prose only, no sentinel
	pass, warn, fail := parseVerdicts(body, config.StageEnforce)
	if pass || warn || fail {
		t.Errorf("enforce: prose-only report must yield NO verdict (sentinel mandatory), got pass=%v warn=%v fail=%v", pass, warn, fail)
	}
	// Below enforce the same prose IS read.
	if p, _, _ := parseVerdicts(body, config.StageOff); !p {
		t.Error("off: prose PASS must still be read")
	}
}

// A foreign-phase sentinel (e.g. a build-report sentinel quoted into the audit
// artifact) must NOT satisfy the audit ship gate at enforce — only an "audit"-phase
// sentinel is trusted. Otherwise a build PASS could ship a failing build.
func TestShipGate_EnforceRejectsForeignPhaseSentinel(t *testing.T) {
	body := phasecontract.RenderVerdictSentinel("build", "PASS") + "\n"
	pass, warn, fail := parseVerdicts(body, config.StageEnforce)
	if pass || warn || fail {
		t.Errorf("enforce: a build-phase sentinel must not satisfy the audit gate, got pass=%v warn=%v fail=%v", pass, warn, fail)
	}
}

// A SKIPPED (or any out-of-vocab) audit sentinel at enforce maps to no verdict
// (all false → CodeAuditBindingMalformed): SKIPPED is not a legitimate ship-gate
// verdict.
func TestShipGate_EnforceSkippedSentinelIsMalformed(t *testing.T) {
	body := phasecontract.RenderVerdictSentinel("audit", "SKIPPED") + "\n"
	if pass, warn, fail := parseVerdicts(body, config.StageEnforce); pass || warn || fail {
		t.Errorf("enforce: SKIPPED sentinel must yield no verdict, got pass=%v warn=%v fail=%v", pass, warn, fail)
	}
}
