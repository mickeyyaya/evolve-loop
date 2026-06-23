package core

// failedrecord_shape_test.go — R7 shape-parity pin: state.json:
// failedApproaches has TWO appenders with different Go types — the
// orchestrator's typed core.FailedRecord (in-memory State, persisted via
// writeFailureLearningState) and failurelog.Recorded (raw read-modify-
// write; reset + loop fatals). Unifying them is out of scope (State
// lifecycle vs on-disk merge), so this test pins the JSON contract
// instead: every key failurelog.Recorded emits must also be a key of
// core.FailedRecord, with identical spelling. If this fails, the two
// appenders have drifted and downstream readers (failure-adapter,
// pruner) see a forked schema.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolveloop/go/internal/failurelog"
	"github.com/mickeyyaya/evolveloop/go/internal/phasecontract"
)

func jsonKeys(t *testing.T, v any) map[string]struct{} {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %T: %v", v, err)
	}
	keys := make(map[string]struct{}, len(m))
	for k := range m {
		keys[k] = struct{}{}
	}
	return keys
}

func TestFailedRecord_SupersetOfFailurelogRecordedShape(t *testing.T) {
	now := time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC)
	recordedKeys := jsonKeys(t, failurelog.Recorded{
		Cycle:          1,
		Classification: failurelog.LoopFatal,
		Summary:        "stop_reason=error",
		RecordedAt:     now.Format(time.RFC3339),
		ExpiresAt:      now.Format(time.RFC3339),
	})
	failedKeys := jsonKeys(t, FailedRecord{
		TS:             now.Format(time.RFC3339),
		Cycle:          1,
		Verdict:        VerdictFAIL,
		Classification: "cycle-mid-execution-fail",
		RecordedAt:     now.Format(time.RFC3339),
		ExpiresAt:      now.Format(time.RFC3339),
		Summary:        "x",
		Defects:        []string{"x"},
		Retrospected:   true,
	})

	for k := range recordedKeys {
		if _, ok := failedKeys[k]; !ok {
			t.Errorf("failurelog.Recorded emits key %q that core.FailedRecord does not — the two failedApproaches appenders drifted", k)
		}
	}
}

// adoptStructuredFailure is the trust boundary for agent-written failure
// blocks: out-of-taxonomy classes are refused (they would round-trip to
// UnknownClassification on the next state read) and sizes are capped.
func TestAdoptStructuredFailure_TrustBoundary(t *testing.T) {
	ws := t.TempDir()
	write := func(class string, defects []string) {
		body := "## Triage\nFAIL\n" + phasecontract.RenderVerdictSentinelWithFailure("triage", "FAIL",
			&phasecontract.FailureBlock{Class: class, Defects: defects}) + "\n"
		if err := os.WriteFile(filepath.Join(ws, "triage-report.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("totally-novel-class", []string{"d"})
	if fb := adoptStructuredFailure(ws, "triage"); fb != nil {
		t.Errorf("out-of-taxonomy class must be refused; got %+v", fb)
	}

	big := strings.Repeat("x", 2000)
	many := make([]string, 50)
	for i := range many {
		many[i] = big
	}
	write("code-build-fail", many)
	fb := adoptStructuredFailure(ws, "triage")
	if fb == nil {
		t.Fatal("canonical class must be adopted")
	}
	if len(fb.Defects) != maxAdoptedDefects {
		t.Errorf("defect list not capped: %d", len(fb.Defects))
	}
	if r := []rune(fb.Defects[0]); len(r) != maxAdoptedDefectRunes+1 { // +1 for the ellipsis
		t.Errorf("defect entry not capped: %d runes", len(r))
	}
}
