package phaseintegrity

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/statemap"
)

// ProvenanceVerified reports whether a binary build-commit is trustworthy —
// typically "is this commit an ancestor of HEAD?". Injected so this package
// stays free of a hard git dependency in the decision path.
type ProvenanceVerified func(buildCommit string) bool

// RepinResult records the outcome of a re-pin so the operator surface can log
// an audit trail (who authorized it, what changed).
type RepinResult struct {
	Repinned   bool
	OldSHA     string
	NewSHA     string
	Authorized string // "provenance" | "operator"
}

// RepinShipSHA re-pins state.json:expected_ship_sha to the running binary —
// the sanctioned successor to hand-editing the file (ADR-0065). It re-pins
// ONLY when the binary is trustworthy:
//
//   - provenance: runningCommit is non-empty and prov(runningCommit) is true
//     (a legitimate rebuild from committed source), OR
//   - operatorAuthorized: the operator explicitly passed --reset-sha.
//
// Otherwise it REFUSES (Repinned=false + an actionable error), preserving the
// anti-tamper guarantee. The write is atomic (temp+rename) under the shared
// state.json sidecar lock, so it is safe against concurrent resume/fleet
// writers and never clobbers an unrelated state key.
func RepinShipSHA(statePath, runningSHA, runningCommit, pluginVer string, prov ProvenanceVerified, operatorAuthorized bool) (RepinResult, error) {
	if runningSHA == "" {
		return RepinResult{}, fmt.Errorf("phaseintegrity: refusing to re-pin to an empty running sha")
	}
	// Defense-in-depth: a security control's target must be a fixed absolute
	// path, never a caller/operator-supplied relative or traversal path.
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

	// Re-pin updates an EXISTING pin; refuse to CREATE state.json (a missing
	// file means there is nothing to re-pin). statemap.UpdateStateMap tolerates a
	// missing file by design, so guard existence explicitly before the RMW.
	if _, err := os.Stat(statePath); err != nil {
		return RepinResult{}, fmt.Errorf("phaseintegrity: re-pin: read state: %w", err)
	}

	// Route the read-modify-write through the single-source statemap adapter
	// (cycle-659): it holds the shared "<state.json>.lock" sidecar across the
	// whole RMW, so this is safe against concurrent resume/fleet writers and
	// preserves every unmodelled key. A malformed file aborts before the write.
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
