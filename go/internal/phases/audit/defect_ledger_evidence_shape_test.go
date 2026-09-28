package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Contract shape. Every case reaches its subject through the REAL production
// seam, hooks{}.Classify — the audit verdict path — never readDispositions
// directly: a decoder that parses an array while the gate still blocks would be
// a fix nobody can use.
//
// Adversarial diversity (skills/adversarial-testing §6):
//   - regression  — the string shape that works today must keep working.
//   - new/positive — array of resolvable cites now PASSes (AC2, the crux).
//   - negative    — array of UNRESOLVABLE cites must still block, and must not
//     be reported as "unparseable": tolerance may not become a bypass.
//   - edge        — an empty array on a FIXED claim is "no evidence", still a
//     block.
//   - negative    — a shape that is neither string nor array (an object) must
//     still be rejected outright; no silent degrade to "".

// evidenceUnparseableMarker is the substring readDispositions uses for the
// blocking parse-failure diagnostic. Several cases below assert its ABSENCE:
// after the fix, an array-shaped file is a file the gate read, whatever it
// then decides about the claim.
const evidenceUnparseableMarker = "is unparseable"

// oneDefect is the inherited-defect text used by every case here. One defect
// keeps each assertion about the EVIDENCE SHAPE rather than about coverage
// arithmetic, which disposition_preflight_test.go already pins.
var oneDefect = []string{"boundary refresh does not repin the short sha"}

// TestClassify_DispositionEvidenceStringShapeAccepted — REGRESSION. The shape
// that works today: `evidence` is a plain string citing a real file. This is
// expected to be GREEN before the fix and must STAY green after it; a tolerant
// decoder that breaks the ordinary case has traded one outage for another.
func TestClassify_DispositionEvidenceStringShapeAccepted(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	cite := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": cite},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})

	if verdict != core.VerdictPASS {
		t.Errorf("AC1 unmet — a string-shaped `evidence` citing a real file is the shape in production use today and must PASS; got %q. diagnostics:\n%s", verdict, diagsText(diags))
	}
}

// TestClassify_DispositionEvidenceArrayShapeAccepted — AC2, THE CRUX.
// `evidence` is a JSON array of two citations, both resolving to real files.
// The gate must read the file (no "unparseable") and honour the closure
// (PASS). RED today: encoding/json refuses the document before any
// resolution logic runs.
func TestClassify_DispositionEvidenceArrayShapeAccepted(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	cite1 := evidenceFile(t, req.ProjectRoot, "go/internal/core/fleet.go")
	cite2 := evidenceFile(t, req.ProjectRoot, "go/internal/core/cyclerun.go")
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": []string{cite1, cite2}},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if strings.Contains(text, evidenceUnparseableMarker) {
		t.Errorf("AC2 unmet — an array-shaped `evidence` must be READ, not rejected as an unparseable document (the cycle-1399 failure). diagnostics:\n%s", text)
	}
	if verdict != core.VerdictPASS {
		t.Errorf("AC2 unmet — a FIXED claim whose `evidence` is an array of resolvable citations must be honoured exactly as the single-string form is; got %q. A decoder that parses the array but leaves the citations unresolvable (e.g. joining them into one non-path string) does NOT satisfy this criterion. diagnostics:\n%s", verdict, text)
	}
}

// TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks —
// NEGATIVE, the anti-gaming half. Tolerance must widen the accepted SHAPE, not
// the accepted CLAIM. An array whose citations name no real file is an
// unevidenced closure and must still block — and must do so on resolution, not
// on parsing, so the operator is told the citations are wrong rather than that
// their file is garbage.
func TestClassify_DispositionEvidenceArrayShapeUnresolvableStillBlocks(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED",
				"evidence": []string{"go/internal/core/does-not-exist.go:12", "also/missing.go:3"}},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if verdict == core.VerdictPASS {
		t.Errorf("AC3 unmet — array-shape tolerance must not admit a FIXED claim whose citations resolve to no file; that is the unevidenced closure the ledger exists to block. diagnostics:\n%s", text)
	}
	if strings.Contains(text, evidenceUnparseableMarker) {
		t.Errorf("AC3 unmet — the block must come from RESOLUTION, not from a parse failure: after the fix the array shape is readable, so an operator must be told the citations do not resolve. diagnostics:\n%s", text)
	}
}

// TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks — EDGE. `[]` is
// a well-formed array carrying no citation at all. It must be treated as the
// existing empty-evidence case (block), never as "an array was supplied, good
// enough" — the boundary where a permissive decoder most easily becomes a
// bypass.
func TestClassify_DispositionEvidenceEmptyArrayOnFixedStillBlocks(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED", "evidence": []string{}},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if verdict == core.VerdictPASS {
		t.Errorf("AC4 unmet — an empty `evidence` array is a FIXED claim with no evidence and must block exactly as `\"evidence\": \"\"` does. diagnostics:\n%s", text)
	}
	if strings.Contains(text, evidenceUnparseableMarker) {
		t.Errorf("AC4 unmet — `[]` is a legal array; the block must be the unevidenced-closure block, not a parse failure. diagnostics:\n%s", text)
	}
}

// TestClassify_DispositionEvidenceObjectShapeStillBlocks — NEGATIVE. Neither
// string nor array-of-strings: an object. This must keep hitting the
// unparseable path and BLOCK. Silently degrading an unrecognised shape to ""
// would hand the gate its cheapest bypass.
func TestClassify_DispositionEvidenceObjectShapeStillBlocks(t *testing.T) {
	ws, req := continuationFixture(t, 1398, 1403, oneDefect)
	writeJSON(t, filepath.Join(ws, dispositionFile), map[string]any{
		"dispositions": []any{
			map[string]any{"id": "d1", "status": "FIXED",
				"evidence": map[string]any{"path": "go/internal/core/fleet.go", "line": 12}},
		},
	})

	verdict, diags, _ := hooks{}.Classify(narrativeReport("PASS"), req, core.BridgeResponse{})
	text := diagsText(diags)

	if verdict == core.VerdictPASS {
		t.Errorf("AC5 unmet — an `evidence` value that is neither a string nor an array of strings must never be silently degraded to empty and allowed to PASS. diagnostics:\n%s", text)
	}
	if !strings.Contains(text, evidenceUnparseableMarker) {
		t.Errorf("AC5 unmet — an unrecognised `evidence` shape must keep reporting %s so the operator learns the file was rejected rather than quietly emptied. diagnostics:\n%s", evidenceUnparseableMarker, text)
	}
}
