package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	gobridge "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func TestPackageHome_IsAScratchDirAndTheDefaultRoutingDoctorReadsUnderIt(t *testing.T) {
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
		if err := os.Remove(auth); err != nil {
			t.Error(err)
		}
	})
	rep, _ := gobridge.NewEngine(gobridge.Deps{}).Doctor(context.Background(), "codex", false)
	if len(rep.Results) != 1 || !rep.Results[0].Auth.Configured {
		t.Fatalf("default doctor probe did not read the scratch home: %+v", rep.Results)
	}
}
