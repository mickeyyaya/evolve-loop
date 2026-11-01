package selfheal_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispositionrouter"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/selfheal"
)

func TestLoadFailureClass_ReadsTheDigestThatCoreWrites(t *testing.T) {
	t.Parallel()
	evolveDir := t.TempDir()
	written := core.FailureDigest{Cycle: 2055, Fingerprint: "build|gate-block|6e9e4730e355", ClassFingerprint: "build|gate-block|class-7098bbb545f3", ClassLayer: dispositionrouter.LayerPipelineCode, ClassRecurrence: 3}
	b, err := json.Marshal(written)
	if err != nil {
		t.Fatal(err)
	}
	path := paths.FailureDigestRecordPath(evolveDir, 2055)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	fc, found, err := selfheal.LoadFailureClass(evolveDir, 2055)

	want := selfheal.FailureClass{Cycle: 2055, Fingerprint: written.ClassFingerprint, Layer: written.ClassLayer, Recurrence: 3}
	if err != nil || !found || fc != want {
		t.Fatalf("LoadFailureClass = %+v %v %v; want %+v, the fields that core writes", fc, found, err, want)
	}
}
