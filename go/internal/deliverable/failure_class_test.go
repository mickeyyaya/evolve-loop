package deliverable

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Cycle 1684 (2026-09-15): the audit declared failure class
// "superseded-predicate-contradiction" — a class no policy category knows —
// the gate verified the deliverable, and the retry envelope declined the
// repair round on "unrecognised class" with no log line and no signal: a
// one-phase builder fix (retire the superseded predicate) went through a
// full retrospective first and re-entered tdd/build with none of the audit's
// findings in its briefs. The class drives the envelope, so the gate
// validates it at the boundary and the correction hands the auditor the
// vocabulary.
func failureExemplarFor(class string) *phasecontract.FailureBlock {
	return &phasecontract.FailureBlock{Class: class, Defects: []string{"EGPS: red_count=1 [TestC1515_006]"}, EvidencePaths: []string{"acs-verdict.json"}}
}

func TestVerify_AuditFailureClassOutsideTheVocabularyIsAViolation(t *testing.T) {
	report := func(class string) string {
		fb := failureExemplarFor(class)
		return failReport("audit", "", true) + "\n" + phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL", fb) + "\n"
	}
	cases := []struct {
		name, class string
		wantCode    bool
	}{
		{"an invented class is refused", "superseded-predicate-contradiction", true},
		{"the canonical audit class passes", "code-audit-fail", false},
		{"a legacy alias passes (NormalizeLegacy)", "audit-fail", false},
		{"a system-level class is still vocabulary", "infrastructure-systemic", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			writeFile(t, ws, "audit-report.md", report(tc.class))
			res, err := VerifyCatalogAwareStage("audit", phasecontract.Roots{Workspace: ws}, config.StageEnforce)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := hasCode(res, CodeFailureClassUnknown); got != tc.wantCode {
				t.Fatalf("failure_class_unknown = %v, want %v: %+v", got, tc.wantCode, res.Violations)
			}
			if tc.wantCode {
				var msg string
				for _, v := range res.Violations {
					if v.Code == CodeFailureClassUnknown {
						msg = v.Message
					}
				}
				if !strings.Contains(msg, tc.class) || !strings.Contains(msg, "code-audit-fail") || !strings.Contains(msg, "infrastructure-transient") {
					t.Errorf("the correction names the offending class and the vocabulary so the auditor can re-emit exactly: %q", msg)
				}
			}
		})
	}
}
