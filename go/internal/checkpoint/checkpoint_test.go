package checkpoint

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func fixedTime() time.Time {
	return time.Date(2026, 5, 22, 17, 0, 0, 0, time.UTC)
}

func TestCompose_FromCycleState(t *testing.T) {
	cs := core.CycleState{
		CycleID:         42,
		Phase:           "build",
		ActiveWorktree:  ".evolve/worktrees/abc",
		CompletedPhases: []string{"intent", "scout", "tdd"},
	}
	cp := Compose(cs, ReasonBatchCapNear, 12.34, "deadbeef", fixedTime())
	if !cp.Enabled {
		t.Error("Enabled=false")
	}
	if cp.Reason != ReasonBatchCapNear {
		t.Errorf("Reason=%q, want batch-cap-near", cp.Reason)
	}
	if cp.ResumeFromPhase != "build" {
		t.Errorf("ResumeFromPhase=%q, want build", cp.ResumeFromPhase)
	}
	if cp.WorktreePath != ".evolve/worktrees/abc" {
		t.Errorf("WorktreePath=%q", cp.WorktreePath)
	}
	if len(cp.CompletedPhases) != 3 {
		t.Errorf("CompletedPhases=%v", cp.CompletedPhases)
	}
	if cp.GitHead != "deadbeef" {
		t.Errorf("GitHead=%q", cp.GitHead)
	}
	if cp.CostAtCheckpoint != 12.34 {
		t.Errorf("Cost=%g", cp.CostAtCheckpoint)
	}
	if cp.SavedAt == "" {
		t.Error("SavedAt empty")
	}
	if cp.AutoResumeMaxAttempts != 3 {
		t.Errorf("AutoResumeMaxAttempts=%d, want 3 default", cp.AutoResumeMaxAttempts)
	}
}

func TestCompose_InvalidReason_Panics(t *testing.T) {
	_, err := ComposeChecked(core.CycleState{CycleID: 1, Phase: "x"}, Reason("nope"), 0, "", fixedTime())
	if err == nil {
		t.Error("ComposeChecked(invalid reason): want error")
	}
}

func TestApplyToStateFile_AddsCheckpointBlock(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cycle-state.json")
	original := `{"cycle_id":7,"phase":"build","active_worktree":".evolve/worktrees/a","completed_phases":["intent","scout","tdd"],"workspace_path":".evolve/runs/cycle-7","intent_required":false}`
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := Checkpoint{
		Enabled:               true,
		Reason:                ReasonBatchCapNear,
		SavedAt:               "2026-05-22T17:00:00Z",
		ResumeFromPhase:       "build",
		WorktreePath:          ".evolve/worktrees/a",
		CompletedPhases:       []string{"intent", "scout", "tdd"},
		GitHead:               "abcdef",
		CostAtCheckpoint:      5.0,
		AutoResumeAttempts:    0,
		AutoResumeMaxAttempts: 3,
	}
	if err := ApplyToStateFile(path, cp); err != nil {
		t.Fatalf("ApplyToStateFile: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["phase"] != "build" {
		t.Errorf("phase mutated: %v", got["phase"])
	}
	bl, ok := got["checkpoint"].(map[string]any)
	if !ok {
		t.Fatalf("checkpoint block missing or wrong type: %v", got["checkpoint"])
	}
	if bl["reason"] != "batch-cap-near" {
		t.Errorf("checkpoint.reason=%v", bl["reason"])
	}
	if bl["enabled"] != true {
		t.Errorf("checkpoint.enabled=%v", bl["enabled"])
	}
}

func TestApplyToStateFile_AtomicReplace(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "cs.json")
	if err := os.WriteFile(path, []byte(`{"cycle_id":1,"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cp := Checkpoint{
		Enabled: true, Reason: ReasonQuotaLikely, ResumeFromPhase: "build",
		AutoResumeMaxAttempts: 3,
	}
	if err := ApplyToStateFile(path, cp); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover tmp file: %s", e.Name())
		}
	}
}

func TestApplyToStateFile_MissingFile_Errors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), core.CycleStateFile)
	err := ApplyToStateFile(missing, Checkpoint{Reason: ReasonBatchCapNear})
	if err == nil {
		t.Error("ApplyToStateFile(missing): want error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("err=%v, want os.ErrNotExist", err)
	}
}

func TestCompose_NilCompletedPhases_NormalizedToEmptySlice(t *testing.T) {
	cs := core.CycleState{CycleID: 1, Phase: "build"}
	cp := Compose(cs, ReasonBatchCapNear, 0, "", fixedTime())
	if cp.CompletedPhases == nil {
		t.Error("CompletedPhases=nil, want empty []string{}")
	}
	if len(cp.CompletedPhases) != 0 {
		t.Errorf("CompletedPhases=%v, want empty", cp.CompletedPhases)
	}
}

func TestComposeChecked_ValidReason(t *testing.T) {
	cs := core.CycleState{CycleID: 1, Phase: "build"}
	cp, err := ComposeChecked(cs, ReasonOperatorRequest, 1.0, "abc", fixedTime())
	if err != nil {
		t.Fatalf("ComposeChecked: %v", err)
	}
	if cp.Reason != ReasonOperatorRequest {
		t.Errorf("Reason=%q", cp.Reason)
	}
}

func TestApplyToStateFile_MalformedJSON_Errors(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ApplyToStateFile(path, Checkpoint{Reason: ReasonBatchCapNear, AutoResumeMaxAttempts: 3})
	if err == nil {
		t.Error("ApplyToStateFile(malformed): want error")
	}
}

func TestApplyToStateFile_RenameFailure_CleansTmp(t *testing.T) {
	tmp := t.TempDir()
	dest := filepath.Join(tmp, "cs.json")
	if err := os.WriteFile(dest, []byte(`{"phase":"build"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyToStateFile(dest, Checkpoint{Reason: ReasonBatchCapNear}); err == nil {
		t.Error("ApplyToStateFile(dir as state): want error")
	}
}

func TestApplyWithHooks_ErrorBranches(t *testing.T) {
	base := func() hooks { return defaultHooks() }
	good := func(_ string) ([]byte, error) { return []byte(`{"phase":"build"}`), nil }
	failJSONMarshalChk := func(v any) ([]byte, error) {
		if _, ok := v.(Checkpoint); ok {
			return nil, errors.New("forced marshal err")
		}
		return json.Marshal(v)
	}
	cases := []struct {
		name string
		mod  func(*hooks)
		want string
	}{
		{
			name: "read error",
			mod:  func(h *hooks) { h.readFile = func(string) ([]byte, error) { return nil, errors.New("boom") } },
			want: "read state",
		},
		{
			name: "parse state error",
			mod: func(h *hooks) {
				h.readFile = good
				h.jsonUnmarshal = func([]byte, any) error { return errors.New("bad json") }
			},
			want: "parse state",
		},
		{
			name: "marshal block error",
			mod: func(h *hooks) {
				h.readFile = good
				h.jsonMarshal = failJSONMarshalChk
			},
			want: "marshal block",
		},
		{
			name: "re-parse block error",
			mod: func(h *hooks) {
				h.readFile = good
				calls := 0
				h.jsonUnmarshal = func(b []byte, v any) error {
					calls++
					if calls == 1 {
						return json.Unmarshal(b, v)
					}
					return errors.New("re-parse fail")
				}
			},
			want: "re-parse block",
		},
		{
			name: "marshal state error",
			mod: func(h *hooks) {
				h.readFile = good
				calls := 0
				h.jsonMarshal = func(v any) ([]byte, error) {
					calls++
					if calls == 1 {
						return json.Marshal(v)
					}
					return nil, errors.New("state marshal fail")
				}
			},
			want: "marshal state",
		},
		{
			name: "write tmp error",
			mod: func(h *hooks) {
				h.readFile = good
				h.writeFile = func(string, []byte, os.FileMode) error { return errors.New("disk full") }
			},
			want: "write tmp",
		},
		{
			name: "rename error",
			mod: func(h *hooks) {
				h.readFile = good
				h.writeFile = func(string, []byte, os.FileMode) error { return nil }
				removeCalled := false
				h.rename = func(string, string) error { return errors.New("rename fail") }
				h.remove = func(string) error { removeCalled = true; return nil }
				_ = removeCalled
			},
			want: "rename",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := base()
			c.mod(&h)
			err := applyWithHooks(h, "ignored", Checkpoint{Reason: ReasonBatchCapNear})
			if err == nil {
				t.Fatalf("want error containing %q", c.want)
			}
			if !contains(err.Error(), c.want) {
				t.Errorf("err=%v missing %q", err, c.want)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestReason_Validate(t *testing.T) {
	for _, ok := range []Reason{ReasonQuotaLikely, ReasonBatchCapNear, ReasonOperatorRequest, ReasonStallInactivity} {
		if !ok.IsValid() {
			t.Errorf("%q should be valid", ok)
		}
	}
	if Reason("bogus").IsValid() {
		t.Error("'bogus' should be invalid")
	}
}
