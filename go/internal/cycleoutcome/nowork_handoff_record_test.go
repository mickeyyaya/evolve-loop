package cycleoutcome

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

func TestApplyNoWork_RecordsTheHandOffOnlyWhenAnItemWentToTheConsole(t *testing.T) {
	t.Parallel()

	root, ws := seedLane(t, []string{"answered"}, []string{"answered"}, `{"top_n":[],"dropped":[{"id":"answered","reason":"stale"}]}`)
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	if !committedset.HandedOff(ws) {
		t.Fatal("a lane that routed its item to the console left no hand-off record")
	}

	root, ws = seedLane(t, nil, []string{"shipped"}, `{"top_n":[],"skip_shipped":[{"task_id":"shipped","git_sha":"abc123"}]}`)
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err != nil {
		t.Fatalf("ApplyNoWork: %v", err)
	}
	if committedset.HandedOff(ws) {
		t.Fatal("a no-work lane that routed nothing was recorded as a hand-off")
	}
}

func TestApplyNoWork_AnUnwritableHandOffRecordIsAnError(t *testing.T) {
	t.Parallel()

	root, ws := seedLane(t, []string{"answered"}, []string{"answered"}, `{"top_n":[],"dropped":[{"id":"answered","reason":"stale"}]}`)
	if err := os.Mkdir(filepath.Join(ws, committedset.HandoffFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyNoWork(NoWorkInputs{ProjectRoot: root, Workspace: ws, Cycle: 7, Stderr: io.Discard}); err == nil {
		t.Fatal("a hand-off the alarm cannot see must fail loudly")
	}
}
