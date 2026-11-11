package main

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/cliroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const modelClassifierProfileName = "model-classifier"

func modelClassifierProfile(t *testing.T) profiles.Profile {
	t.Helper()
	prof, err := profiles.NewFromDir(filepath.Join("..", "..", "..", ".evolve", "profiles")).Get(modelClassifierProfileName)
	if err != nil {
		t.Fatalf("the model classifier profile does not load: %v", err)
	}
	return prof
}

func TestClassifierDrivers_ABenchedDriverGoesLast(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	table := `{"cli_routing":{"clis":["codex","claude","agy"],"default":["codex","claude","agy"]}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(table), 0o644); err != nil {
		t.Fatal(err)
	}
	var benchRoot, benchLabel string
	host := cliroute.Host{
		Bench: func(projectRoot, label string, plan llmroute.Plan) llmroute.Plan {
			benchRoot, benchLabel = projectRoot, label
			return llmroute.ApplyDriverBench(plan, map[string]time.Time{"codex-tmux": time.Now()})
		},
	}

	drivers, err := classifierPreference(root, host)

	if err != nil {
		t.Fatalf("classifierPreference: %v", err)
	}
	if want := []string{"claude-tmux", "agy-tmux", "codex-tmux"}; !reflect.DeepEqual(drivers, want) {
		t.Fatalf("drivers = %v, want the benched codex-tmux last: %v", drivers, want)
	}
	if benchRoot != root || benchLabel != cliroute.ClassifierAgent {
		t.Errorf("the bench saw root %q label %q, want the project root %q and %q", benchRoot, benchLabel, root, cliroute.ClassifierAgent)
	}
}

func TestClassifierDrivers_AnUnreadyFamilyDropsOut(t *testing.T) {
	t.Parallel()
	drivers, err := classifierDrivers(classifierRouter(t, policy.Policy{}), t.TempDir())
	if err != nil {
		t.Fatalf("classifierDrivers: %v", err)
	}

	got := pickClassifierCLI([]string{"claude", "agy"}, drivers)

	if want := []string{"claude-tmux", "agy-tmux"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("chain = %v, want %v: the codex family is not ready, so codex-tmux leaves", got, want)
	}
}

func TestClassifierDispatch_EachDriverOfTheChainIsRegistered(t *testing.T) {
	t.Parallel()
	block := policy.CLIRouting{
		CLIs:    []string{"agy-claude-tmux", "claude-p", "claude-tmux", "agy-tmux", "codex-tmux"},
		Default: []string{"agy-claude-tmux", "claude-p", "claude-tmux", "agy-tmux", "codex-tmux"},
	}
	for name, pol := range map[string]policy.Policy{"legacy": {}, "declared": {CLIRouting: &block}} {
		drivers, err := classifierDrivers(classifierRouter(t, pol), t.TempDir())
		if err != nil {
			t.Fatalf("%s: classifierDrivers: %v", name, err)
		}
		if len(drivers) == 0 {
			t.Fatalf("%s: the chain is empty", name)
		}
		for _, driver := range drivers {
			if _, ok := bridge.LookupDriver(driver); !ok {
				t.Errorf("%s: driver %q of the classifier chain has no entry in the bridge registry", name, driver)
			}
		}
	}
}

func TestModelClassifierProfile_DeniesBashAndTheWeb(t *testing.T) {
	t.Parallel()
	prof := modelClassifierProfile(t)
	for _, tool := range []string{"Bash", "WebFetch", "WebSearch", "Task", "NotebookEdit"} {
		if !slices.Contains(prof.DisallowedTools, tool) {
			t.Errorf("the profile does not deny %s: %v", tool, prof.DisallowedTools)
		}
	}
	if prof.Sandbox == nil || !prof.Sandbox.Enabled || len(prof.Sandbox.WriteSubpaths) == 0 {
		t.Errorf("the profile needs an enabled sandbox with a write scope: %+v", prof.Sandbox)
	}
	for _, tool := range prof.AllowedTools {
		if strings.HasPrefix(tool, "Bash") {
			t.Errorf("the profile grants a shell tool: %s", tool)
		}
	}

	d := bridgePromptDispatcher{workspace: t.TempDir(), projectRoot: "/project"}
	if got := d.request("claude-p", "P").Profile; got != filepath.Join("/project", ".evolve", "profiles", modelClassifierProfileName+".json") {
		t.Errorf("the request names profile %q, want the model classifier profile", got)
	}

	var realizing []string
	for _, name := range bridge.ManifestNames() {
		m, err := bridge.LoadManifest(name)
		if err != nil {
			t.Fatalf("LoadManifest %s: %v", name, err)
		}
		if spec, ok := m.Params["disallowed_tools"]; ok && spec.Channel == "flag" && spec.Flag != "" {
			realizing = append(realizing, name)
		}
	}
	slices.Sort(realizing)
	if want := []string{"claude-p", "claude-tmux"}; !reflect.DeepEqual(realizing, want) {
		t.Errorf("drivers that realize the deny list = %v, want %v; for any other driver the sandbox is the only containment", realizing, want)
	}
}

func TestModelClassifierProfile_GrantMatchesTheArtifactOfTheWorkspace(t *testing.T) {
	t.Parallel()
	prof := modelClassifierProfile(t)
	root := t.TempDir()
	granted := func(artifact string) bool {
		rel, err := filepath.Rel(root, artifact)
		if err != nil {
			return false
		}
		for _, tool := range []string{"Write", "Edit"} {
			ok := false
			for _, grant := range prof.AllowedTools {
				pattern, found := strings.CutPrefix(grant, tool+"(")
				if !found {
					continue
				}
				if m, _ := path.Match(strings.TrimSuffix(pattern, ")"), filepath.ToSlash(rel)); m {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
		return true
	}

	inside := bridgePromptDispatcher{workspace: filepath.Join(root, ".evolve", "ephemeral", "model-classifier", "probe-1"), projectRoot: root}
	outside := bridgePromptDispatcher{workspace: t.TempDir(), projectRoot: root}

	if !granted(inside.request("claude-p", "P").ArtifactPath) {
		t.Errorf("the grant %v does not match the artifact %s", prof.AllowedTools, inside.request("claude-p", "P").ArtifactPath)
	}
	if granted(outside.request("claude-p", "P").ArtifactPath) {
		t.Errorf("an artifact under the temp directory must not match the grant: %v", prof.AllowedTools)
	}
}

func TestLiveRefresh_TheScratchWorkspaceLivesUnderTheProject(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	got := classifierScratchParent(root)
	if want := filepath.Join(root, ".evolve", "ephemeral", "model-classifier"); got != want {
		t.Fatalf("scratch parent = %q, want %q", got, want)
	}
	scratch, err := classifierScratch(root)
	if err != nil {
		t.Fatalf("classifierScratch: %v", err)
	}
	if filepath.Dir(scratch) != got {
		t.Fatalf("scratch %q lives under %q, want %q", scratch, filepath.Dir(scratch), got)
	}
}

func TestClassifierChain_ACLIHealthBenchMovesTheBenchedDriverLast(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	pol := `{"cli_routing":{"clis":["codex","claude","agy"],"default":["codex","claude","agy"]}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := clihealth.NewStore(root, time.Now).BenchWall("codex", "rate_limit", ""); err != nil {
		t.Fatalf("bench codex: %v", err)
	}

	got, err := classifierChain(root, []string{"codex", "claude", "agy"}, io.Discard)

	if err != nil {
		t.Fatalf("classifierChain: %v", err)
	}
	if want := []string{"claude-tmux", "agy-tmux", "codex-tmux"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("chain = %v, want the benched codex-tmux last: %v", got, want)
	}
}

func TestClassifierChain_AnUnreadyFamilyLeavesTheChain(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	pol := `{"cli_routing":{"clis":["codex","claude","agy"],"default":["codex","claude","agy"]}}`
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(pol), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := classifierChain(root, []string{"claude"}, io.Discard)

	if err != nil {
		t.Fatalf("classifierChain: %v", err)
	}
	if want := []string{"claude-tmux"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("chain = %v, want only the driver of the ready family: %v", got, want)
	}
}

func TestClassifierChain_ARefusedRouteIsAnError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "profiles"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(`{"cli_routing":{"clis":["agy"],"default":["agy"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := classifierChain(root, []string{"claude"}, io.Discard); err == nil {
		t.Fatal("a table that cannot route the classifier must refuse")
	}
}
