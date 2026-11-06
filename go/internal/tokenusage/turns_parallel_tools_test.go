package tokenusage

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
)

func parallelToolRoot(t *testing.T, lines ...string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "projects", "-repo-worktrees-cycle-2175")
	user := `{"type":"user","cwd":"` + turnWorktree + `","timestamp":"2026-07-07T10:00:01Z","message":{"id":"u1","content":"start"}}` + "\n"
	body := user
	for _, ln := range lines {
		body += ln + "\n"
	}
	writeTranscript(t, dir, "sess-p.jsonl", body)
	return root
}

func TestTurns_ParallelToolCallsOfOneMessageKeepEveryToolNameInOrder(t *testing.T) {
	t.Parallel()
	root := parallelToolRoot(t,
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","id":"t1","name":"Read"}]`, 100, 5, 10, 1),
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","id":"t2","name":"Grep"}]`, 100, 9, 10, 1),
		turnLine("m1", "2026-07-07T10:00:06Z", `[{"type":"tool_use","id":"t3","name":"Bash"}]`, 100, 20, 10, 1),
	)

	res, err := ScanConfigRoot(root, turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Turns) != 1 {
		t.Fatalf("Turns = %+v, want 1 turn for the one message id", res.Turns)
	}
	if want := []string{"Read", "Grep", "Bash"}; !reflect.DeepEqual(res.Turns[0].Tools, want) {
		t.Errorf("Tools = %v, want %v: every tool_use line of the message id contributes its name, not only the last line", res.Turns[0].Tools, want)
	}
	if want := (cyclestate.TokenUsage{Input: 100, Output: 20, CacheRead: 10, CacheWrite: 1}); res.Turns[0].Usage != want || res.Usage != want {
		t.Errorf("turn usage = %+v, row usage = %+v, want both %+v: usage stays the last line of the id", res.Turns[0].Usage, res.Usage, want)
	}
}

func TestTurns_ToolNamesSpreadOverTextAndToolLinesKeepOnlyToolUse(t *testing.T) {
	t.Parallel()
	root := parallelToolRoot(t,
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"thinking","thinking":"plan"}]`, 10, 1, 0, 0),
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","id":"t1","name":"Edit"}]`, 10, 2, 0, 0),
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"text","text":"x"}]`, 10, 3, 0, 0),
		turnLine("m1", "2026-07-07T10:00:06Z", `[{"type":"tool_use","id":"t2","name":"Write"}]`, 10, 4, 0, 0),
	)

	res, err := ScanConfigRoot(root, turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Turns) != 1 || !reflect.DeepEqual(res.Turns[0].Tools, []string{"Edit", "Write"}) {
		t.Errorf("Turns = %+v, want one turn with Tools [Edit Write]", res.Turns)
	}
}

func TestTurns_RepeatedToolUseLineOfOneMessageIsCountedOnce(t *testing.T) {
	t.Parallel()
	root := parallelToolRoot(t,
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","id":"t1","name":"Read"}]`, 10, 1, 0, 0),
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","id":"t1","name":"Read"}]`, 10, 1, 0, 0),
		turnLine("m1", "2026-07-07T10:00:06Z", `[{"type":"tool_use","id":"t2","name":"Read"}]`, 10, 2, 0, 0),
	)

	res, err := ScanConfigRoot(root, turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Turns) != 1 || !reflect.DeepEqual(res.Turns[0].Tools, []string{"Read", "Read"}) {
		t.Errorf("Turns = %+v, want Tools [Read Read]: a re-written line with the same tool_use id counts once, a second call with a new id counts again", res.Turns)
	}
}

func TestTurns_ToolNamesOutsideTheWindowAreNotAttributedToTheTurn(t *testing.T) {
	t.Parallel()
	root := parallelToolRoot(t,
		turnLine("m1", "2026-07-07T09:59:00Z", `[{"type":"tool_use","id":"t0","name":"Glob"}]`, 10, 1, 0, 0),
		turnLine("m1", "2026-07-07T10:00:05Z", `[{"type":"tool_use","id":"t1","name":"Read"}]`, 10, 2, 0, 0),
	)

	res, err := ScanConfigRoot(root, turnWindow(t))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Turns) != 1 || !reflect.DeepEqual(res.Turns[0].Tools, []string{"Read"}) {
		t.Errorf("Turns = %+v, want one turn with Tools [Read]: the pre-window Glob line is not part of the launch", res.Turns)
	}
}
