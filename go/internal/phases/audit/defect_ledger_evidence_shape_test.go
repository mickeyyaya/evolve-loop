package audit

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const evidenceUnparseableMarker = "is unparseable"

var oneDefect = []string{"boundary refresh does not repin the short sha"}

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
