package ship

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

func manifestTestHasArg(args []string, needle string) bool {
	for _, a := range args {
		if a == needle {
			return true
		}
	}
	return false
}

// TestReconcileManifest_EnforceBlocksCrossLaneLeak is the ship-stage-explicit-
// paths guard (cycle-645): under enforce, a worktree carrying an untracked file
// that no phase report declared — typically a sibling fleet-lane's leaked
// artifact — must FAIL the ship closed, not silently commit it to main. Shadow
// (the default) must NOT block: behavior-preserving. The workspace build-report
// declares only docs/declared.md; the fake `git status --porcelain -uall`
// reports an undeclared untracked file.
func TestReconcileManifest_EnforceBlocksCrossLaneLeak(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"), "changed `docs/declared.md`")
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if manifestTestHasArg(args, "status") {
			fmt.Fprintln(stdout, "?? go/internal/foreign/leak_test.go")
		}
		return 0, nil
	}
	opts := &Options{WorkspacePath: ws, ProjectRoot: t.TempDir(), Runner: runner, Stdout: io.Discard, Stderr: io.Discard}

	// ENFORCE → fail closed, naming the leaked path.
	opts.ManifestGate = ManifestGateEnforce
	if err := reconcileManifest(context.Background(), opts, &RunResult{}, "wt", "main", "cycle"); err == nil || !strings.Contains(err.Error(), "leak_test.go") {
		t.Fatalf("enforce must block the out-of-manifest leak, got err=%v", err)
	}

	// SHADOW (default "") → do NOT block; log the leak instead (behavior-preserving).
	opts.ManifestGate = ""
	res := &RunResult{}
	if err := reconcileManifest(context.Background(), opts, res, "wt", "main", "cycle"); err != nil {
		t.Fatalf("shadow must not block: %v", err)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "leak_test.go") {
		t.Fatalf("shadow must LOG the out-of-manifest leak; logs=%v", res.Logs)
	}
}

// TestReconcileManifest_EnforceAllowsFullyDeclaredChange guards against a
// false-positive block: an enforce ship whose entire dirty set IS declared by
// the manifest must pass cleanly.
func TestReconcileManifest_EnforceAllowsFullyDeclaredChange(t *testing.T) {
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"), "changed `go/internal/foo/bar.go`")
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if manifestTestHasArg(args, "status") {
			fmt.Fprintln(stdout, " M go/internal/foo/bar.go")
		}
		return 0, nil
	}
	opts := &Options{WorkspacePath: ws, ProjectRoot: t.TempDir(), Runner: runner, ManifestGate: ManifestGateEnforce, Stdout: io.Discard, Stderr: io.Discard}
	if err := reconcileManifest(context.Background(), opts, &RunResult{}, "wt", "main", "cycle"); err != nil {
		t.Fatalf("enforce must NOT block a fully-declared change set: %v", err)
	}
}
