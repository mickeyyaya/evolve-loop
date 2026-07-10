package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// post_build_repin.go — cycle 636, task ship-sha-repin-after-build.
//
// The cycle-514 boot healer auto-re-pins expected_ship_sha ONLY at boot; a
// legitimate within-version rebuild of go/bin/evolve between boots leaves a
// frozen pin that denied the terminal ship gate on 8 consecutive cycles
// (625->634, SELF_SHA_TAMPERED). This closes the class: the orchestrator re-pins
// immediately AFTER a successful build phase, reusing the SAME provenance-gated
// primitive the boot healer uses (phaseintegrity.RepinIfDrifted) so the two
// paths can never diverge.

// postBuildRepinProvenanceFn resolves the running binary's build-commit + the
// provenance predicate authorizing a post-build auto-repin. A package-var seam
// (mirrors cmd/evolve's shipRepinProvenanceFn) so the decision stays git-free —
// hence deterministic — under test. Production = defaultPostBuildRepinProvenance.
var postBuildRepinProvenanceFn = defaultPostBuildRepinProvenance

// defaultPostBuildRepinProvenance mirrors cmd/evolve's defaultShipRepinProvenance:
// the running binary's embedded build-commit, plus a closure asserting that commit
// is an ancestor of HEAD (`git merge-base --is-ancestor`). An empty commit is
// unverifiable (returns false), so a stripped/tampered binary can never
// self-authorize a post-build re-pin.
func defaultPostBuildRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return version.Commit(), func(c string) bool {
		if c == "" {
			return false
		}
		return exec.Command("git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}
}

// repinShipSHAAfterBuild re-pins <projectRoot>/.evolve/state.json:expected_ship_sha
// to the freshly-built <projectRoot>/go/bin/evolve after a successful build, via
// phaseintegrity.RepinIfDrifted. NEVER operator-authorized (unattended). Fail-open:
// a refusal/error (unverified provenance, absent binary, unwritable state) WARNs
// and returns a zero RepinResult — the ship gate stays the backstop.
func repinShipSHAAfterBuild(projectRoot string) phaseintegrity.RepinResult {
	statePath := filepath.Join(projectRoot, ".evolve", "state.json")
	binPath := filepath.Join(projectRoot, "go", "bin", "evolve")
	commit, prov := postBuildRepinProvenanceFn(projectRoot)
	res, err := phaseintegrity.RepinIfDrifted(statePath, binPath, commit, "", prov)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] post-build ship-SHA auto-repin declined (%v) — rebuild from committed source then `evolve reset-sha` to authorize, or investigate tampering\n", err)
		return phaseintegrity.RepinResult{}
	}
	if res.Repinned {
		fmt.Fprintf(os.Stderr, "[orchestrator] post-build: auto-repinned expected_ship_sha %.12s -> %.12s (authorized: %s) — legitimate rebuild self-healed after build\n", res.OldSHA, res.NewSHA, res.Authorized)
	}
	return res
}
