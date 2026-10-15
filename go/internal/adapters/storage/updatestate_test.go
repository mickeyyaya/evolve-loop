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

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/carryover"
)

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

var (
	_ carryover.Store   = (*FilesystemStorage)(nil)
	_ carryover.Updater = (*FilesystemStorage)(nil)
)
