package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseblock"
)

// seedState writes a cycle-state.json with an existing checkpoint block plus
// any extra top-level keys, so we can assert nothing is clobbered.
func seedState(t *testing.T, dir string, extra map[string]any) string {
	t.Helper()
	p := filepath.Join(dir, "cycle-state.json")
	state := map[string]any{
		"cycle_id": 7,
		"phase":    "build",
		"checkpoint": map[string]any{
			"enabled":         true,
			"reason":          string(ReasonPhaseComplete),
			"resumeFromPhase": "build",
		},
	}
	for k, v := range extra {
		state[k] = v
	}
	b, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// readIntegrity parses the file and returns the recorded chain — and FAILS the
// test if the JSON is torn/corrupt (the atomic-write assertion).
func readIntegrity(t *testing.T, path string) []phaseblock.Digest {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Checkpoint struct {
			PhaseIntegrity []phaseblock.Digest `json:"phaseIntegrity"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("torn/invalid cycle-state.json: %v\n%s", err, b)
	}
	return s.Checkpoint.PhaseIntegrity
}

func TestUpsertIntegrity_AppendThenReplaceNoDuplicate(t *testing.T) {
	scout := phaseblock.Digest{Phase: "scout", Combined: "c1"}
	build := phaseblock.Digest{Phase: "build", Combined: "c2"}
	buildRegen := phaseblock.Digest{Phase: "build", Combined: "c2-regenerated"}

	got := upsertIntegrity([]phaseblock.Digest{scout}, build)
	if len(got) != 2 || got[1].Phase != "build" {
		t.Fatalf("append failed: %+v", got)
	}
	got = upsertIntegrity(got, buildRegen)
	if len(got) != 2 {
		t.Fatalf("upsert must not duplicate same phase: %+v", got)
	}
	if got[1].Combined != "c2-regenerated" {
		t.Errorf("upsert must replace (regenerate): %+v", got[1])
	}
}

func TestUpsertIntegrity_DoesNotMutateInput(t *testing.T) {
	in := []phaseblock.Digest{{Phase: "scout", Combined: "c1"}}
	_ = upsertIntegrity(in, phaseblock.Digest{Phase: "scout", Combined: "MUTATED"})
	if in[0].Combined != "c1" {
		t.Errorf("upsertIntegrity mutated its input: %+v", in[0])
	}
}

func TestRecordPhaseIntegrity_PreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	p := seedState(t, dir, map[string]any{"expensiveKey": "must-survive"})

	if err := RecordPhaseIntegrity(p, phaseblock.Digest{Phase: "scout", Combined: "x"}); err != nil {
		t.Fatal(err)
	}

	b, _ := os.ReadFile(p)
	var s map[string]any
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatal(err)
	}
	if s["expensiveKey"] != "must-survive" {
		t.Error("lost a top-level state key")
	}
	cp := s["checkpoint"].(map[string]any)
	if cp["reason"] != string(ReasonPhaseComplete) || cp["resumeFromPhase"] != "build" {
		t.Errorf("lost an existing checkpoint field: %+v", cp)
	}
	if got := readIntegrity(t, p); len(got) != 1 || got[0].Phase != "scout" {
		t.Errorf("integrity not recorded: %+v", got)
	}
}

// THE concurrency test the design hinges on: simulate concurrent pipeline
// phases (and fleet cycles sharing the host-global cycle-state.json) all
// appending their per-phase integrity at once. flock serializes goroutines +
// processes alike (flock.go:8-9), so EVERY entry must survive (no lost
// updates) and the file must never be torn (atomic temp+rename). Run -race.
func TestRecordPhaseIntegrity_ConcurrentPipeline_NoLostUpdates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	p := seedState(t, dir, nil)

	const phases = 48
	var wg sync.WaitGroup
	wg.Add(phases)
	for i := 0; i < phases; i++ {
		go func(i int) {
			defer wg.Done()
			d := phaseblock.Digest{
				Phase:    fmt.Sprintf("phase-%02d", i),
				Combined: fmt.Sprintf("combined-%02d", i),
			}
			if err := RecordPhaseIntegrity(p, d); err != nil {
				t.Errorf("concurrent record %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	got := readIntegrity(t, p) // also asserts JSON is not torn
	if len(got) != phases {
		t.Fatalf("lost updates under concurrency: got %d entries, want %d", len(got), phases)
	}
	seen := make(map[string]bool, phases)
	for _, d := range got {
		seen[d.Phase] = true
	}
	for i := 0; i < phases; i++ {
		if name := fmt.Sprintf("phase-%02d", i); !seen[name] {
			t.Errorf("missing %s — a concurrent update was lost", name)
		}
	}
}

// The phase-complete chokepoint must CARRY a recorded chain, not clobber it
// (the borderline-HIGH review finding): a phase boundary write that used plain
// Compose would drop phaseIntegrity via omitempty.
func TestPhaseBoundaryCheckpointer_CarriesIntegrityChain(t *testing.T) {
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(evolveDir, "cycle-state.json")
	state := map[string]any{
		"cycle_id": 7,
		"phase":    "build",
		"checkpoint": map[string]any{
			"enabled": true,
			"reason":  string(ReasonPhaseComplete),
			"phaseIntegrity": []map[string]any{
				{"phase": "scout", "combined": "c1"},
				{"phase": "triage", "combined": "c2"},
			},
		},
	}
	b, _ := json.MarshalIndent(state, "", "  ")
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}

	if core.PhaseBoundaryCheckpointer == nil {
		t.Fatal("PhaseBoundaryCheckpointer not wired by init()")
	}
	err := core.PhaseBoundaryCheckpointer(
		core.CycleState{Phase: "build", CompletedPhases: []string{"scout", "triage"}},
		root, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}

	got := readIntegrity(t, p)
	if len(got) != 2 || got[0].Phase != "scout" || got[1].Phase != "triage" {
		t.Errorf("chokepoint clobbered the recorded chain: %+v", got)
	}
}

// ComposeWithIntegrity must carry the chain so the phase-complete checkpoint
// write does not clobber recorded integrity.
func TestComposeWithIntegrity_CarriesChain(t *testing.T) {
	chain := []phaseblock.Digest{{Phase: "scout", Combined: "c1"}, {Phase: "build", Combined: "c2"}}
	cp := ComposeWithIntegrity(core.CycleState{Phase: "build"}, ReasonPhaseComplete, 0, "", time.Unix(0, 0).UTC(), chain)
	if len(cp.PhaseIntegrity) != 2 || cp.PhaseIntegrity[1].Phase != "build" {
		t.Errorf("ComposeWithIntegrity dropped the chain: %+v", cp.PhaseIntegrity)
	}
	// Plain Compose must remain integrity-free (back-compat, omitempty).
	plain := Compose(core.CycleState{Phase: "build"}, ReasonPhaseComplete, 0, "", time.Unix(0, 0).UTC())
	if plain.PhaseIntegrity != nil {
		t.Errorf("plain Compose must not set PhaseIntegrity: %+v", plain.PhaseIntegrity)
	}
}
