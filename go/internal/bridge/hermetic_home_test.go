package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestPackageHome_IsAScratchDirAndTheDefaultDoctorProbeReadsUnderIt(t *testing.T) {
	if err := gittest.HermeticHomeError(); err != nil {
		t.Fatalf("package HOME: %v", err)
	}
	codexDir := filepath.Join(os.Getenv("HOME"), ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		t.Fatal(err)
	}
	auth := filepath.Join(codexDir, "auth.json")
	if err := os.WriteFile(auth, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Remove(auth); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Error(err)
		}
	})
	rep, _ := newTestEngine(Deps{}).Doctor(context.Background(), "codex", false)
	if len(rep.Results) != 1 || !rep.Results[0].Auth.Configured {
		t.Fatalf("default doctor probe did not read the scratch home: %+v", rep.Results)
	}
}
