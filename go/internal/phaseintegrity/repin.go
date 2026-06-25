package phaseintegrity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
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

	res := RepinResult{NewSHA: runningSHA, Authorized: authorized}
	err := flock.WithPathLock(statePath, func() error {
		b, err := os.ReadFile(statePath)
		if err != nil {
			return fmt.Errorf("read state: %w", err)
		}
		var state map[string]any
		if err := json.Unmarshal(b, &state); err != nil {
			return fmt.Errorf("parse state: %w", err)
		}
		res.OldSHA, _ = state["expected_ship_sha"].(string)
		state["expected_ship_sha"] = runningSHA
		if pluginVer != "" {
			state["expected_ship_version"] = pluginVer
		}
		out, err := json.MarshalIndent(state, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal state: %w", err)
		}
		tmp := statePath + ".repin.tmp"
		if err := os.WriteFile(tmp, append(out, '\n'), 0o644); err != nil {
			return fmt.Errorf("write tmp: %w", err)
		}
		if err := os.Rename(tmp, statePath); err != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("rename: %w", err)
		}
		return nil
	})
	if err != nil {
		return RepinResult{}, fmt.Errorf("phaseintegrity: re-pin: %w", err)
	}
	res.Repinned = true
	return res, nil
}
