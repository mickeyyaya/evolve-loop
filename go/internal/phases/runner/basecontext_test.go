package runner

// RED-phase contract for cycle-249 task `runner-base-cycle-context`:
// BaseCycleContext(body, req) is the single source for the "## Cycle
// Context" core block that 10 phase files currently copy-paste. The
// helper must emit the four mandatory fields BYTE-IDENTICALLY to the
// duplicated block so callers can swap to it with zero prompt drift.
//
// These tests fail at baseline because BaseCycleContext does not exist
// yet (compile error: undefined) — that is the correct RED signal.

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestBaseCycleContext_CoreBlockByteIdentical(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       249,
		GoalHash:    "8274f532",
		ProjectRoot: "/proj/root",
		Workspace:   "/ws/dir",
	}
	got := BaseCycleContext("AGENT BODY", req)
	// Byte-for-byte parity with the duplicated block:
	//   b.WriteString(body)
	//   b.WriteString("\n\n## Cycle Context\n")
	//   fmt.Fprintf(&b, "- cycle: %d\n", req.Cycle)
	//   fmt.Fprintf(&b, "- goal_hash: %s\n", req.GoalHash)
	//   fmt.Fprintf(&b, "- project_root: %s\n", req.ProjectRoot)
	//   fmt.Fprintf(&b, "- workspace: %s\n", req.Workspace)
	want := "AGENT BODY\n\n## Cycle Context\n" +
		"- cycle: 249\n" +
		"- goal_hash: 8274f532\n" +
		"- project_root: /proj/root\n" +
		"- workspace: /ws/dir\n"
	if got != want {
		t.Errorf("BaseCycleContext output drifted from the duplicated block\n got: %q\nwant: %q", got, want)
	}
}

// Negative: the helper owns ONLY the four mandatory fields. Phase-specific
// extras (worktree, goal text, mode, carryover_summary) remain the caller's
// responsibility — emitting them here would change every phase's prompt.
func TestBaseCycleContext_OmitsPhaseSpecificExtras(t *testing.T) {
	req := core.PhaseRequest{
		Cycle:       7,
		GoalHash:    "h",
		ProjectRoot: "/p",
		Workspace:   "/w",
		Worktree:    "/wt/cycle-7",
		Context:     map[string]string{"goal": "secret goal text", "carryover_summary": "stuff"},
	}
	got := BaseCycleContext("BODY", req)
	for _, forbidden := range []string{"worktree", "goal:", "mode:", "carryover_summary"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("core block must not emit phase-specific extra %q; got:\n%s", forbidden, got)
		}
	}
}

// Edge: empty body still yields a well-formed block (callers like
// specrunner may compose from inline bodies that can be empty).
func TestBaseCycleContext_EmptyBody(t *testing.T) {
	got := BaseCycleContext("", core.PhaseRequest{Cycle: 1, GoalHash: "g", ProjectRoot: "/r", Workspace: "/s"})
	if !strings.HasPrefix(got, "\n\n## Cycle Context\n") {
		t.Errorf("empty body must still open the block with \\n\\n## Cycle Context\\n; got: %q", got)
	}
}

// Edge: zero values are emitted, not skipped — parity with the current
// duplicated block, which prints all four lines unconditionally.
func TestBaseCycleContext_ZeroValuesStillEmitAllFourKeys(t *testing.T) {
	got := BaseCycleContext("B", core.PhaseRequest{})
	for _, key := range []string{"- cycle: 0\n", "- goal_hash: \n", "- project_root: \n", "- workspace: \n"} {
		if !strings.Contains(got, key) {
			t.Errorf("zero-value request must still emit %q (unconditional parity); got: %q", key, got)
		}
	}
}
