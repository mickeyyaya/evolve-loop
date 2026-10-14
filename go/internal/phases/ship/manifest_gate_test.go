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

func TestReconcileManifest_EnforceBlocksCrossLaneLeak(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"), "changed `docs/declared.md`")
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if manifestTestHasArg(args, "status") {
			fmt.Fprintln(stdout, "?? go/internal/foreign/leak_test.go")
		}
		return 0, nil
	}
	opts := &Options{WorkspacePath: ws, ProjectRoot: t.TempDir(), Runner: runner, Stdout: io.Discard, Stderr: io.Discard}

	opts.ManifestGate = ManifestGateEnforce
	if err := reconcileManifest(context.Background(), opts, &RunResult{}, "wt", "main", "cycle"); err == nil || !strings.Contains(err.Error(), "leak_test.go") {
		t.Fatalf("enforce must block the out-of-manifest leak, got err=%v", err)
	}

	opts.ManifestGate = ""
	res := &RunResult{}
	if err := reconcileManifest(context.Background(), opts, res, "wt", "main", "cycle"); err != nil {
		t.Fatalf("shadow must not block: %v", err)
	}
	if !strings.Contains(strings.Join(res.Logs, "\n"), "leak_test.go") {
		t.Fatalf("shadow must LOG the out-of-manifest leak; logs=%v", res.Logs)
	}
}

func TestReconcileManifest_EnforceAllowsFullyDeclaredChange(t *testing.T) {
	t.Parallel()
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
