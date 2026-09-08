//go:build integration

package swarmrunner

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/swarm"
)

func TestDecorator_WriterLiveModesRefusedBeforeDispatch(t *testing.T) {
	for _, stage := range []string{"advisory", "enforce"} {
		t.Run(stage, func(t *testing.T) {
			root := gitInitForTest(t)
			ws := t.TempDir()
			writePlan(t, ws, swarm.ModeWriter)
			inner, bridge := &fakeInner{name: "build"}, &fakeBridge{}
			d := New(inner, bridge, swarm.ModeWriter, Config{Stage: stage, WorktreeBase: t.TempDir()})
			resp, err := d.Run(context.Background(), reqWithRoot(root, ws, nil))
			if err == nil || !strings.Contains(err.Error(), "writer swarm is unsupported") || resp.Verdict != core.VerdictFAIL {
				t.Fatalf("unsupported writer path must fail explicitly: response=%+v error=%v", resp, err)
			}
			if atomic.LoadInt32(&inner.ran) != 0 || atomic.LoadInt32(&bridge.launches) != 0 {
				t.Fatal("unsupported writer mode must refuse before inner or worker side effects")
			}
		})
	}
}
