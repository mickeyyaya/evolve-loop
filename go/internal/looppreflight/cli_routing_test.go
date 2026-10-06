package looppreflight

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/doctor"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func TestDistinctDrivers_FollowsTheResolvedChains(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}}, nil
	}
	opts.Routing = func() (Routing, error) { return Routing{Drivers: []string{"agy-tmux", "claude-tmux"}}, nil }
	var probed []string
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		probed = append(probed, bin)
		return doctor.Result{Tool: bin, Found: true}, nil
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(probed, "codex") || !slices.Contains(probed, "agy") || !slices.Contains(probed, "claude") {
		t.Fatalf("the preflight probes the families the resolved chains launch, never a profile's codex: %v", probed)
	}
	if _, inventoried := r.CLIVersions["codex"]; inventoried {
		t.Fatalf("the version inventory follows the resolved chains too: %v", r.CLIVersions)
	}
}

func TestPreflight_CliRoutingHaltsOnAFinding(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.Routing = func() (Routing, error) {
		return Routing{Findings: []cliroute.Finding{{Severity: cliroute.SeverityError, Key: "cli_routing.clis", Message: "claude must be listed"}}},
			errors.New("cliroute: the routing table has 1 error finding(s)")
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	c := findCheck(t, r, "cli-routing")
	if c.Level != LevelHalt || !r.Halted() || !strings.Contains(c.Detail, "cli_routing.clis") {
		t.Fatalf("a table that does not compile halts the batch naming the finding: %+v", c)
	}
}

func TestPreflight_CliRoutingWarnsOnAWarning(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.Routing = func() (Routing, error) {
		return Routing{Drivers: []string{"claude-tmux"}, Findings: []cliroute.Finding{{Severity: cliroute.SeverityWarn, Key: "cross_family_with.auditor+builder", Message: "shared fallback"}}}, nil
	}
	r, _ := Run(opts)
	if c := findCheck(t, r, "cli-routing"); c.Level != LevelWarn || !strings.Contains(c.Detail, "cross_family_with.auditor+builder") {
		t.Fatalf("a warning finding warns: %+v", c)
	}
}

func TestPreflight_CliRoutingPassesACleanTable(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.Routing = func() (Routing, error) { return Routing{Drivers: []string{"claude-tmux"}}, nil }
	r, _ := Run(opts)
	if c := findCheck(t, r, "cli-routing"); c.Level != LevelPass || !strings.Contains(c.Message, "claude-tmux") {
		t.Fatalf("a clean table passes and names the drivers: %+v", c)
	}
}

func TestPreflight_WithoutTheRoutingSeamTheProfileSeamsCompileALegacyTable(t *testing.T) {
	r, _ := Run(goodPipelineOptions(t))
	if c := findCheck(t, r, "cli-routing"); c.Level != LevelPass || !strings.Contains(c.Message, "claude-tmux") {
		t.Fatalf("no routing seam: the profile seams compile the legacy table: %+v", c)
	}
}

func TestPreflight_AProfileSeamThatCannotListHaltsTheRoutingCheck(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileLister = func() ([]string, error) { return nil, errors.New("profiles unreadable") }
	r, _ := Run(opts)
	if c := findCheck(t, r, "cli-routing"); c.Level != LevelHalt || !strings.Contains(c.Detail, "profiles unreadable") {
		t.Fatalf("an unlistable profile seam halts the routing check: %+v", c)
	}
}

func TestRoutedDrivers_IsTheUnionOfEveryAgentsResolvedChain(t *testing.T) {
	r, _, err := cliroute.Build(cliroute.Setup{Profiles: cliroute.SingleProfile{Agent: "scout", Profile: &profiles.Profile{Name: "scout", CLI: "codex-tmux", CLIFallback: []string{"claude-tmux"}}}})
	if err != nil {
		t.Fatal(err)
	}
	drivers, err := RoutedDrivers(r, []string{"scout", "nobody"})
	if err != nil || strings.Join(drivers, " ") != "claude-tmux codex-tmux" {
		t.Fatalf("drivers = %v, %v", drivers, err)
	}
	t.Setenv("EVOLVE_CLI", "agy-tmux")
	block := policy.CLIRouting{CLIs: []string{"claude"}, Default: []string{"claude"}}
	declared, _, err := cliroute.Build(cliroute.Setup{Policy: policy.Policy{CLIRouting: &block}, Profiles: cliroute.SingleProfile{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RoutedDrivers(declared, []string{"scout"}); err == nil {
		t.Fatal("a refused agent fails the driver list")
	}
}

func TestPreflight_ATableThatDoesNotCompileProbesNoProfileChain(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "codex-tmux"}, nil
	}
	opts.Routing = func() (Routing, error) {
		return Routing{}, errors.New("cliroute: the routing table has 1 error finding(s)")
	}
	var probed []string
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		probed = append(probed, bin)
		return doctor.Result{Tool: bin, Found: true}, nil
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(probed, "codex") || len(r.CLIVersions) != 0 || findCheck(t, r, "cli-routing").Level != LevelHalt {
		t.Fatalf("a refused table halts and probes nothing, never a profile chain the table may exclude: probed=%v versions=%v", probed, r.CLIVersions)
	}
}

func TestPreflight_WithoutARoutingSeamTheDriversComeFromTheResolver(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name}, nil
	}
	var probed []string
	opts.ProbeCLI = func(bin string) (doctor.Result, error) {
		probed = append(probed, bin)
		return doctor.Result{Tool: bin, Found: true}, nil
	}
	if _, err := Run(opts); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(probed, "claude") {
		t.Fatalf("a profile with no cli resolves to the claude default, so the resolver's preflight probes claude: %v", probed)
	}
}

func TestPreflight_NoSeamAndAMissingProfilesDirectoryWarnsAndNeverHalts(t *testing.T) {
	opts := goodPipelineOptions(t)
	opts.ProfileLister = func() ([]string, error) {
		return nil, &fs.PathError{Op: "open", Path: ".", Err: fs.ErrNotExist}
	}
	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}
	if c := findCheck(t, r, "cli-routing"); c.Level != LevelWarn || !strings.Contains(c.Detail, "does not exist") {
		t.Fatalf("with no seam a missing profiles directory is the legacy table's warning, as internal-looppreflight.md says: %+v", c)
	}
}

func TestCompileRouting_ToleratesOnlyAMissingProfilesDirectory(t *testing.T) {
	r, _, err := cliroute.Build(cliroute.Setup{Profiles: cliroute.SingleProfile{}})
	if err != nil {
		t.Fatal(err)
	}
	compiled := func() (*cliroute.Router, []cliroute.Finding, error) { return r, nil, nil }
	missing := func() ([]string, error) { return nil, &fs.PathError{Op: "open", Path: ".", Err: fs.ErrNotExist} }
	if routing, err := CompileRouting(compiled, missing); err != nil || len(routing.Drivers) != 0 {
		t.Fatalf("a missing profiles directory routes nothing and is no error: %+v %v", routing, err)
	}
	unreadable := func() ([]string, error) { return nil, errors.New("permission denied") }
	if _, err := CompileRouting(compiled, unreadable); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("any other list failure halts: %v", err)
	}
	refused := func() (*cliroute.Router, []cliroute.Finding, error) {
		return nil, []cliroute.Finding{{Severity: cliroute.SeverityError, Key: "cli_routing.clis"}}, errors.New("refused")
	}
	if routing, err := CompileRouting(refused, missing); err == nil || len(routing.Findings) != 1 {
		t.Fatalf("a refused compile carries its findings: %+v %v", routing, err)
	}
}
