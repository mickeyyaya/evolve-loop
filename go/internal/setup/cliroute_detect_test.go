package setup

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func phaseStatusOf(t *testing.T, rep DetectReport, role string) PhaseStatus {
	t.Helper()
	for _, ps := range rep.Phases {
		if ps.Role == role {
			return ps
		}
	}
	t.Fatalf("no phase status for %s in %+v", role, rep.Phases)
	return PhaseStatus{}
}

func TestDetect_RendersEachRoleFromTheInjectedTable(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	block := policy.CLIRouting{CLIs: []string{"agy", "claude"}, Default: []string{"agy", "claude"}}
	router, _, err := cliroute.Build(cliroute.Setup{
		Policy: policy.Policy{CLIRouting: &block}, Profiles: profiles.NewFromDir(filepath.Join(evolveDir, "profiles")),
		Host: cliroute.Host{LookPath: func(string) (string, error) { return "/fake", nil }},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	rep := Detect(context.Background(), DetectOptions{ProjectRoot: project, EvolveDir: evolveDir, Env: func(string) string { return "" },
		Doctor: fakeDoctor, CapTier: func(string) string { return "full" }, Router: router})
	if scout := phaseStatusOf(t, rep, "scout"); scout.CurrentCLI != "agy-tmux" || scout.Source != "default" {
		t.Fatalf("detect renders the table's primary and rule: %+v", scout)
	}
}

func TestDetect_ATableItCannotCompileLeavesThePhasesUnresolved(t *testing.T) {
	project, evolveDir := fixtureRepo(t)
	writeFile(t, filepath.Join(evolveDir, "policy.json"), `{"cli_routing":{"clis":["agy"],"default":["agy"]}}`)
	rep := Detect(context.Background(), DetectOptions{ProjectRoot: project, EvolveDir: evolveDir, Env: func(string) string { return "" },
		Doctor: fakeDoctor, CapTier: func(string) string { return "full" }})
	if scout := phaseStatusOf(t, rep, "scout"); scout.Source != "unresolved" || scout.CurrentCLI != "" {
		t.Fatalf("a refused table renders no route: %+v", scout)
	}
}
