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

func TestEngine_ConcurrentLaunch_OnBootRaceFree(t *testing.T) {
	t.Parallel()
	eng := NewEngine(Deps{
		LookupEnv: mapLookup(nil),
		Now:       func() time.Time { return time.Unix(0, 0) },
	})
	ws := t.TempDir()

	const goroutines, iters = 8, 20
	type outcome struct {
		resp core.BridgeResponse
		err  error
	}
	outcomes := make([]outcome, goroutines*iters)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			<-start
			for i := 0; i < iters; i++ {
				agent := fmt.Sprintf("a%d-%d", g, i)
				resp, err := eng.Launch(context.Background(), core.BridgeRequest{
					CLI:          "claude-tmux",
					Profile:      "__nonexistent_profile__",
					Workspace:    ws,
					ArtifactPath: filepath.Join(ws, agent+"-artifact.md"),
					Prompt:       "x",
					Agent:        agent,
				})
				outcomes[g*iters+i] = outcome{resp: resp, err: err}
			}
		}(g)
	}
	close(start)
	wg.Wait()
	for i, o := range outcomes {
		if o.err == nil || o.resp.ExitCode != ExitBadFlags {
			t.Fatalf("launch %d = exit %d, err %v; want the bad-flags refusal of the missing profile", i, o.resp.ExitCode, o.err)
		}
	}
}
