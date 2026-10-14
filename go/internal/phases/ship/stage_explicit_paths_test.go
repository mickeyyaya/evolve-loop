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

type porcelainCapture struct {
	calls           [][]string
	porcelain       string
	ignored         []string
	checkIgnoreRC   int
	refuseAddPaths  []string
	refuseAddAlways string
}

func (c *porcelainCapture) runner() CmdRunner {
	return func(_ context.Context, name, _ string, args, _ []string,
		_ io.Reader, stdout, stderr io.Writer) (int, error) {
		c.calls = append(c.calls, append([]string{name}, args...))
		if name != "git" {
			return 0, nil
		}
		switch {
		case slices.Contains(args, "add") && c.refuseAddAlways != "":
			if stderr != nil {
				_, _ = io.WriteString(stderr, "The following paths are ignored by one of your .gitignore files:\n"+
					c.refuseAddAlways+"\nhint: Use -f if you really want to add them.\n")
			}
			return 1, nil
		case slices.Contains(args, "add") && len(c.refuseAddPaths) > 0:
			var named []string
			for _, p := range c.refuseAddPaths {
				if slices.Contains(args, p) {
					named = append(named, p)
				}
			}
			if len(named) > 0 {
				if stderr != nil {
					_, _ = io.WriteString(stderr, "The following paths are ignored by one of your .gitignore files:\n"+
						strings.Join(named, "\n")+"\nhint: Use -f if you really want to add them.\n")
				}
				return 1, nil
			}
			return 0, nil
		case slices.Contains(args, "diff") && slices.Contains(args, "--cached") &&
			slices.Contains(args, "--quiet"):
			return 1, nil
		case slices.Contains(args, "status") && slices.Contains(args, "--porcelain"):
			if stdout != nil {
				_, _ = io.WriteString(stdout, c.porcelain)
			}
			return 0, nil
		case slices.Contains(args, "check-ignore"):
			if c.checkIgnoreRC != 0 {
				return c.checkIgnoreRC, nil
			}
			var hit bool
			for _, p := range c.ignored {
				if slices.Contains(args, p) {
					if stdout != nil {
						_, _ = io.WriteString(stdout, p+"\n")
					}
					hit = true
				}
			}
			if hit {
				return 0, nil
			}
			return 1, nil
		}
		return 0, nil
	}
}

func (c *porcelainCapture) unscopedAddAll() []string {
	for _, call := range c.calls {
		if len(call) < 2 || call[0] != "git" {
			continue
		}
		args := call[1:]
		hasAdd, hasA, pathAfterDashDash := false, false, false
		for i, a := range args {
			switch a {
			case "add":
				hasAdd = true
			case "-A", "--all":
				hasA = true
			case "--":
				if i+1 < len(args) {
					pathAfterDashDash = true
				}
			}
		}
		if hasAdd && hasA && !pathAfterDashDash {
			return call
		}
	}
	return nil
}

func (c *porcelainCapture) gitCallWith(needles ...string) []string {
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

func (c *porcelainCapture) addPathspec() ([]string, bool) {
	for _, call := range c.calls {
		if call[0] != "git" || !slices.Contains(call[1:], "add") {
			continue
		}
		for i, a := range call {
			if a == "--" {
				return call[i+1:], true
			}
		}
		return nil, true
	}
	return nil, false
}

func writeWorkspaceReports(t *testing.T, declared ...string) string {
	t.Helper()
	ws := t.TempDir()
	var b strings.Builder
	b.WriteString("# Build Report\n\n## Files Changed\n\n")
	for _, p := range declared {
		b.WriteString("- `" + p + "`\n")
	}
	mustWrite(t, filepath.Join(ws, "build-report.md"), b.String())
	mustWrite(t, filepath.Join(ws, "test-report.md"), "# TDD Report\n\nno additional paths\n")
	return ws
}

func stageExplicitTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, rel := range []string{
		"go/internal/phases/ship/gitops.go",
		"go/internal/phases/ship/manifest.go",
		"sibling-lane-leak.txt",
	} {
		mustWrite(t, filepath.Join(root, filepath.FromSlash(rel)), "content of "+rel)
	}
	return root
}

func stageExplicitOpts(root, workspace string, class Class, runner CmdRunner) *Options {
	return &Options{
		Class:         class,
		ProjectRoot:   root,
		PluginRoot:    root,
		WorkspacePath: workspace,
		CommitMessage: "feat: explicit staging",
		Runner:        runner,
		Stderr:        io.Discard,
	}
}

func TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll(t *testing.T) {
	t.Parallel()
	root := stageExplicitTree(t)
	ws := writeWorkspaceReports(t,
		"go/internal/phases/ship/gitops.go",
		"go/internal/phases/ship/manifest.go")
	cap := &porcelainCapture{porcelain: " M go/internal/phases/ship/gitops.go\n?? sibling-lane-leak.txt\n"}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}

	if call := cap.unscopedAddAll(); call != nil {
		t.Errorf("RED: cycle ship staged with an UNSCOPED `git add -A` (%v) — must stage the declared path set explicitly (git_add_explicit_paths); `-A -- <paths>` is the allowed SCOPED sweep (staged-deletion handling)", call)
	}
	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd {
		t.Fatal("cycle ship never invoked git add at all — staging must never be silently skipped")
	}
	for _, want := range []string{
		"go/internal/phases/ship/gitops.go",
		"go/internal/phases/ship/manifest.go",
	} {
		if !slices.Contains(pathspec, want) {
			t.Errorf("declared path %q missing from staging pathspec %v", want, pathspec)
		}
	}
	if slices.Contains(pathspec, "sibling-lane-leak.txt") {
		t.Errorf("undeclared stray staged: %v — explicit staging must bind only declared paths", pathspec)
	}
}

func TestShipDirect_AReportlessWorkspaceAdoptsNoPath(t *testing.T) {
	t.Parallel()
	root := stageExplicitTree(t)
	emptyWS := t.TempDir()
	cap := &porcelainCapture{porcelain: " M go/internal/phases/ship/gitops.go\n?? docs/new-note.md\n"}
	opts := stageExplicitOpts(root, emptyWS, ClassManual, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(manual): %v", err)
	}
	if call := cap.unscopedAddAll(); call != nil {
		t.Errorf("a report-less workspace never falls back to `git add -A`: %v", call)
	}
	if pathspec, sawAdd := cap.addPathspec(); sawAdd {
		t.Fatalf("a cycle whose reports declare nothing adopts no path (its tracked edits ship through the audit binding's add -u; an empty `git add -A --` would stage the whole tree): %v", pathspec)
	}
}

func TestShipDirect_NoWorkspacePath_StillStagesExplicitly(t *testing.T) {
	t.Parallel()
	root := stageExplicitTree(t)
	cap := &porcelainCapture{porcelain: " M README.md\n"}
	opts := stageExplicitOpts(root, "", ClassManual, cap.runner())

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(manual, no workspace): %v", err)
	}

	if call := cap.unscopedAddAll(); call != nil {
		t.Errorf("RED: workspace-less manual ship staged with `git add -A` (%v)", call)
	}
	pathspec, sawAdd := cap.addPathspec()
	if !sawAdd || len(pathspec) == 0 {
		t.Fatalf("workspace-less manual ship must still stage the changed set explicitly (sawAdd=%v pathspec=%v)", sawAdd, pathspec)
	}
	if !slices.Contains(pathspec, "README.md") {
		t.Errorf("changed path README.md missing from pathspec %v", pathspec)
	}
}

func TestShipDirect_NonReleaseClasses_NeverAddAll(t *testing.T) {
	t.Parallel()
	for _, class := range []Class{ClassCycle, ClassManual, ClassTrivial} {
		t.Run(string(class), func(t *testing.T) {
			root := stageExplicitTree(t)
			ws := writeWorkspaceReports(t, "go/internal/phases/ship/gitops.go")
			cap := &porcelainCapture{porcelain: " M go/internal/phases/ship/gitops.go\n"}
			opts := stageExplicitOpts(root, ws, class, cap.runner())

			if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
				t.Fatalf("shipDirect(%s): %v", class, err)
			}
			if call := cap.unscopedAddAll(); call != nil {
				t.Errorf("class %s still stages with an UNSCOPED `git add -A`: %v", class, call)
			}
			if _, sawAdd := cap.addPathspec(); !sawAdd {
				t.Errorf("class %s never staged anything", class)
			}
		})
	}
}

func TestShipDirect_CycleClass_KeepsChurnDiscard(t *testing.T) {
	t.Parallel()
	root := initReleaseStagingTree(t)
	ws := writeWorkspaceReports(t, "go/evolve", "CHANGELOG.md")
	cap := &porcelainCapture{porcelain: " M CHANGELOG.md\n"}
	opts := stageExplicitOpts(root, ws, ClassCycle, cap.runner())
	opts.ShipBinaryPath = filepath.Join(root, "go", "evolve")

	if err := shipDirect(context.Background(), opts, &RunResult{}, "main"); err != nil {
		t.Fatalf("shipDirect(cycle): %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go", "evolve")); !os.IsNotExist(err) {
		t.Error("cycle ship skipped discardBinaryChurn — unaudited binary churn would ride into cycle commits")
	}
}
