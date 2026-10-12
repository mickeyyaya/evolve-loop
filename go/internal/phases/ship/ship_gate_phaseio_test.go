package ship

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

func TestShipGate_EnforceSentinelFirst(t *testing.T) {
	t.Parallel()
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

func TestShipGate_DualVerdictImpossibleAtEnforce(t *testing.T) {
	t.Parallel()
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

func TestShipGate_EnforceNoSentinelIsMalformed(t *testing.T) {
	t.Parallel()
	body := "## Verdict\n**PASS**\n"
	pass, warn, fail := parseVerdicts(body, config.StageEnforce)
	if pass || warn || fail {
		t.Errorf("enforce: prose-only report must yield NO verdict (sentinel mandatory), got pass=%v warn=%v fail=%v", pass, warn, fail)
	}
	if p, _, _ := parseVerdicts(body, config.StageOff); !p {
		t.Error("off: prose PASS must still be read")
	}
}

func TestShipGate_EnforceRejectsForeignPhaseSentinel(t *testing.T) {
	t.Parallel()
	body := phasecontract.RenderVerdictSentinel("build", "PASS") + "\n"
	pass, warn, fail := parseVerdicts(body, config.StageEnforce)
	if pass || warn || fail {
		t.Errorf("enforce: a build-phase sentinel must not satisfy the audit gate, got pass=%v warn=%v fail=%v", pass, warn, fail)
	}
}

func TestShipGate_EnforceSkippedSentinelIsMalformed(t *testing.T) {
	t.Parallel()
	body := phasecontract.RenderVerdictSentinel("audit", "SKIPPED") + "\n"
	if pass, warn, fail := parseVerdicts(body, config.StageEnforce); pass || warn || fail {
		t.Errorf("enforce: SKIPPED sentinel must yield no verdict, got pass=%v warn=%v fail=%v", pass, warn, fail)
	}
}
