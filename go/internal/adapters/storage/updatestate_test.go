package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// updatestate_test.go — CA.3 (concurrency-factory plan, Track C-A):
// Storage.UpdateState is the serialized lossless read-modify-write for
// state.json. Plain ReadState→mutate→WriteState loses updates when two
// mutators interleave (the 278/279 two-session class) and DROPS unmodeled
// keys (core.State is a subset view). UpdateState fixes both: blocking
// flock around the RMW, revision++ per write, raw-merge preserving
// unmodeled keys.

func TestUpdateState_InProcessConcurrentMutators_ZeroLostUpdates(t *testing.T) {
	s, _ := newStore(t)
	const G, perG = 8, 25
	var wg sync.WaitGroup
	for g := 0; g < G; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				if _, err := s.UpdateState(context.Background(), func(st *core.State) {
					st.LastCycleNumber++
				}); err != nil {
					t.Errorf("UpdateState: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
	st, err := s.ReadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.LastCycleNumber != G*perG {
		t.Errorf("LastCycleNumber = %d, want %d (lost updates)", st.LastCycleNumber, G*perG)
	}
	if st.StateRevision != G*perG {
		t.Errorf("StateRevision = %d, want %d (one bump per write)", st.StateRevision, G*perG)
	}
}

const updStressEnv = "EVOLVE_UPDATESTATE_STRESS_DIR"
const updStressN = 60

// TestHelperStateUpdater is the child-process body for the two-process test.
func TestHelperStateUpdater(t *testing.T) {
	dir := os.Getenv(updStressEnv)
	if dir == "" {
		t.Skip("helper process body; run via TestUpdateState_TwoProcessStress")
	}
	s := New(dir)
	for i := 0; i < updStressN; i++ {
		if _, err := s.UpdateState(context.Background(), func(st *core.State) {
			st.LastCycleNumber++
		}); err != nil {
			fmt.Fprintf(os.Stderr, "child update %d: %v\n", i, err)
			os.Exit(1)
		}
	}
}

// TestUpdateState_TwoProcessStress — the 278/279 class directly: two OS
// processes mutate state.json concurrently; zero lost updates.
func TestUpdateState_TwoProcessStress(t *testing.T) {
	if testing.Short() {
		t.Skip("two-process stress skipped in -short")
	}
	s, evolveDir := newStore(t)

	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperStateUpdater$", "-test.v=false")
	cmd.Env = append(os.Environ(), updStressEnv+"="+evolveDir)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	for i := 0; i < updStressN; i++ {
		if _, err := s.UpdateState(context.Background(), func(st *core.State) {
			st.LastCycleNumber++
		}); err != nil {
			t.Fatalf("parent update %d: %v", i, err)
		}
	}
	if err := cmd.Wait(); err != nil {
		t.Fatalf("child process failed: %v", err)
	}
	st, err := s.ReadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.LastCycleNumber != 2*updStressN {
		t.Errorf("LastCycleNumber = %d, want %d (cross-process lost updates)", st.LastCycleNumber, 2*updStressN)
	}
}

// TestUpdateState_PreservesUnmodeledKeys — core.State is a SUBSET view;
// UpdateState must never drop operator keys like expected_ship_sha (the
// WriteState hazard the setup marker's raw-merge precedent exists for).
func TestUpdateState_PreservesUnmodeledKeys(t *testing.T) {
	s, evolveDir := newStore(t)
	seed := `{"lastCycleNumber":5,"expected_ship_sha":"abc123","customOperatorKey":{"nested":true}}`
	if err := os.WriteFile(filepath.Join(evolveDir, "state.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateState(context.Background(), func(st *core.State) {
		st.LastCycleNumber++
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if string(obj["expected_ship_sha"]) != `"abc123"` {
		t.Errorf("expected_ship_sha dropped/changed: %s", obj["expected_ship_sha"])
	}
	if _, ok := obj["customOperatorKey"]; !ok {
		t.Error("customOperatorKey dropped")
	}
	var st core.State
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.LastCycleNumber != 6 {
		t.Errorf("mutation lost: LastCycleNumber = %d, want 6", st.LastCycleNumber)
	}
}

// TestUpdateState_ClearedModeledFieldStaysCleared — the raw-merge must not
// resurrect a modeled key the mutation emptied (omitempty drops it from the
// typed marshal; the old raw value must not survive).
func TestUpdateState_ClearedModeledFieldStaysCleared(t *testing.T) {
	s, evolveDir := newStore(t)
	if err := s.WriteState(context.Background(), core.State{
		LastCycleNumber: 1,
		CarryoverTodos:  []core.CarryoverTodo{{ID: "todo-1", Action: "x"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateState(context.Background(), func(st *core.State) {
		st.CarryoverTodos = nil
	}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(evolveDir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["carryoverTodos"]; ok {
		t.Errorf("carryoverTodos resurrected from raw map: %s", obj["carryoverTodos"])
	}
}

// TestUpdateState_RevisionOmittedAtZero — byte-stability (CA.6 additive-
// fields-only): states that never went through UpdateState carry no
// stateRevision key.
func TestUpdateState_RevisionOmittedAtZero(t *testing.T) {
	data, err := json.Marshal(core.State{LastCycleNumber: 3})
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	if _, ok := obj["stateRevision"]; ok {
		t.Errorf("zero StateRevision must be omitted: %s", data)
	}
}
