package bridge

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/retryladder"
)

func TestTryRecord_AnR1TryStartsANewSessionAndRecordsTheDecision(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	prof := writeProfile(t, ws, "try-record", "")
	artifact := filepath.Join(ws, "artifact.md")
	tmux := &artifactOnPasteTmux{FakeTmuxController: &FakeTmuxController{CaptureFrames: slices.Repeat([]string{"working ❯"}, 40)}, artifact: artifact}
	eng := NewEngine(Deps{Tmux: tmux, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)})
	base := core.BridgeRequest{
		CLI: "claude-tmux", Profile: prof, Model: "balanced", Prompt: "work",
		Workspace: ws, Worktree: ws, ProjectRoot: ws, ArtifactPath: artifact, Agent: "builder", Attempt: 1,
	}
	raised := base
	raised.Attempt, raised.Prompt = 2, "work\n\n## Retry digest (rung R1, class capability)"
	raised.Ladder = &retryladder.Try{
		Rung: retryladder.R1, FailureClass: retryladder.Capability, Fingerprint: "codes:cover_changed",
		Context: retryladder.ContextFresh, Stage: retryladder.StageEnforce, Would: retryladder.R1,
	}
	for _, req := range []core.BridgeRequest{base, raised} {
		if _, err := eng.Launch(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}

	var sessions []string
	for _, ev := range tmux.Events {
		if name, ok := strings.CutPrefix(ev, "new-session:"); ok && !slices.Contains(sessions, name) {
			sessions = append(sessions, name)
		}
	}
	if len(sessions) != 2 {
		t.Fatalf("two tries opened sessions %v, want a new session for the raised try", sessions)
	}
	recs := readTryRecords(t, ws)
	if len(recs) != 2 {
		t.Fatalf("records = %d, want 2", len(recs))
	}
	if recs[0].Rung != "R0" || recs[0].Context != "new" {
		t.Fatalf("the base try = %+v, want R0 new", recs[0])
	}
	got := recs[1]
	if got.Rung != "R1" || got.Context != "fresh" || got.LadderStage != "enforce" || got.LadderWould != "R1" ||
		got.FailureClass != "capability" || got.Fingerprint != "codes:cover_changed" {
		t.Fatalf("the raised try = %+v, want the decision of the ladder", got)
	}
}
