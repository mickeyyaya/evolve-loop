package core

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/recoveryretro"
)

func TestLaneStartRef_ARetrospectiveThatTheBoundaryPublishedIsAProvenBase(t *testing.T) {
	t.Parallel()
	_, runtime := startRefFixture(t)
	pending := recoveryretro.PendingDir(runtime.Dir)
	if err := os.MkdirAll(pending, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"cycle-9-recovery-x.md", "cycle-9-recovery-x.yaml"} {
		if err := os.WriteFile(filepath.Join(pending, name), []byte("lesson\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	published, err := recoveryretro.PublishPending(runtime.Dir, io.Discard)
	if err != nil || len(published) != 1 {
		t.Fatalf("PublishPending = %v, %v; want one published pair", published, err)
	}
	want := runtime.Git("rev-parse", "HEAD")

	ref, err := laneStartRef(context.Background(), runtime.Dir)

	if err != nil || ref != want {
		t.Fatalf("ref=%q err=%v, want the local main HEAD %s: a boundary-published retrospective must not defer every lane (wave 166)", ref, err, want)
	}
}
