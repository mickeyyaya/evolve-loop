package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
)

func TestLoopPreflight_AnUnknownPolicyKeyRefusesTheLoopStart(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evolveDir := filepath.Join(root, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evolveDir, "policy.json"), []byte(`{"fleet":{"cuont":4}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	res := defaultLoopPreflight(loopConfig{ProjectRoot: root, EvolveDir: evolveDir, SkipPreflightBoot: true}, io.Discard)

	if res.OverallLevel != looppreflight.LevelHalt {
		t.Fatalf("a policy with an unknown key halts the readiness gate, got %s", res.OverallLevel)
	}
	for _, ch := range res.Checks {
		if ch.Level == looppreflight.LevelHalt && strings.Contains(ch.Message, `"fleet.cuont"`) {
			return
		}
	}
	t.Fatalf("no halting check names the unknown key: %+v", res.Checks)
}
