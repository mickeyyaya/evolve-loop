package ship

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// TestWithStateLock_SerializesWithUpdateState pins ADR-0049 S2 / gap G2: ship's
// map-based state.json read-modify-write must hold the SAME advisory lock
// storage.UpdateState holds (<path>.lock), so the two whole-file writers cannot
// clobber each other. Half the goroutines bump an UNMODELED key via
// withStateLock; the other half bump a MODELED key via UpdateState — all on one
// state.json. Without a shared lock the interleaved whole-file writes lose
// updates on BOTH counters (RED); with the shared flock every write serializes
// and both counters reach the full total (GREEN). Run with -race.
func TestWithStateLock_SerializesWithUpdateState(t *testing.T) {
	dir := t.TempDir()
	stPath := filepath.Join(dir, "state.json")
	st := storage.New(dir) // UpdateState locks <dir>/state.json.lock — same file withStateLock must lock
	const G, perG = 8, 25

	var wg sync.WaitGroup
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if err := withStateLock(stPath, func() error {
					m, err := readStateMap(stPath)
					if err != nil {
						return err
					}
					n, _ := m["ship_counter"].(float64)
					m["ship_counter"] = n + 1
					return writeStateMap(stPath, m)
				}); err != nil {
					t.Errorf("withStateLock: %v", err)
					return
				}
			}
		}()
	}
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if _, err := st.UpdateState(context.Background(), func(s *core.State) { s.LastCycleNumber++ }); err != nil {
					t.Errorf("UpdateState: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()

	m, err := readStateMap(stPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m["ship_counter"].(float64); int(got) != G*perG {
		t.Errorf("ship_counter=%v, want %d — withStateLock lost updates (not sharing UpdateState's lock)", got, G*perG)
	}
	final, err := st.ReadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if final.LastCycleNumber != G*perG {
		t.Errorf("LastCycleNumber=%d, want %d — typed updates lost to an unlocked ship write", final.LastCycleNumber, G*perG)
	}
}
