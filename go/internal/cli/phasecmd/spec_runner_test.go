package phasecmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
)

type artifactBridge struct {
	artifact string
	launched int
}

func (b *artifactBridge) Launch(_ context.Context, req core.BridgeRequest) (core.BridgeResponse, error) {
	b.launched++
	return core.BridgeResponse{}, os.WriteFile(req.ArtifactPath, []byte(b.artifact), 0o644)
}

func (b *artifactBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

func adversarialReviewProject(t *testing.T) string {
	t.Helper()
	root := specPhaseProject(t)
	repo := filepath.Join("..", "..", "..", "..")
	for _, rel := range []string{
		".evolve/phases/adversarial-review/phase.json",
		".evolve/profiles/adversarial-review.json",
		"agents/evolve-adversarial-review.md",
	} {
		raw, err := os.ReadFile(filepath.Join(repo, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		dst := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dst, raw, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func catalogSpec(t *testing.T, root, name string) phasespec.PhaseSpec {
	t.Helper()
	cat, err := LoadCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	spec, ok := cat.Get(name)
	if !ok {
		t.Fatalf("catalog has no %s", name)
	}
	return spec
}

func TestNewSpecRunner_RunsTheSpecPhaseOnTheGivenBridge(t *testing.T) {
	root := adversarialReviewProject(t)
	fb := &artifactBridge{artifact: "## Threat Model\nnone\n## Findings\nnone\n## Verdict\nPASS\n"}

	runner := NewSpecRunner(catalogSpec(t, root, "adversarial-review"), root, fb)

	resp, err := runner.Run(context.Background(), core.PhaseRequest{ProjectRoot: root, Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if fb.launched != 1 {
		t.Errorf("the given bridge launched %d times, want 1", fb.launched)
	}
	if resp.Verdict != core.VerdictPASS {
		t.Errorf("Verdict = %q, want PASS (diags %v)", resp.Verdict, resp.Diagnostics)
	}
}

func TestDefaultBridge_IsTheProductionBridgeAdapter(t *testing.T) {
	if _, ok := DefaultBridge(t.TempDir()).(*bridge.Adapter); !ok {
		t.Errorf("DefaultBridge must build the production bridge adapter")
	}
}

func TestSpecRunnable(t *testing.T) {
	root := adversarialReviewProject(t)
	cases := []struct {
		spec phasespec.PhaseSpec
		want bool
	}{
		{catalogSpec(t, root, "adversarial-review"), true},
		{catalogSpec(t, root, "tdd"), false},
		{phasespec.PhaseSpec{Name: "x-control", Role: string(phasespec.RoleControl)}, false},
		{phasespec.PhaseSpec{Name: "x-native", Kind: "native"}, false},
	}
	for _, c := range cases {
		if got := SpecRunnable(c.spec); got != c.want {
			t.Errorf("SpecRunnable(%s) = %v, want %v", c.spec.Name, got, c.want)
		}
	}
}
