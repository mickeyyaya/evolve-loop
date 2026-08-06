package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// defect_ledger_cite_annotation_test.go — the decorated-cite long-tail
// (2026-08-06, evidence-cite-annotation-tolerance).
//
// Two independent chains decorated otherwise-VALID path:range cites with a
// trailing parenthetical annotation and ground on "resolves to no file":
// cycle-1356 "go/internal/phases/triage/triage.go:114-129
// (carryforwardCandidatesTimestamp...)" and cycle-~1360
// "go/internal/core/runlease_hook.go:56-73 (stale lease)". The annotation is
// reasonable agent output, not gaming — but evidenceResolves strips only
// numeric :suffixes, so the whole decorated string stats as a nonexistent
// path and the agent, believing its cite correct, re-decorates every round
// (the accretion grind). Tolerance: ONE trailing " (…)" group is DROPPED
// before resolution. Every anti-gaming rejection must survive: the stripped
// path still has to be a real, repo-relative, non-self-vouching regular file.

// TestClassify_AnnotatedRangeCiteCloses — POSITIVE, the live fixture shape.
func TestClassify_AnnotatedRangeCiteCloses(t *testing.T) {
	ws, wt, req := worktreeContinuationFixture(t, 1350, 1356, []string{"carryforward candidates timestamp is stale"})
	evidenceFile(t, wt, "go/internal/phases/triage/triage.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED",
				"evidence": "go/internal/phases/triage/triage.go:114-129 (carryforwardCandidatesTimestamp now cycle-scoped)",
				"reason":   "fix landed in this lane's worktree"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("a valid path:N-M cite with a trailing parenthetical annotation must be accepted (annotation dropped, path resolved); verdict = %q\ndiagnostics:\n%s", verdict, diagsText(diags))
	}
}

// TestClassify_AnnotationCannotLaunderARejection — NEGATIVE table: the
// tolerance strips decoration, never weakens a rejection. Each row is a cite
// that must STILL block after stripping (or because stripping does not apply).
func TestClassify_AnnotationCannotLaunderARejection(t *testing.T) {
	cases := []struct {
		name, evidence string
	}{
		{"annotation-only cite", "(see the fix above)"},
		{"empty path before annotation", " (all better now)"},
		{"nonexistent path with annotation", "go/internal/nowhere/imaginary.go:12-40 (definitely fixed)"},
		{"root escape with annotation", "../outside.go:3-9 (fixed elsewhere)"},
		{"self-vouch with annotation", "defect-ledger.json:1-5 (the ledger says so)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ws, wt, req := worktreeContinuationFixture(t, 1350, 1356, []string{"carryforward candidates timestamp is stale"})
			// A real repo file exists so only the CITE is at fault.
			evidenceFile(t, wt, "go/internal/phases/triage/triage.go")
			writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
				"dispositions": []any{
					map[string]any{"id": "d1", "status": "FIXED", "evidence": c.evidence, "reason": "claims closure"},
				},
			})

			verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

			if verdict == core.VerdictPASS {
				t.Errorf("cite %q must still block PASS — annotation tolerance strips decoration from a VALID cite, it never launders an invalid one\ndiagnostics:\n%s", c.evidence, diagsText(diags))
			}
			if text := diagsText(diags); !strings.Contains(text, "d1") {
				t.Errorf("the rejected closure must be named by id; diagnostics:\n%s", text)
			}
		})
	}
}
