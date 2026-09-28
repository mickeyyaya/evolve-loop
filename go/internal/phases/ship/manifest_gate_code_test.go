package ship

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// manifestLeakOpts builds a reconcileManifest fixture whose declared manifest
// covers docs/declared.md while `git status --porcelain -uall` reports an
// UNDECLARED untracked file — the cross-lane leak shape.
func manifestLeakOpts(t *testing.T) *Options {
	t.Helper()
	ws := t.TempDir()
	mustWrite(t, filepath.Join(ws, "build-report.md"), "changed `docs/declared.md`")
	runner := func(ctx context.Context, name, cwd string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		if manifestTestHasArg(args, "status") {
			fmt.Fprintln(stdout, "?? go/internal/foreign/leak_test.go")
		}
		return 0, nil
	}
	return &Options{WorkspacePath: ws, ProjectRoot: t.TempDir(), Runner: runner, Stdout: io.Discard, Stderr: io.Discard}
}

// The enforce-mode block must surface the DEDICATED core.CodeManifestGate,
// never the generic CodeGitStageFailed a real failing `git add` also emits.
// Ledger/debugger triage keys off Code, so reuse would make an integrity
// block indistinguishable from a transient git failure.
func TestReconcileManifest_EnforceCarriesManifestGateCode(t *testing.T) {
	opts := manifestLeakOpts(t)
	opts.ManifestGate = ManifestGateEnforce

	err := reconcileManifest(context.Background(), opts, &RunResult{}, "wt", "main", "cycle")
	if err == nil {
		t.Fatalf("enforce must block the out-of-manifest leak, got nil")
	}
	se, ok := core.AsShipError(err)
	if !ok {
		t.Fatalf("enforce block must be a structured *core.ShipError, got %T: %v", err, err)
	}
	if se.Code != core.CodeManifestGate {
		t.Errorf("Code = %q, want %q (dedicated manifest-gate code)", se.Code, core.CodeManifestGate)
	}
	if se.Code == core.CodeGitStageFailed {
		t.Errorf("Code must not remain the generic %q", core.CodeGitStageFailed)
	}
	if se.Class != core.ShipClassPrecondition || se.Stage != core.StageAtomicShip {
		t.Errorf("class/stage = %q/%q, want %q/%q", se.Class, se.Stage, core.ShipClassPrecondition, core.StageAtomicShip)
	}
	if !strings.Contains(se.Debug["out_of_manifest"], "leak_test.go") {
		t.Errorf("Debug[out_of_manifest] = %q, want the leaked path", se.Debug["out_of_manifest"])
	}
	if !strings.Contains(se.Message, "leak_test.go") {
		t.Errorf("message must name the leaked path, got %q", se.Message)
	}
}

// Introducing the dedicated code must not disturb the SHADOW path, which
// still returns nil and only logs; shadow returning a MANIFEST_GATE error
// would block every cycle today, since the gate is permanently shadow in
// production.
func TestReconcileManifest_ShadowUnaffectedByCodeChange(t *testing.T) {
	for _, mode := range []string{"", "shadow"} {
		opts := manifestLeakOpts(t)
		opts.ManifestGate = mode
		res := &RunResult{}
		if err := reconcileManifest(context.Background(), opts, res, "wt", "main", "cycle"); err != nil {
			t.Fatalf("ManifestGate=%q must not block: %v", mode, err)
		}
		if !strings.Contains(strings.Join(res.Logs, "\n"), "leak_test.go") {
			t.Errorf("ManifestGate=%q must LOG the leak; logs=%v", mode, res.Logs)
		}
	}
}
