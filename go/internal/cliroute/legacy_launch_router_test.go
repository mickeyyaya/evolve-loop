package cliroute_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestNewLegacyLaunchRouter_RefusesADeclaredProjectAndRoutesALegacyOne(t *testing.T) {
	sp := cliroute.SingleProfile{Agent: "retrospective", Profile: &profiles.Profile{Name: "retrospective", CLI: "codex-tmux"}}
	declared := t.TempDir()
	if err := os.MkdirAll(filepath.Join(declared, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(declared, ".evolve", "policy.json"), []byte(`{"cli_routing":{"clis":["claude"],"default":["claude"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := cliroute.NewLegacyLaunchRouter(declared, sp); !errors.Is(err, cliroute.ErrNoRootRouter) {
		t.Fatalf("err = %v, want the missing root router named", err)
	}

	r, err := cliroute.NewLegacyLaunchRouter(t.TempDir(), sp)
	if err != nil {
		t.Fatal(err)
	}
	d, err := r.Resolve(cliroute.Request{Agent: "retrospective", DefaultModel: "deep"})
	if err != nil || d.Plan.Candidates[0] != "codex-tmux" || !d.Legacy() {
		t.Fatalf("a project with no table routes the profile as before: %+v %v", d, err)
	}
}

func TestNewLegacyLaunchRouter_AMalformedPolicyIsAnErrorNotALegacyRoute(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_routing": `), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := cliroute.NewLegacyLaunchRouter(root, cliroute.SingleProfile{Agent: "retrospective"})

	if err == nil || r != nil || errors.Is(err, cliroute.ErrNoRootRouter) {
		t.Fatalf("router=%v err=%v: an unreadable policy fails loudly, never routes as legacy", r, err)
	}
}
