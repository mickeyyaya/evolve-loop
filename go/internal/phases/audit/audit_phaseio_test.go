package audit

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/config"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// ADR-0050 §3.10 Slice 5: at enforce the machine-readable evolve-verdict sentinel
// is mandatory — the legacy prose/regex verdict fallbacks are gated off. Below
// enforce (off/shadow/advisory) every path stays active, byte-identical.

const auditSentinelPASS = "<!-- evolve-verdict: {\"phase\":\"audit\",\"verdict\":\"PASS\",\"schema_version\":1} -->"

// A prose-only report (canonical heading, NO sentinel) is read below enforce but
// NOT at enforce (sentinel mandatory there).
func TestExtractAuditVerdict_EnforceSentinelOnly(t *testing.T) {
	prose := "## Verdict\n**PASS**\n"
	for _, st := range []config.Stage{config.StageOff, config.StageShadow, config.StageAdvisory} {
		if v, ok := extractAuditVerdict(prose, st); !ok || v != core.VerdictPASS {
			t.Errorf("stage %s: prose verdict must be read below enforce, got (%q,%v)", st, v, ok)
		}
	}
	if v, ok := extractAuditVerdict(prose, config.StageEnforce); ok {
		t.Errorf("enforce: prose-only report must NOT yield a verdict (sentinel mandatory), got (%q,%v)", v, ok)
	}
}

// The sentinel is honored at EVERY stage — gating the prose fallback must never
// touch the sentinel path. Prose here says FAIL, the sentinel says PASS; sentinel
// wins at both off and enforce.
func TestExtractAuditVerdict_SentinelWinsBothStages(t *testing.T) {
	withSentinel := "## Verdict\n**FAIL**\n" + auditSentinelPASS + "\n"
	for _, st := range []config.Stage{config.StageOff, config.StageEnforce} {
		v, ok := extractAuditVerdict(withSentinel, st)
		if !ok || v != core.VerdictPASS {
			t.Errorf("stage %s: sentinel must win, got (%q,%v) want (PASS,true)", st, v, ok)
		}
	}
}
