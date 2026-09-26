package bridge

// tmux_identity_test.go — the pasted prompt ends by stating who the agent is.
//
// Cycle 1707's tdd agent listed tmux sessions, found its own, read its own prompt file, and refused the
// phase as a prompt injection racing "the real agent" — an hour lost to a process misunderstanding. Only
// the driver knows the session name, so it appends the statement to the bytes it pastes; the engine's
// composed prompt stays byte-identical across dispatches.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/phaseidentity"
)

var sessionLineRE = regexp.MustCompile(`(?m)^\[[a-z-]+\] session=(\S+) `)

func pastedIdentity(t *testing.T, fx launchFixture, stderr string) (pasted string, want string) {
	t.Helper()
	m := sessionLineRE.FindStringSubmatch(stderr)
	if m == nil {
		t.Fatalf("the driver did not report its session; stderr:\n%s", stderr)
	}
	pastedFile := filepath.Join(fx.ws, "resolved-prompt.txt")
	raw, err := os.ReadFile(pastedFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw), phaseidentity.Block(phaseidentity.Facts{
		Agent: "tdd", Cycle: 1707, Session: m[1],
		PromptFile: fx.promptFile, PastedFile: pastedFile, Artifact: fx.artifact,
	})
}

func TestTmuxDispatch_PastedPromptEndsWithTheAgentsIdentity(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	_, stderr := runTmux(t, fx, &fakeTmux{}, nil, "--agent=tdd", "--cycle=1707", "--worktree="+t.TempDir())
	pasted, want := pastedIdentity(t, fx, stderr)
	if !strings.HasSuffix(pasted, "\n\n"+want) {
		t.Fatalf("the pasted bytes must end with the identity block for this session:\n%s\nwant suffix:\n%s", pasted, want)
	}
	body, _ := os.ReadFile(fx.promptFile)
	if !strings.HasPrefix(pasted, strings.TrimRight(string(body), "\n")) {
		t.Fatalf("the composed prompt must still come first, unchanged:\n%s", pasted)
	}
	if strings.Count(pasted, phaseidentity.Heading) != 1 {
		t.Fatalf("one identity block, not %d", strings.Count(pasted, phaseidentity.Heading))
	}
}

func TestTmuxDispatch_IdentityIsStatedForEveryTmuxCLI(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	_, stderr := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, nil, "--allow-bypass", "--agent=tdd", "--cycle=1707", "--worktree="+t.TempDir())
	pasted, want := pastedIdentity(t, fx, stderr)
	if !strings.HasSuffix(pasted, "\n\n"+want) {
		t.Fatalf("codex-tmux must state the identity too:\n%s\nwant suffix:\n%s", pasted, want)
	}
}

func TestTmuxDispatch_StdoutCompletionClaimsNoArtifact(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	runTmux(t, fx, &fakeTmux{}, nil, "--agent=router", "--completion=stdout", "--worktree="+t.TempDir())
	raw, err := os.ReadFile(filepath.Join(fx.ws, "resolved-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sole writer") || strings.Contains(string(raw), fx.artifact) {
		t.Fatalf("a stdout-completion phase never writes the artifact, so the block must not claim it:\n%s", raw)
	}
	if !strings.Contains(string(raw), "The bridge reads your answer from this pane") {
		t.Fatalf("the block must say the pane is read:\n%s", raw)
	}
}

func TestTmuxDispatch_NoAgentNameMeansNoIdentity(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	runTmux(t, fx, &fakeTmux{}, nil, "--worktree="+t.TempDir())
	raw, err := os.ReadFile(filepath.Join(fx.ws, "resolved-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(fx.promptFile)
	if string(raw) != string(body)+"\n" {
		t.Fatalf("a probe pane has no phase to be; the pasted bytes must be the prompt alone:\n%s", raw)
	}
}
