package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type stagingCapture struct {
	calls [][]string
}

func (c *stagingCapture) runner() CmdRunner {
	return func(_ context.Context, name, _ string, args, _ []string,
		_ io.Reader, _, _ io.Writer) (int, error) {
		c.calls = append(c.calls, append([]string{name}, args...))
		if name == "git" && len(args) >= 3 && args[0] == "diff" && args[1] == "--cached" && args[2] == "--quiet" {
			return 1, nil
		}
		return 0, nil
	}
}

func (c *stagingCapture) gitCallWith(needles ...string) []string {
	for _, call := range c.calls {
		if call[0] != "git" {
			continue
		}
		ok := true
		for _, n := range needles {
			if !slices.Contains(call[1:], n) {
				ok = false
				break
			}
		}
		if ok {
			return call
		}
	}
	return nil
}

func initReleaseStagingTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		".codex-plugin/plugin.json",
		"skills/loop/SKILL.md",
		"README.md",
		"CHANGELOG.md",
		"go/evolve",
		"evolve.log",
	} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "content of "+rel)
	}
	return root
}

func releaseStagingOpts(root string, class Class, runner CmdRunner) *Options {
	return &Options{
		Class:          class,
		ProjectRoot:    root,
		PluginRoot:     root,
		CommitMessage:  "release: v9.9.9",
		Runner:         runner,
		Stderr:         io.Discard,
		ShipBinaryPath: filepath.Join(root, "go", "evolve"),
	}
}

func TestShipDirect_ReleaseClass_StagesExplicitSetKeepsBinary(t *testing.T) {
	t.Parallel()
	cap := &stagingCapture{}
	root := initReleaseStagingTree(t)
	opts := releaseStagingOpts(root, ClassRelease, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(release): %v", err)
	}

	if call := cap.gitCallWith("checkout", "--", "go/evolve"); call != nil {
		t.Errorf("RED (v18.5.0): release ship discarded the rebuilt binary via %v — the rebuild-binary step is its audited producer; the release commit must include it", call)
	}
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); err != nil {
		t.Errorf("rebuilt binary removed from disk during release ship: %v", err)
	}

	if call := cap.gitCallWith("add", "-A"); call != nil {
		t.Errorf("RED (v18.4.0 sweep): release ship staged with `git add -A` (%v) — untracked operator files ride into release commits; must stage the explicit release set", call)
	}
	addCall := cap.gitCallWith("add")
	if addCall == nil {
		t.Fatal("release ship never invoked git add at all")
	}
	joined := strings.Join(addCall, " ")
	for _, want := range []string{
		".claude-plugin/plugin.json",
		".claude-plugin/marketplace.json",
		".codex-plugin/plugin.json",
		"skills/loop/SKILL.md",
		"README.md",
		"CHANGELOG.md",
		"go/evolve",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("release staging missing %s (got: %v)", want, addCall)
		}
	}
	if strings.Contains(joined, "evolve.log") {
		t.Errorf("release staging swept the untracked stray evolve.log: %v", addCall)
	}
}
