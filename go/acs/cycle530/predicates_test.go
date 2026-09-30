//go:build acs

package cycle530

import (
	"os"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

func toolUse(seq int64, name, excerpt string) phasestream.Envelope {
	return phasestream.Envelope{
		SchemaVersion: phasestream.SchemaVersion,
		Seq:           seq,
		Kind:          phasestream.KindToolUse,
		Severity:      phasestream.SeverityInfo,
		Data:          map[string]any{"name": name, "id": name, "input_excerpt": excerpt},
	}
}

func toolResult(seq int64, id, excerpt string) phasestream.Envelope {
	return phasestream.Envelope{
		SchemaVersion: phasestream.SchemaVersion,
		Seq:           seq,
		Kind:          phasestream.KindToolResult,
		Severity:      phasestream.SeverityInfo,
		Data:          map[string]any{"tool_use_id": id, "is_error": false, "excerpt": excerpt},
	}
}

func isMasked(e phasestream.Envelope) bool {
	v, ok := e.Data["masked"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

func contentOf(e phasestream.Envelope) any {
	switch e.Kind {
	case phasestream.KindToolUse:
		return e.Data["input_excerpt"]
	case phasestream.KindToolResult:
		return e.Data["excerpt"]
	default:
		return nil
	}
}

func TestC530_001_MasksToolObservationsOlderThanWindow(t *testing.T) {
	in := make([]phasestream.Envelope, 0, 12)
	for i := int64(1); i <= 12; i++ {
		if i%2 == 1 {
			in = append(in, toolUse(i, "read", "ORIGINAL-USE-content-seq"))
		} else {
			in = append(in, toolResult(i, "read", "ORIGINAL-RESULT-content-seq"))
		}
	}
	out := phasestream.MaskStaleObservations(in, 4)
	if len(out) != len(in) {
		t.Fatalf("MaskStaleObservations changed slice length: got %d want %d", len(out), len(in))
	}
	maskedCount := 0
	for _, e := range out {
		if isMasked(e) {
			maskedCount++
		}
	}
	if maskedCount != 8 {
		t.Fatalf("windowTurns=4 over 12 evictable observations must mask the 8 oldest, got %d masked", maskedCount)
	}
	for _, e := range out {
		switch {
		case e.Seq <= 8:
			if !isMasked(e) {
				t.Errorf("Seq %d (older than window) must be masked but was not; data=%v", e.Seq, e.Data)
			}
			if c, _ := contentOf(e).(string); c == "ORIGINAL-USE-content-seq" || c == "ORIGINAL-RESULT-content-seq" {
				t.Errorf("Seq %d masked envelope still carries original content %q", e.Seq, c)
			}
		case e.Seq >= 9:
			if isMasked(e) {
				t.Errorf("Seq %d (inside window) must NOT be masked; data=%v", e.Seq, e.Data)
			}
			if c, _ := contentOf(e).(string); c != "ORIGINAL-USE-content-seq" && c != "ORIGINAL-RESULT-content-seq" {
				t.Errorf("Seq %d in-window content was altered: %q", e.Seq, c)
			}
		}
	}
}

func TestC530_002_NeverEvictsVerdictAndErrorEvenWhenOld(t *testing.T) {
	verdict := phasestream.Envelope{
		Seq: 1, Kind: phasestream.KindResult, Severity: phasestream.SeverityInfo,
		Data: map[string]any{"is_error": false, "num_turns": int64(7)},
	}
	errEnv := phasestream.Envelope{
		Seq: 2, Kind: phasestream.KindError, Severity: phasestream.SeverityIncident,
		Data: map[string]any{"message": "boom: current failing state"},
	}
	wantVerdict := map[string]any{"is_error": false, "num_turns": int64(7)}
	wantErr := map[string]any{"message": "boom: current failing state"}

	in := []phasestream.Envelope{verdict, errEnv}
	for i := int64(3); i <= 10; i++ {
		in = append(in, toolUse(i, "read", "recent-tool-content"))
	}
	out := phasestream.MaskStaleObservations(in, 2)

	for _, e := range out {
		switch e.Kind {
		case phasestream.KindResult:
			if isMasked(e) {
				t.Errorf("verdict (KindResult) was masked — never-evict class violated; data=%v", e.Data)
			}
			if !reflect.DeepEqual(e.Data, wantVerdict) {
				t.Errorf("verdict Data mutated: got %v want %v", e.Data, wantVerdict)
			}
		case phasestream.KindError:
			if isMasked(e) {
				t.Errorf("error (KindError) was masked — never-evict class violated; data=%v", e.Data)
			}
			if !reflect.DeepEqual(e.Data, wantErr) {
				t.Errorf("error Data mutated: got %v want %v", e.Data, wantErr)
			}
		}
	}
}

func TestC530_003_KeepsObservationsInsideWindow(t *testing.T) {
	in := []phasestream.Envelope{
		toolUse(1, "read", "u1"),
		toolResult(2, "read", "r2"),
		toolUse(3, "grep", "u3"),
	}
	out := phasestream.MaskStaleObservations(in, 10)
	if len(out) != len(in) {
		t.Fatalf("length changed: got %d want %d", len(out), len(in))
	}
	for i, e := range out {
		if isMasked(e) {
			t.Errorf("observation %d is inside the window (3 <= 10) and must not be masked; data=%v", i, e.Data)
		}
		if !reflect.DeepEqual(contentOf(e), contentOf(in[i])) {
			t.Errorf("in-window observation %d content changed: got %v want %v", i, contentOf(e), contentOf(in[i]))
		}
	}
}

func TestC530_004_WindowLEZeroReturnsInputUnchangedAndPure(t *testing.T) {
	for _, w := range []int{0, -1} {
		in := []phasestream.Envelope{
			toolUse(1, "read", "keep-1"),
			toolResult(2, "read", "keep-2"),
			toolUse(3, "grep", "keep-3"),
		}
		out := phasestream.MaskStaleObservations(in, w)
		if len(out) != len(in) {
			t.Fatalf("windowTurns=%d changed length: got %d want %d", w, len(out), len(in))
		}
		for i := range out {
			if isMasked(out[i]) {
				t.Errorf("windowTurns=%d must mask nothing, but observation %d is masked", w, i)
			}
			if !reflect.DeepEqual(contentOf(out[i]), contentOf(in[i])) {
				t.Errorf("windowTurns=%d altered observation %d content: got %v want %v", w, i, contentOf(out[i]), contentOf(in[i]))
			}
		}
		if got, _ := contentOf(in[0]).(string); got != "keep-1" {
			t.Errorf("windowTurns=%d mutated the caller's input envelope in place: in[0] content=%q want %q", w, got, "keep-1")
		}
	}
}

func TestC530_005_PolicyDefaultWindowIsTen(t *testing.T) {
	got := policy.Policy{}.ObservationMaskConfig().WindowTurns
	if got != 10 {
		t.Errorf("default ObservationMaskConfig().WindowTurns = %d, want 10", got)
	}
}

func TestC530_006_PolicyWindowReadFromJSONNoEnvFlag(t *testing.T) {
	dir := t.TempDir()

	overridePath := dir + "/policy-override.json"
	if err := os.WriteFile(overridePath, []byte(`{"observation_mask":{"window_turns":5}}`), 0o644); err != nil {
		t.Fatalf("write override policy: %v", err)
	}
	p, err := policy.Load(overridePath)
	if err != nil {
		t.Fatalf("policy.Load(override) error: %v", err)
	}
	if got := p.ObservationMaskConfig().WindowTurns; got != 5 {
		t.Errorf("window_turns=5 in policy.json resolved to %d, want 5 (must be read from the file)", got)
	}

	emptyPath := dir + "/policy-empty.json"
	if err := os.WriteFile(emptyPath, []byte(`{}`), 0o644); err != nil {
		t.Fatalf("write empty policy: %v", err)
	}
	pe, err := policy.Load(emptyPath)
	if err != nil {
		t.Fatalf("policy.Load(empty) error: %v", err)
	}
	if got := pe.ObservationMaskConfig().WindowTurns; got != 10 {
		t.Errorf("absent observation_mask block resolved to %d, want default 10", got)
	}
}

func TestC530_007_PhasestreamAndPolicyVetClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", root+"/go", "./internal/phasestream/", "./internal/policy/")
	if err != nil || code != 0 {
		t.Fatalf("go vet ./internal/phasestream/ ./internal/policy/ reported problems (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

func TestC530_008_PhasestreamRaceTestsGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", root+"/go", "-race", "-count=1", "./internal/phasestream/")
	if err != nil || code != 0 {
		t.Fatalf("go test -race ./internal/phasestream/ failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
