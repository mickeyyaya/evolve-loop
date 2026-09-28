package bridge

import (
	"context"
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

func launchLine(t *testing.T, tmux *fakeTmux) string {
	t.Helper()
	for _, keys := range tmux.sentKeys {
		if strings.Contains(keys, "claude ") || strings.HasSuffix(keys, "claude") {
			return keys
		}
	}
	t.Fatalf("no claude launch line among the typed keys: %q", tmux.sentKeys)
	return ""
}

func TestTmuxDispatch_ClaudeStatesItsAuthorityAsItsSystemPrompt(t *testing.T) {
	fx := newFixture(t, "claude-tmux", "plan")
	tmux := &fakeTmux{}
	_, stderr := runTmux(t, fx, tmux, nil, "--agent=tdd", "--cycle=1707", "--worktree="+t.TempDir())
	pasted, want := pastedIdentity(t, fx, stderr)
	authorityFile := filepath.Join(fx.ws, "pane-authority.md")
	stated, err := os.ReadFile(authorityFile)
	if err != nil || string(stated) != phaseidentity.Authority() {
		t.Fatalf("the system prompt file must hold the standing authority (err=%v):\n%s", err, stated)
	}
	if line := launchLine(t, tmux); !strings.Contains(line, "--append-system-prompt-file") || !strings.Contains(line, authorityFile) {
		t.Fatalf("claude must launch with the authority as its system prompt: %q", line)
	}
	body, _ := os.ReadFile(fx.promptFile)
	if pasted != strings.TrimRight(string(body), "\n")+"\n\n"+strings.TrimRight(want, "\n")+"\n" {
		t.Fatalf("the paste is the composed prompt and this dispatch's facts, without the authority:\n%s", pasted)
	}
}

func TestTmuxDispatch_ABootSmokeWithARealizedFileStillWritesTheFileItLaunchesWith(t *testing.T) {
	ws := t.TempDir()
	authorityFile := filepath.Join(ws, "pane-authority.md")
	cfg := &Config{Workspace: ws, Agent: "tdd", Realization: RealizeFor("claude-tmux", LaunchIntent{SystemPromptFile: authorityFile})}
	deps, _ := bootSmokeDeps(&fakeTmux{paneSeq: []string{"❯"}})
	if rc, _ := BootSmokeTest(context.Background(), "claude-tmux", cfg, deps); rc != ExitOK {
		t.Fatalf("rc = %d, want ExitOK", rc)
	}
	if stated, err := os.ReadFile(authorityFile); err != nil || string(stated) != phaseidentity.Authority() {
		t.Fatalf("a launch line that names the system prompt file needs the file (err=%v): %q", err, stated)
	}
}

func TestTmuxDispatch_IdentityIsStatedForEveryTmuxCLI(t *testing.T) {
	fx := newFixture(t, "codex-tmux", "")
	_, stderr := runTmuxCLI(t, fx, "codex-tmux", &fakeTmux{}, nil, "--allow-bypass", "--agent=tdd", "--cycle=1707", "--worktree="+t.TempDir())
	pasted, want := pastedIdentity(t, fx, stderr)
	if !strings.HasSuffix(pasted, "\n\n"+want) {
		t.Fatalf("codex-tmux must state the identity too:\n%s\nwant suffix:\n%s", pasted, want)
	}
	if !strings.Contains(pasted, "\n\n"+strings.TrimRight(phaseidentity.Authority(), "\n")+"\n\n"+want) {
		t.Fatalf("a CLI without a system prompt channel pastes the authority before the facts:\n%s", pasted)
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
	tmux := &fakeTmux{}
	runTmux(t, fx, tmux, nil, "--worktree="+t.TempDir())
	if line := launchLine(t, tmux); strings.Contains(line, "--append-system-prompt-file") {
		t.Fatalf("a pane with no phase launches with no system prompt file: %q", line)
	}
	raw, err := os.ReadFile(filepath.Join(fx.ws, "resolved-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(fx.promptFile)
	if string(raw) != string(body)+"\n" {
		t.Fatalf("a probe pane has no phase to be; the pasted bytes must be the prompt alone:\n%s", raw)
	}
}
