package audit

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// TestIntegrationTierGate_ScopesToTouchedPackages — the integration-tier gate must
// run `go test -race -tags integration` ONLY over the packages the cycle touched
// (the same O(change) scoping the apicover-enforce gate uses), NOT the whole suite.
//
// Root cause this pins (cycles 858/859/862, every cycle failing): the whole-suite
// run is parallel-unsafe under the loop's contended local environment — two fleet
// lanes running `go test -race -tags integration ./...` concurrently, plus real
// tmux/git, flaked heavy env-dependent tests (TestFleetSoak, TestShipFromWorktree)
// EVERY cycle, while CI (the same command, run once, isolated) stayed green.
// Scoping means a cycle touching internal/bridge never runs the fleet/ship tests;
// CI keeps the whole-suite backstop, exactly like apicover-enforce.
func TestIntegrationTierGate_ScopesToTouchedPackages(t *testing.T) {
	root, _ := goWorktree(t) // root/go/go.mod (module ciparitytest)
	// Build handoff naming ONE changed package → cycleTouchedGo true AND
	// changedPackagesForAudit returns exactly ["./cmd/foo/..."].
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-7")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/cmd/foo/main.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var listed bool
	var testArgs []string
	withFakeRunner(t, func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if name == "go" && len(args) > 0 {
			switch args[0] {
			case "list": // the scoped path must NOT shell out to `go list` at all
				listed = true
				_, _ = io.WriteString(so, "ciparitytest/cmd/foo\nciparitytest/internal/other\n")
			case "test":
				testArgs = append([]string(nil), args...)
			}
		}
		return 0, nil // clean → no offenders
	})

	off, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 7, ProjectRoot: root, Worktree: root})
	if err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if len(off) != 0 {
		t.Fatalf("clean run: want no offenders, got %v", off)
	}
	if listed {
		t.Fatal("scoped gate must derive packages from the change-set, not enumerate the whole module with `go list`")
	}
	joined := strings.Join(testArgs, " ")
	if !strings.Contains(joined, "./cmd/foo/...") {
		t.Fatalf("scoped gate must run the touched package ./cmd/foo/...; got go test args %v", testArgs)
	}
	if strings.Contains(joined, "internal/other") || strings.Contains(joined, "./...") {
		t.Fatalf("scoped gate must NOT run untouched packages / the whole suite; got %v", testArgs)
	}
	if !strings.Contains(joined, "-race") || !strings.Contains(joined, "integration") {
		t.Fatalf("scoped gate must preserve -race -tags integration (CI parity); got %v", testArgs)
	}
}

// TestIntegrationTierGate_BoundsParallelismForFDSafety — the gate's `go test`
// invocation must cap -p (concurrent package binaries) and -parallel (in-package
// parallel tests) so a run cannot exhaust file descriptors / memory under the
// loop's concurrent fleet lanes (the EBADF-on-pipe root cause). CI runs unbounded
// (isolated box); the local gate bounds execution concurrency only — same tests,
// same -race, same tags.
func TestIntegrationTierGate_BoundsParallelismForFDSafety(t *testing.T) {
	root, _ := goWorktree(t)
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-9")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/cmd/foo/main.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var testArgs []string
	withFakeRunner(t, func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if name == "go" && len(args) > 0 && args[0] == "test" {
			testArgs = append([]string(nil), args...)
		}
		return 0, nil
	})
	if _, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 9, ProjectRoot: root, Worktree: root}); err != nil {
		t.Fatalf("gate err: %v", err)
	}
	joined := strings.Join(testArgs, " ")
	if !strings.Contains(joined, "-p "+integrationTierParallelismArg) {
		t.Fatalf("gate must cap -p at %s for FD safety; got %v", integrationTierParallelismArg, testArgs)
	}
	if !strings.Contains(joined, "-parallel "+integrationTierParallelismArg) {
		t.Fatalf("gate must cap -parallel at %s for FD safety; got %v", integrationTierParallelismArg, testArgs)
	}
	// The caps must not displace the CI-parity flags.
	if !strings.Contains(joined, "-race") || !strings.Contains(joined, "integration") {
		t.Fatalf("caps must not drop -race/-tags integration; got %v", testArgs)
	}
}

// TestIntegrationTierGate_ModuleRootChangeRunsWholeSuite — a module-root file
// change (go.mod/go.sum/root main.go) derives to a `./...` pattern for which no
// narrower scope exists, so the gate falls back to the whole module (minus /acs/)
// to preserve CI parity on that rare cycle.
func TestIntegrationTierGate_ModuleRootChangeRunsWholeSuite(t *testing.T) {
	root, _ := goWorktree(t)
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-8")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A handoff naming a module-root Go file → changedPackagesForAudit yields the
	// "./..." pattern → integrationTierScope takes the whole-suite fallback.
	if err := os.WriteFile(filepath.Join(runDir, "handoff-build.json"),
		[]byte(`{"thrusts":[{"files_modified":["go/main.go"]}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var listed bool
	var testArgs []string
	withFakeRunner(t, func(_ context.Context, name, _ string, args, _ []string, _ io.Reader, so, se io.Writer) (int, error) {
		if name == "go" && len(args) > 0 {
			switch args[0] {
			case "list":
				listed = true
				_, _ = io.WriteString(so, "ciparitytest/cmd/foo\nciparitytest/acs/regression\nciparitytest/internal/other\n")
			case "test":
				testArgs = append([]string(nil), args...)
			}
		}
		return 0, nil
	})
	if _, err := integrationTierCheckDefault(core.PhaseRequest{Cycle: 8, ProjectRoot: root, Worktree: root}); err != nil {
		t.Fatalf("gate err: %v", err)
	}
	if !listed {
		t.Fatal("module-root change must fall back to the whole suite (go list ./...)")
	}
	joined := strings.Join(testArgs, " ")
	if !strings.Contains(joined, "ciparitytest/cmd/foo") || !strings.Contains(joined, "ciparitytest/internal/other") {
		t.Fatalf("whole-suite fallback must test all non-acs packages; got %v", testArgs)
	}
	if strings.Contains(joined, "acs/regression") {
		t.Fatalf("whole-suite fallback must still drop /acs/ (own -tags acs gate); got %v", testArgs)
	}
}
