package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestEngine_ConcurrentLaunch_OnBootRaceFree pins the fresh-engine-per-Launch
// contract as STRUCTURAL, not merely comment-enforced: concurrent Launch calls
// on ONE Engine must not race. Launch installs a call-local OnBoot hook to
// capture BootMS; the pre-fix implementation mutated the SHARED e.deps.OnBoot
// field (restored via defer), so two concurrent Launches raced on that write.
// `go test -race` catches it. The fix threads OnBoot through a per-call Deps
// copy (mirroring the per-call Stdout/Stderr already threaded in LaunchArgs),
// making the field write call-local and race-free by construction.
//
// The requests fast-fail inside LaunchArgs (a nonexistent profile => ExitBadFlags)
// AFTER Launch has installed its OnBoot hook, so the test exercises the shared
// mutation deterministically without spawning any real driver.
func TestEngine_ConcurrentLaunch_OnBootRaceFree(t *testing.T) {
	eng := NewEngine(Deps{
		LookupEnv: mapLookup(nil),
		Now:       func() time.Time { return time.Unix(0, 0) },
	})
	ws := t.TempDir()

	const goroutines, iters = 8, 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start // release all goroutines together to maximize overlap
			for i := 0; i < iters; i++ {
				agent := fmt.Sprintf("a%d-%d", g, i)
				_, _ = eng.Launch(context.Background(), core.BridgeRequest{
					CLI:          "claude-tmux",
					Profile:      "__nonexistent_profile__",
					Workspace:    ws,
					ArtifactPath: filepath.Join(ws, agent+"-artifact.md"),
					Prompt:       "x",
					Agent:        agent, // unique => no prompt/log/artifact file collisions
				})
			}
		}(g)
	}
	close(start)
	wg.Wait()
}
