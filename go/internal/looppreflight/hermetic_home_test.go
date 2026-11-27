package looppreflight

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
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
	rep, _ := bridge.NewEngine(bridge.Deps{}).Doctor(context.Background(), "codex", false)
	if len(rep.Results) != 1 || !rep.Results[0].Auth.Configured {
		t.Fatalf("default doctor probe did not read the scratch home: %+v", rep.Results)
	}
}

func TestVersionCellBinRoot_StagesUnderAScratchDirInTests(t *testing.T) {
	if versionCellBinRoot == nil {
		t.Fatal("the package test entry did not set versionCellBinRoot; CLIVersion would stage into the real ~/.evolve/cell-bin")
	}
	root, err := versionCellBinRoot()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(root, os.TempDir()) {
		t.Fatalf("cell-bin root = %q, want a dir under %q", root, os.TempDir())
	}
}
