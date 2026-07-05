//go:build acs

// Package cycle530 materialises the cycle-530 acceptance criteria for the
// SINGLE triage-committed task (this lane's assigned fleet_scope id):
//
//	observation-masking-stale-tool-eviction — a first, deterministic slice of
//	config-driven observation masking. In the phasestream layer (the one Go
//	abstraction that models a per-turn envelope sequence for headless drivers),
//	add a PURE transform that replaces the bulky content of tool-observation
//	envelopes (KindToolUse / KindToolResult) older than a rolling window with a
//	compact placeholder, preserving the reasoning/action chain and NEVER
//	touching the never-evict classes (verdict = KindResult, error = KindError,
//	etc.). Wire the window through policy.json (default 10), no new env flag.
//	→ C530_001..008
//
// TASK BINDING (cycle-522/523 lesson): cycle 530's triage-report `## top_n`
// restricts this lane to `observation-masking-stale-tool-eviction` ONLY (an
// orchestrator fleet_scope override). scout-report.md's own Task 1-3 (cycle-525
// recovery, fleet-width projection, treediff dossier false-leak) are all
// triage-report `## deferred` to sibling lanes / future cycles and get ZERO
// predicates here. Predicates bind only to triage-committed work (R9.3).
//
// SCOPE (triage `medium`, single transport): this cycle lands the deterministic,
// unit-testable transform + policy plumbing ONLY. The live wiring into a running
// single-shot `claude -p` / `codex exec` subprocess (whose internal context
// evolve-loop cannot mutate mid-run) and the tmux-REPL re-injection recipe are
// explicit follow-up work — see fault-localization-report.md Architecture
// findings #3-5. inbox AC3 (soak-batch token-telemetry / no solve-rate
// regression) is therefore NOT a predicate this cycle; it is materialised as a
// manual+checklist item addressed to the Auditor in test-report.md.
//
// 1:1 AC-materialization (see .evolve/evals/observation-masking-stale-tool-eviction.md):
// 4 predicate ACs + 1 manual+checklist AC + 0 removed = 5 inbox ACs total, none
// double-counted (an AC may drive >1 predicate but carries exactly one
// disposition).
//
// Why these predicates exercise the SUT directly (cycle-85 predicate-quality):
// every load-bearing predicate CALLS phasestream.MaskStaleObservations (or
// policy.ObservationMaskConfig) in-process on a crafted envelope slice / policy
// and asserts on the returned values — never a "source file contains text X"
// check. Neither SUT symbol exists on `main` yet, so the package is RED by
// COMPILE FAILURE until Builder adds them (the intended RED form for a
// new-capability task; see test-report.md "RED Run Output").
//
// The masking contract these predicates pin (TDD-defined, Builder implements
// exactly this — surfaced as a design decision in test-report.md):
//   - Evictable kinds = {KindToolUse, KindToolResult}. Each such envelope is one
//     observation, ordered by slice position (already Seq-ordered upstream).
//   - The newest `windowTurns` evictable observations are RETAINED unmasked;
//     every OLDER evictable observation is MASKED: its Data gets marker key
//     "masked"=true and its bulky content field ("input_excerpt" for tool_use,
//     "excerpt" for tool_result) is replaced by a compact placeholder string.
//     Identity keys ("name"/"id"/"tool_use_id") stay so the action chain reads.
//   - windowTurns <= 0 ⇒ input returned unchanged (feature off / byte-identical).
//   - Never-evict kinds (KindResult, KindError, KindStall, KindInteraction,
//     KindCorrelation, ...) are ALWAYS returned unchanged regardless of age.
//   - Pure: the input slice's envelopes are never mutated in place (immutability
//     rule); masked entries are copies.
//
// Adversarial diversity (skills/adversarial-testing SKILL §6):
//
//	Negative:   C530_002 — old verdict/error envelopes must NEVER be masked; a
//	            fake that masks everything-old fails this. C530_004 — windowTurns<=0
//	            must NOT fabricate masking AND must not mutate the input.
//	Edge / OOD: C530_003 — a session entirely INSIDE the window masks nothing;
//	            C530_004 — window 0 and negative; C530_006 — absent policy block.
//	Semantic:   C530_001 (past-window → masked) vs C530_003 (within-window →
//	            untouched) are DISTINCT behaviors driven only by the window — a
//	            fake with a constant answer passes one and fails the other.
package cycle530

import (
	"os"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// toolUse builds a KindToolUse observation envelope shaped like classify.go's
// default branch (name/id/input_excerpt).
func toolUse(seq int64, name, excerpt string) phasestream.Envelope {
	return phasestream.Envelope{
		SchemaVersion: phasestream.SchemaVersion,
		Seq:           seq,
		Kind:          phasestream.KindToolUse,
		Severity:      phasestream.SeverityInfo,
		Data:          map[string]any{"name": name, "id": name, "input_excerpt": excerpt},
	}
}

// toolResult builds a KindToolResult observation envelope shaped like
// classify.go (tool_use_id/is_error/excerpt).
func toolResult(seq int64, id, excerpt string) phasestream.Envelope {
	return phasestream.Envelope{
		SchemaVersion: phasestream.SchemaVersion,
		Seq:           seq,
		Kind:          phasestream.KindToolResult,
		Severity:      phasestream.SeverityInfo,
		Data:          map[string]any{"tool_use_id": id, "is_error": false, "excerpt": excerpt},
	}
}

// isMasked reports whether an envelope carries the mask marker.
func isMasked(e phasestream.Envelope) bool {
	v, ok := e.Data["masked"]
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// contentOf returns the bulky observation payload field for an evictable kind.
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

// TestC530_001_MasksToolObservationsOlderThanWindow (inbox AC1 + AC4, positive —
// THE feature). A session of 12 evictable observations (tool_use/tool_result,
// Seq 1..12) with windowTurns=4 must mask exactly the 8 oldest (Seq 1..8) and
// leave the newest 4 (Seq 9..12) untouched. Masked ⇒ Data["masked"]==true and
// the original bulky content is gone; unmasked ⇒ original content intact. RED
// today by compile failure (MaskStaleObservations undefined). Gaming fake
// killed: an identity no-op leaves everything unmasked (masked count 0 ≠ 8).
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
		case e.Seq <= 8: // older than the newest-4 window ⇒ masked
			if !isMasked(e) {
				t.Errorf("Seq %d (older than window) must be masked but was not; data=%v", e.Seq, e.Data)
			}
			if c, _ := contentOf(e).(string); c == "ORIGINAL-USE-content-seq" || c == "ORIGINAL-RESULT-content-seq" {
				t.Errorf("Seq %d masked envelope still carries original content %q", e.Seq, c)
			}
		case e.Seq >= 9: // inside the newest-4 window ⇒ preserved
			if isMasked(e) {
				t.Errorf("Seq %d (inside window) must NOT be masked; data=%v", e.Seq, e.Data)
			}
			if c, _ := contentOf(e).(string); c != "ORIGINAL-USE-content-seq" && c != "ORIGINAL-RESULT-content-seq" {
				t.Errorf("Seq %d in-window content was altered: %q", e.Seq, c)
			}
		}
	}
}

// TestC530_002_NeverEvictsVerdictAndErrorEvenWhenOld (inbox AC4 never-evict,
// negative / anti-gaming — the strongest predicate). An OLD verdict (KindResult)
// and an OLD error (KindError) sit at the head of the session (oldest), followed
// by many recent tool observations; windowTurns=2. The verdict and error must be
// returned byte-identical (no "masked" marker, Data untouched) despite being the
// oldest envelopes. Kills the fake that masks everything past the window: such a
// fake would mask the verdict/error and fail here.
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

// TestC530_003_KeepsObservationsInsideWindow (inbox AC1, semantic / distinct
// behavior). A short session (3 evictable observations) with a window >= the
// session length masks NOTHING. Distinguishes the real transform from an
// always-mask fake, and pins that in-window content is preserved verbatim.
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

// TestC530_004_WindowLEZeroReturnsInputUnchangedAndPure (inbox AC4 byte-identical
// passthrough + immutability, negative / edge). windowTurns<=0 is the feature-off
// state: the output must be content-equal to the input (nothing masked), and the
// call must be PURE — the caller's input envelopes are not mutated in place. Both
// windowTurns=0 and windowTurns=-1 are checked. Mirrors this repo's established
// "count<2 byte-identical" regression guarantee for new config knobs.
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
		// Purity: the ORIGINAL input envelope content must survive the call.
		if got, _ := contentOf(in[0]).(string); got != "keep-1" {
			t.Errorf("windowTurns=%d mutated the caller's input envelope in place: in[0] content=%q want %q", w, got, "keep-1")
		}
	}
}

// TestC530_005_PolicyDefaultWindowIsTen (inbox AC2, default). An absent
// observation_mask block resolves to WindowTurns=10 (the paper's optimum). Calls
// the resolved getter directly on a zero Policy — no source scan.
func TestC530_005_PolicyDefaultWindowIsTen(t *testing.T) {
	got := policy.Policy{}.ObservationMaskConfig().WindowTurns
	if got != 10 {
		t.Errorf("default ObservationMaskConfig().WindowTurns = %d, want 10", got)
	}
}

// TestC530_006_PolicyWindowReadFromJSONNoEnvFlag (inbox AC2, read-from-policy.json
// / no env flag). Writing observation_mask.window_turns into a policy.json and
// loading it through the real policy.Load pipeline yields that window — proving
// the knob is sourced from the file, not a new env dial. An empty policy.json
// falls back to the default 10. Exercises Load + the getter end to end.
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

// TestC530_007_PhasestreamAndPolicyVetClean (inbox AC5, repo-gate pin). The two
// packages this task touches must stay `go vet`-clean through the change.
// Subprocess against the real toolchain — not a text scan.
func TestC530_007_PhasestreamAndPolicyVetClean(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "vet", "-C", root+"/go", "./internal/phasestream/", "./internal/policy/")
	if err != nil || code != 0 {
		t.Fatalf("go vet ./internal/phasestream/ ./internal/policy/ reported problems (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}

// TestC530_008_PhasestreamRaceTestsGreen (inbox AC5, repo-gate pin). The
// phasestream package — including the new mask_test.go Builder must add — passes
// under the race detector. Exercises the real toolchain on the SUT package.
func TestC530_008_PhasestreamRaceTestsGreen(t *testing.T) {
	root := acsassert.RepoRoot(t)
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-C", root+"/go", "-race", "-count=1", "./internal/phasestream/")
	if err != nil || code != 0 {
		t.Fatalf("go test -race ./internal/phasestream/ failed (code=%d err=%v)\nstdout:\n%s\nstderr:\n%s", code, err, stdout, stderr)
	}
}
