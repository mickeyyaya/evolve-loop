package intent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
)

func TestRegistryFactory_ReadsDeltaModeFromThePolicy(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	factory, ok := registry.For(string(core.PhaseIntent))
	if !ok {
		t.Fatal("intent is not registered")
	}
	req := core.PhaseRequest{ProjectRoot: root}
	if got := factory(req).(*Phase).ComposePrompt("B", req); !strings.Contains(got, "- mode: full") {
		t.Fatalf("prompt = %q, want full mode without a policy key", got)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"workflow":{"intent_delta":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := factory(req).(*Phase).ComposePrompt("B", req); !strings.Contains(got, "- mode: delta") {
		t.Fatalf("prompt = %q, want delta mode from workflow.intent_delta", got)
	}
}
