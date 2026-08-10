package audit

// defect_ledger_evidence_edge_test.go — edge-case pins for the cycle-1403
// tolerant-evidence fix (#422), added after the 2026-08-09 zero-ship batch
// postmortem (docs/incidents/2026-08-09-zero-ship-batch.md). The base suite
// (defect_ledger_evidence_shape_test.go) pins string/array/empty/object
// shapes; these cases close the corners the adversarial review left
// UNVERIFIED or noted as untested:
//   - mixed-type array (["cite", 42]) — encoding/json rejects mid-decode;
//     must fail CLOSED, never PASS, never crash.
//   - null evidence on FIXED — "evidence": null decodes to the zero value;
//     must be treated as no evidence.
//   - whitespace-only string — trim must not admit "   " as a citation.
//   - literal "; " inside ONE string — the join token doubles as a split
//     token, so a semicolon-joined pair behaves exactly like the array form:
//     both halves must resolve (stricter-never-looser, pinned both ways).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestClassify_DispositionEvidenceMixedTypeArrayFailsClosed(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": []any{cite, 42}},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("a mixed-type evidence array ([string, number]) must fail closed — admitting it would let a malformed claim close an inherited defect. diagnostics:\n%s", diagsText(diags))
	}
}

func TestClassify_DispositionEvidenceNullOnFixedStillBlocks(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": nil},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("`evidence: null` on a FIXED claim is an unevidenced closure and must block. diagnostics:\n%s", diagsText(diags))
	}
}

func TestClassify_DispositionEvidenceWhitespaceOnlyStillBlocks(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": "   \t  "},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict == core.VerdictPASS {
		t.Errorf("whitespace-only evidence must be treated as no evidence — trimming must not admit it. diagnostics:\n%s", diagsText(diags))
	}
}

func TestClassify_DispositionEvidenceSemicolonJoinedBothResolvePasses(t *testing.T) {
	// The "; " join token doubles as a split token: one string carrying two
	// REAL citations must behave exactly like the two-element array form.
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	cite1 := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	cite2 := evidenceFile(t, req.ProjectRoot, "go/internal/core/cyclerun.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite1 + "; " + cite2},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("a semicolon-joined pair of resolvable citations must be honoured exactly like the array form; got %q. diagnostics:\n%s", verdict, diagsText(diags))
	}
}

func TestClassify_DispositionEvidenceSemicolonJoinedOneMissingBlocks(t *testing.T) {
	// AND-semantics survive the split: one resolvable + one phantom half
	// must block — the split can only ever make acceptance stricter.
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite + "; go/missing/phantom.go:9"},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if verdict == core.VerdictPASS {
		t.Errorf("one unresolvable half of a semicolon-joined citation must block the claim (AND-semantics). diagnostics:\n%s", text)
	}
	if strings.Contains(text, evidenceUnparseableMarker) {
		t.Errorf("the block must come from resolution, not parsing. diagnostics:\n%s", text)
	}
}
