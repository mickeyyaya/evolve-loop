package bridge

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/guidance"
)

const injectRepoRoot = "../../../.."

const cycleBoundary = "\n\n## Cycle Context\n"

func injectReq(t *testing.T, cli, root string, cycle int) core.BridgeRequest {
	t.Helper()
	return core.BridgeRequest{
		CLI: cli, Profile: "/p", Workspace: t.TempDir(), ArtifactPath: "/abs/report.md", Agent: "build",
		ProjectRoot: root, Cycle: cycle,
		Prompt: "PERSONA" + cycleBoundary + "- cycle: " + string(rune('0'+cycle)) + "\n",
	}
}

func rulesBlock(t *testing.T, prompt string) string {
	t.Helper()
	i := strings.Index(prompt, "## Rules\n\n")
	if i < 0 {
		t.Fatalf("prompt has no Rules block:\n%s", prompt)
	}
	rest := prompt[i:]
	return rest[:strings.Index(rest, "\n\n---\n\n")]
}

func TestComposePrompt_InjectManifestPutsL0AndPointerInTheRulesPrefix(t *testing.T) {
	t.Parallel()
	got, err := New().ComposePrompt(injectReq(t, "ollama-tmux", injectRepoRoot, 1))
	if err != nil {
		t.Fatal(err)
	}
	l0, err := guidance.L0Region(injectRepoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if block := rulesBlock(t, got.Prompt); block != "## Rules\n\n"+l0 {
		t.Errorf("Rules block is not exactly the L0 region:\n%s", block)
	}
	if !strings.Contains(got.Prompt, guidance.PointerLine) {
		t.Error("the pointer line must ride with L0")
	}
	if strings.Index(got.Prompt, "## Rules") > strings.Index(got.Prompt, "PERSONA") || strings.Index(got.Prompt, "## Rules") > strings.Index(got.Prompt, cycleBoundary) {
		t.Error("L0 must sit above the persona and the cycle-context boundary")
	}
}

func TestComposePrompt_InjectedL0IsByteStableAcrossCycles(t *testing.T) {
	t.Parallel()
	a := New()
	first, err := a.ComposePrompt(injectReq(t, "ollama-tmux", injectRepoRoot, 1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := a.ComposePrompt(injectReq(t, "ollama-tmux", injectRepoRoot, 2))
	if err != nil {
		t.Fatal(err)
	}
	if first.Prompt == second.Prompt {
		t.Fatal("the two cycles must differ in their cycle context, or the test proves nothing")
	}
	if a, b := rulesBlock(t, first.Prompt), rulesBlock(t, second.Prompt); a != b {
		t.Errorf("injected L0 differs between cycles:\n%q\n%q", a, b)
	}
	static := func(p string) string { return p[:strings.Index(p, cycleBoundary)] }
	if static(first.Prompt) != static(second.Prompt) {
		t.Error("the prompt above the cycle-context boundary must be byte-identical across cycles")
	}
}

func TestComposePrompt_ACliWithAFileInjectsNothing(t *testing.T) {
	t.Parallel()
	for _, cli := range []string{"claude-tmux", "codex-tmux", "agy-tmux"} {
		got, err := New().ComposePrompt(injectReq(t, cli, injectRepoRoot, 1))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got.Prompt, "## Rules") {
			t.Errorf("%s loads its own file; the bridge must not add a second copy", cli)
		}
	}
}

func TestComposePrompt_InjectKeepsTheProfileSystemPromptAfterL0(t *testing.T) {
	t.Parallel()
	req := injectReq(t, "ollama-tmux", injectRepoRoot, 1)
	req.SystemPrompt = "PROFILE-RULES"
	got, err := New().ComposePrompt(req)
	if err != nil {
		t.Fatal(err)
	}
	block := rulesBlock(t, got.Prompt)
	if l0 := strings.Index(block, guidance.PointerLine); l0 < 0 || strings.Index(block, "PROFILE-RULES") < l0 {
		t.Errorf("profile text must follow L0 in one Rules block:\n%s", block)
	}
}

func TestComposePrompt_InjectFailsLoudlyWithoutAnL0Region(t *testing.T) {
	t.Parallel()
	bare := t.TempDir()
	if err := os.WriteFile(filepath.Join(bare, "AGENTS.md"), []byte("# AGENTS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New().ComposePrompt(injectReq(t, "ollama-tmux", bare, 1)); !errors.Is(err, guidance.ErrNoL0Region) {
		t.Errorf("no region: err = %v, want ErrNoL0Region", err)
	}
	if _, err := New().ComposePrompt(injectReq(t, "ollama-tmux", t.TempDir(), 1)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("no file: err = %v, want a missing-file error", err)
	}
}

func TestComposePrompt_InjectRefusesAnEmptyProjectRoot(t *testing.T) {
	t.Parallel()
	if _, err := New().ComposePrompt(injectReq(t, "ollama-tmux", "", 1)); !errors.Is(err, ErrNoProjectRoot) {
		t.Errorf("err = %v, want ErrNoProjectRoot", err)
	}
}

func TestComposePrompt_AnUnknownCliFailsLoudly(t *testing.T) {
	t.Parallel()
	if _, err := New().ComposePrompt(injectReq(t, "no-such-cli", injectRepoRoot, 1)); err == nil {
		t.Error("a cli with no manifest must not compose a prompt")
	}
}

func TestLaunch_BuiltByNewDefaultDispatchesL0ToAnInjectSeat(t *testing.T) {
	t.Parallel()
	fe := &fakeEngine{}
	a := NewDefault(t.TempDir(), nil)
	a.engineFactory = func(map[string]string) core.Bridge { return fe }
	if _, err := a.Launch(context.Background(), injectReq(t, "ollama-tmux", injectRepoRoot, 1)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(fe.gotReq.Prompt, guidance.PointerLine) {
		t.Errorf("the engine received a prompt with no L0:\n%s", fe.gotReq.Prompt)
	}
}
