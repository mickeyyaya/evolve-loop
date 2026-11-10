//go:build acs

package cycle2261

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	bridgePkg          = "github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	reasoningEffortKey = "model_reasoning_effort="
)

func toolListManifest() bridge.Manifest {
	return bridge.Manifest{
		CLI: "toolcli",
		Params: map[string]bridge.ParamSpec{
			"allowed_tools":    {Channel: "flag", Flag: "--allowedTools"},
			"disallowed_tools": {Channel: "flag", Flag: "--disallowedTools"},
		},
	}
}

func effortValues(args []string) []string {
	kept := slices.DeleteFunc(slices.Clone(args), func(a string) bool {
		return !strings.HasPrefix(a, reasoningEffortKey)
	})
	for i := range kept {
		kept[i] = strings.TrimPrefix(kept[i], reasoningEffortKey)
	}
	return kept
}

func runInPackageTest(t *testing.T, name string) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", "test", "-count=1", "-v", "-run", "^"+name+"$", bridgePkg)
	if code != 0 || err != nil {
		t.Fatalf("go test %s -run %s exited %d (err=%v)\nstdout:\n%s\nstderr:\n%s", bridgePkg, name, code, err, acsassert.OutputTail(stdout, 40), stderr)
	}
	if !strings.Contains(stdout, "--- PASS: "+name) {
		t.Fatalf("no PASS line for %s (renamed or not run?)\n%s", name, acsassert.OutputTail(stdout, 40))
	}
}

func TestC2261_001_AllowedAndDisallowedToolRunsKeepSharedValues(t *testing.T) {
	r := bridge.Realize(toolListManifest(), bridge.LaunchIntent{
		AllowedTools:    []string{"Read", "Write", "Bash"},
		DisallowedTools: []string{"Bash", "Write"},
	})
	want := []string{"--allowedTools", "Read", "Write", "Bash", "--disallowedTools", "Bash", "Write"}
	if !reflect.DeepEqual(r.LaunchFlags, want) {
		t.Errorf("Realize tool lists = %v, want %v: a value shared by two flag runs was dropped", r.LaunchFlags, want)
	}
}

func TestC2261_002_RawMultiValueFlagKeepsValuesSharedWithToolList(t *testing.T) {
	r := bridge.Realize(toolListManifest(), bridge.LaunchIntent{
		AllowedTools: []string{"Read", "Write"},
		RawByCLI:     map[string][]string{"toolcli": {"--tools", "Bash", "Read", "Write", "Grep", "Glob"}},
	})
	want := []string{"--allowedTools", "Read", "Write", "--tools", "Bash", "Read", "Write", "Grep", "Glob"}
	if !reflect.DeepEqual(r.LaunchFlags, want) {
		t.Errorf("Realize with raw --tools = %v, want %v", r.LaunchFlags, want)
	}
}

func TestC2261_003_HeadlessCodexEffortComesFromTheFamilyTable(t *testing.T) {
	family, err := bridge.LoadManifest("codex-tmux")
	if err != nil {
		t.Fatalf("LoadManifest(codex-tmux): %v", err)
	}
	table := family.Params["effort"].Values
	if len(table) == 0 {
		t.Fatal("codex-tmux declares no effort table")
	}
	for tier, familyTokens := range table {
		got := bridge.RealizeFor("codex", bridge.LaunchIntent{Effort: tier}).LaunchFlags
		if v := effortValues(got); !slices.Equal(v, []string{tier}) {
			t.Errorf("effort %q: headless %s values = %v in %v, want exactly [%s]", tier, reasoningEffortKey, v, got, tier)
		}
		for _, tok := range got {
			if !slices.Contains(familyTokens, tok) {
				t.Errorf("effort %q: headless token %q is not in the family table %v", tier, tok, familyTokens)
			}
		}
	}
	for _, unknown := range []string{"", "bogus"} {
		if v := effortValues(bridge.RealizeFor("codex", bridge.LaunchIntent{Effort: unknown}).LaunchFlags); len(v) != 0 {
			t.Errorf("effort %q realized %s%v, want no reasoning key", unknown, reasoningEffortKey, v)
		}
	}
}

func TestC2261_004_HeadlessCodexDriverRunsTheResolvedEffortNotALiteral(t *testing.T) {
	driver, ok := bridge.LookupDriver("codex")
	if !ok {
		t.Fatal("no codex driver registered")
	}
	for _, tier := range []string{"low", "medium", "xhigh"} {
		var gotArgs []string
		runner := func(_ context.Context, _, _ string, args, _ []string, _ io.Reader, _, _ io.Writer) (int, error) {
			gotArgs = append([]string(nil), args...)
			return 0, nil
		}
		dir := t.TempDir()
		prompt := filepath.Join(dir, "prompt.md")
		if err := os.WriteFile(prompt, []byte("implement"), 0o600); err != nil {
			t.Fatal(err)
		}
		var stderr bytes.Buffer
		deps := bridge.Deps{Runner: runner, Stderr: &stderr,
			Env: map[string]string{"ANTHROPIC_API_KEY": "", "ANTHROPIC_BASE_URL": "", "OPENAI_API_KEY": ""}}
		rc, err := driver.Launch(context.Background(), &bridge.Config{
			CLI: "codex", PromptFile: prompt, Model: "auto", Worktree: dir,
			Artifact:  filepath.Join(dir, "artifact.md"),
			StdoutLog: filepath.Join(dir, "out.log"), StderrLog: filepath.Join(dir, "err.log"),
			Realization: bridge.RealizeFor("codex", bridge.LaunchIntent{Effort: tier}),
		}, deps)
		if err != nil || rc != 0 {
			t.Fatalf("effort %q: Launch rc=%d err=%v stderr=%q", tier, rc, err, stderr.String())
		}
		if v := effortValues(gotArgs); !slices.Equal(v, []string{tier}) {
			t.Errorf("effort %q: codex argv %s values = %v in %v, want exactly [%s]", tier, reasoningEffortKey, v, gotArgs, tier)
		}
	}
}

func TestC2261_005_CapabilityMatrixHasACodexEffortRow(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "architecture", "cli-capability-matrix.md")
	if !acsassert.LineContainsAll(doc, "model_reasoning_effort", "plan_mode_reasoning_effort") {
		t.Errorf("%s has no single line naming model_reasoning_effort and plan_mode_reasoning_effort", doc)
	}
}

func TestC2261_006_ResolvedLaunchEffortReachesTheHeadlessArgvThroughLaunchIntent(t *testing.T) {
	runInPackageTest(t, "TestCodexEffort_LaunchIntentEffortReachesHeadlessArgv")
}

func TestC2261_007_EffortGuardMatchesOnlyTheKeyAtTokenStart(t *testing.T) {
	runInPackageTest(t, "TestArgsContainEffort_MatchesOnlyTheKeyAtTokenStart")
}
