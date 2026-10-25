package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

func TestCommentFloorStage_ARefusedPolicyFailsClosedToEnforce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"comment_floor": {"stage": "shadow"}, "fleet": {"cuont": 3}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var warn strings.Builder

	stage := commentFloorStage(root, &warn)

	if stage != config.StageEnforce || !strings.Contains(warn.String(), `"fleet.cuont"`) {
		t.Fatalf("stage %q warn %q: a refused policy runs the comment floor at enforce, not the shadow stage it asked for", stage, warn.String())
	}
}
