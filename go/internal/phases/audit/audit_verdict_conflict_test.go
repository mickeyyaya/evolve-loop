package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

const conflictMarker = "verdict-conflict"

func narrativeReport(verdict string) string {
	return "# Audit Report\n\n## Findings\n\nnone\n\n## Verdict\n**" + verdict + "**\n"
}

func classifyWith(t *testing.T, artifact string, prep func(ws string)) (string, []core.Diagnostic) {
	t.Helper()
	ws := t.TempDir()
	if prep != nil {
		prep(ws)
	}
	verdict, diags, _ := hooks{}.Classify(artifact, core.PhaseRequest{Workspace: ws}, core.BridgeResponse{})
	return verdict, diags
}

func conflictDiags(diags []core.Diagnostic) []core.Diagnostic {
	var out []core.Diagnostic
	for _, d := range diags {
		if strings.Contains(d.Message, conflictMarker) {
			out = append(out, d)
		}
	}
	return out
}

func requireConflict(t *testing.T, diags []core.Diagnostic, narrative string) string {
	t.Helper()
	got := conflictDiags(diags)
	if len(got) != 1 {
		t.Fatalf("want exactly 1 %q diagnostic, got %d; diags=%v", conflictMarker, len(got), diags)
	}
	if got[0].Severity != "error" {
		t.Errorf("conflict diagnostic severity=%q, want \"error\" — cyclestate.ErrorMessages "+
			"(cyclestate/result.go (ErrorMessages)) drops anything else, so the record never reaches "+
			"AuditFailReasons/the dossier: %s", got[0].Severity, got[0].Message)
	}
	if !strings.Contains(got[0].Message, narrative) {
		t.Errorf("conflict diagnostic omits the auditor's narrative verdict %q — an operator "+
			"still cannot tell a genuine defect from a poisoned predicate: %s", narrative, got[0].Message)
	}
	return got[0].Message
}

func TestVerdictConflict_RedCountBranch(t *testing.T) {
	verdict, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictReds(t, ws, "cycle1117/TestC1117_002_ProbeTreeIsolation")
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL — the deterministic gate must still outrank the narrative", verdict)
	}
	msg := requireConflict(t, diags, "PASS")
	if !strings.Contains(msg, "red") {
		t.Errorf("conflict diagnostic does not name the gate reason (red_count): %s", msg)
	}
}

func TestVerdictConflict_ShipEligibleBranch(t *testing.T) {
	no := false
	verdict, diags := classifyWith(t, narrativeReport("WARN"), func(ws string) {
		writeACSVerdictShip(t, ws, 0, &no)
	})
	if verdict != core.VerdictFAIL {
		t.Fatalf("verdict=%q, want FAIL", verdict)
	}
	requireConflict(t, diags, "WARN")
}

func TestVerdictConflict_ACSErrorBranch(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		verdict, diags := classifyWith(t, narrativeReport("PASS"), nil)
		if verdict != core.VerdictFAIL {
			t.Fatalf("verdict=%q, want FAIL", verdict)
		}
		requireConflict(t, diags, "PASS")
	})
	t.Run("unparseable", func(t *testing.T) {
		verdict, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
			if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte("{not json"), 0o644); err != nil {
				t.Fatalf("write: %v", err)
			}
		})
		if verdict != core.VerdictFAIL {
			t.Fatalf("verdict=%q, want FAIL", verdict)
		}
		requireConflict(t, diags, "PASS")
	})
}

func TestVerdictConflict_BranchesAreDistinguishable(t *testing.T) {
	_, redDiags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictReds(t, ws, "cycleX/TestRed_A")
	})
	red := requireConflict(t, redDiags, "PASS")

	no := false
	_, shipDiags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictShip(t, ws, 0, &no)
	})
	ship := requireConflict(t, shipDiags, "PASS")

	if red == ship {
		t.Errorf("red_count and ship_eligible conflicts produced IDENTICAL messages — "+
			"one fingerprint for two distinct gate reasons:\n  %s", red)
	}
	if strings.Contains(red, "\n") || strings.Contains(ship, "\n") {
		t.Errorf("conflict diagnostics must stay one line:\n  %q\n  %q", red, ship)
	}
}

func TestVerdictConflict_NarrativeVerdictIsCarriedVerbatim(t *testing.T) {
	reds := func(ws string) { writeACSVerdictReds(t, ws, "cycleX/TestRed_A") }
	_, passDiags := classifyWith(t, narrativeReport("PASS"), reds)
	_, warnDiags := classifyWith(t, narrativeReport("WARN"), reds)
	pass := requireConflict(t, passDiags, "PASS")
	warn := requireConflict(t, warnDiags, "WARN")
	if pass == warn {
		t.Errorf("narrative PASS and narrative WARN produced identical conflict records: %s", pass)
	}
}

func TestVerdictConflict_NoNoiseWhenNarrativeAlreadyFAIL(t *testing.T) {
	_, diags := classifyWith(t, narrativeReport("FAIL"), func(ws string) {
		writeACSVerdictReds(t, ws, "cycleX/TestRed_A")
	})
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict diagnostic(s) on the COHERENT narrative-FAIL case (noise): %v", len(got), got)
	}
}

func TestVerdictConflict_NoNoiseWhenNarrativeUnparseable(t *testing.T) {
	_, diags := classifyWith(t, "# Audit Report\n\nprose with no verdict declaration\n", func(ws string) {
		writeACSVerdictReds(t, ws, "cycleX/TestRed_A")
	})
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict diagnostic(s) when NO narrative verdict was parsed "+
			"(fabricated disagreement): %v", len(got), got)
	}
}

func TestVerdictConflict_NoNoiseWhenGateGreen(t *testing.T) {
	yes := true
	verdict, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		writeACSVerdictShip(t, ws, 0, &yes)
	})
	if verdict != core.VerdictPASS {
		t.Fatalf("verdict=%q, want PASS — a green gate must not be disturbed", verdict)
	}
	if got := conflictDiags(diags); len(got) != 0 {
		t.Errorf("emitted %d conflict diagnostic(s) with no override at all: %v", len(got), got)
	}
}

func TestVerdictConflict_SingleRecordPerClassify(t *testing.T) {
	_, diags := classifyWith(t, narrativeReport("PASS"), func(ws string) {
		no := false
		v := map[string]any{
			"cycle":         1124,
			"red_count":     1,
			"ship_eligible": no,
			"results": []map[string]any{
				{"ac_id": "cycleX/TestRed_A", "result": "red"},
			},
		}
		b, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), b, 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	})
	if got := conflictDiags(diags); len(got) != 1 {
		t.Errorf("want exactly 1 conflict record for a single Classify call, got %d: %v", len(got), got)
	}
}
