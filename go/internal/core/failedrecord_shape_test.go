package core

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
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
