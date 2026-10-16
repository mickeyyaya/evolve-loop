package storage

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func coverfixNoopMutate(*core.State) {}

func TestCoverfixUpdateStateLockFailureIsReported(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := New(filepath.Join(blocker, ".evolve")).UpdateState(context.Background(), coverfixNoopMutate)
	if err == nil || !strings.Contains(err.Error(), "update state:") {
		t.Fatalf("want wrapped lock error, got %v", err)
	}
}

func TestCoverfixUpdateStateRefusesUnreadableOrCorruptState(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, path string)
		want string
	}{
		{"malformed", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "malformed"},
		{"typeMismatch", func(t *testing.T, p string) {
			if err := os.WriteFile(p, []byte(`{"lastCycleNumber":"x"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}, "decode"},
		{"unreadable", func(t *testing.T, p string) {
			if err := os.Mkdir(p, 0o755); err != nil {
				t.Fatal(err)
			}
		}, "read"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, dir := newStore(t)
			path := filepath.Join(dir, "state.json")
			tc.seed(t, path)
			called := false
			_, err := s.UpdateState(context.Background(), func(*core.State) { called = true })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want %q error, got %v", tc.want, err)
			}
			if called {
				t.Error("mutate must not run when the on-disk state cannot be trusted")
			}
		})
	}
}

func TestCoverfixUpdateStateMarshalFailuresDoNotWrite(t *testing.T) {
	s, dir := newStore(t)
	path := filepath.Join(dir, "state.json")
	_, err := s.UpdateState(context.Background(), func(st *core.State) { st.CurrentBatch.CycleAccruedCostUSD = math.NaN() })
	if err == nil || !strings.Contains(err.Error(), "marshal") {
		t.Fatalf("want marshal error, got %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Errorf("failed update must not create state.json")
	}

	saved := marshalState
	t.Cleanup(func() { marshalState = saved })
	marshalState = func(any) ([]byte, error) { return []byte("[1]"), nil }
	_, err = s.UpdateState(context.Background(), coverfixNoopMutate)
	if err == nil || !strings.Contains(err.Error(), "remap") {
		t.Fatalf("want remap error, got %v", err)
	}
}

func TestCoverfixUpdateStateWriteFailureIsReported(t *testing.T) {
	s, _ := newStore(t)
	boom := errors.New("boom")
	var err error
	withHooks(ioHooks{marshal: func(any) ([]byte, error) { return nil, boom }}, func() {
		_, err = s.UpdateState(context.Background(), coverfixNoopMutate)
	})
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "update state: write") {
		t.Fatalf("want wrapped write error, got %v", err)
	}
}
