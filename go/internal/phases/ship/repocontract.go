package ship

// repocontract.go — the ship-time repo-contract scanner pack (2026-08-05).
//
// Lane ships push directly to main (per-lane landing), so a landing that
// breaks a REPO-WIDE guard suite reds main until a console fix lands. Four
// live incidents in one week, each an operator CI-email storm: the router
// Digest injection (cycle-1250), the phase-catalog metadata stub (1262), the
// tracked profile stubs (v22.13.0 release red), and the incident-postmortem
// spec rework (1313). Per-cycle changed-scope testing structurally cannot
// catch them — the guard suites scan repo-wide state (on-disk catalogs,
// tracked profiles, rendering parity) that a config-only diff never selects.
//
// The pack runs the four guard packages in the lane worktree BEFORE the ship
// binds/pushes. They are existing deterministic tests with FP≈0 by
// construction: if one fails here, main's next run fails identically. A RED
// pack fails the ship closed with the dedicated CodeRepoContractGate
// (mirroring CodeManifestGate, cycle-1064) so the lane FAILs honestly in
// place instead of redding main. Dial: policy.json gates.repo_contract_gate
// ("enforce" default — see policy.go for the shadow-first deviation
// rationale; "off" disables).

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// repoContractPackages are the repo-wide guard suites whose breakage turned
// main red. Kept to the incident-proven set deliberately: every addition
// costs every ship wall-time and must carry the same FP≈0 property.
var repoContractPackages = []string{
	"./internal/phasespec/...",
	"./internal/profiles/...",
	"./internal/phasecoherence/...",
	"./internal/routingtest/...",
}

// repoContractTestFn is the seam for the pack execution (package var, mirrors
// the runner seams elsewhere in this package's tests). Production runs
// `go test` in the lane worktree's module dir.
var repoContractTestFn = defaultRepoContractTest

func defaultRepoContractTest(ctx context.Context, moduleDir string, stderr io.Writer) error {
	args := append([]string{"test", "-count=1"}, repoContractPackages...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleDir
	cmd.Stdout = stderr // scanner chatter is ship diagnostics
	cmd.Stderr = stderr
	return cmd.Run()
}

// runRepoContractGate executes the scanner pack per the resolved dial.
// Returns nil when the dial is off/empty-off or the pack is green; a RED pack
// returns the dedicated ship error so the router/debugger see a contract
// block, not a git failure.
func runRepoContractGate(ctx context.Context, gate, projectRoot string, stderr io.Writer) error {
	if gate != "enforce" {
		if gate != "" && gate != "off" {
			fmt.Fprintf(stderr, "[ship] repo-contract gate: unknown stage %q — treating as enforce (a typo must not silently disable a red-main guard)\n", gate)
		} else {
			return nil
		}
	}
	moduleDir := filepath.Join(projectRoot, "go")
	if err := repoContractTestFn(ctx, moduleDir, stderr); err != nil {
		return shiperr.NewShipError(shiperr.CodeRepoContractGate, shiperr.ShipClassPrecondition, shiperr.StageAtomicShip,
			fmt.Sprintf("repo-contract scanner pack RED in the lane worktree (%v) — pushing would red main; fix the violation in-lane (the four suites: phasespec, profiles, phasecoherence, routingtest)", err))
	}
	return nil
}
