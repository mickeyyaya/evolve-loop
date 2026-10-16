package phaseintegrity

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

type ProvenanceVerified func(buildCommit string) bool

type RepinResult struct {
	Repinned   bool
	OldSHA     string
	NewSHA     string
	Authorized string
}

func RepinShipSHA(statePath, runningSHA, runningCommit, pluginVer string, prov ProvenanceVerified, operatorAuthorized bool) (RepinResult, error) {
	if runningSHA == "" {
		return RepinResult{}, fmt.Errorf("phaseintegrity: refusing to re-pin to an empty running sha")
	}
	if !filepath.IsAbs(statePath) {
		return RepinResult{}, fmt.Errorf("phaseintegrity: statePath must be absolute, got %q", statePath)
	}

	authorized := ""
	switch {
	case operatorAuthorized:
		authorized = "operator"
	case runningCommit != "" && prov != nil && prov(runningCommit):
		authorized = "provenance"
	default:
		return RepinResult{}, fmt.Errorf(
			"phaseintegrity: re-pin refused — binary build-commit %q is not provenance-verified (not an ancestor of HEAD); pass operator authorization (--reset-sha) to override",
			runningCommit)
	}

	if _, err := os.Stat(statePath); err != nil {
		return RepinResult{}, fmt.Errorf("phaseintegrity: re-pin: read state: %w", err)
	}

	res := RepinResult{NewSHA: runningSHA, Authorized: authorized}
	if err := statemap.UpdateStateMap(statePath, func(state map[string]any) {
		res.OldSHA, _ = state["expected_ship_sha"].(string)
		state["expected_ship_sha"] = runningSHA
		if pluginVer != "" {
			state["expected_ship_version"] = pluginVer
		}
	}); err != nil {
		return RepinResult{}, fmt.Errorf("phaseintegrity: re-pin: %w", err)
	}
	res.Repinned = true
	return res, nil
}
