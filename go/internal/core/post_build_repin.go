package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/phaseintegrity"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

var postBuildRepinProvenanceFn = defaultPostBuildRepinProvenance

func defaultPostBuildRepinProvenance(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {
	return version.Commit(), func(c string) bool {
		if c == "" {
			return false
		}
		return sysexec.Command(context.Background(), "git", "-C", projectRoot, "merge-base", "--is-ancestor", c, "HEAD").Run() == nil
	}
}

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
