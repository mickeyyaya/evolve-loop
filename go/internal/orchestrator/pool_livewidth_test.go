package orchestrator

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func TestPool_TheOrchestratorPoolPublishesTheLiveWidth(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), lanebudget.LiveWidthFile)
	pool := Pool{Target: 1, Publish: (&eventLog{}).publish, LiveWidthFile: path,
		Launch: func(context.Context, fleet.CycleSpec) (int, error) { return 0, nil }}

	pool.Run(context.Background(), []fleet.Todo{{ID: "only"}})

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the orchestrator pool wrote no live-width file: %v", err)
	}
	var rec struct {
		PID   int `json:"pid"`
		Width int `json:"width"`
	}
	if err := json.Unmarshal(body, &rec); err != nil || rec.PID != os.Getpid() || rec.Width != 1 {
		t.Errorf("live-width file = %s, want this pool's pid and the floor width 1", body)
	}
}
