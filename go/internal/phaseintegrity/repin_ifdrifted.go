package phaseintegrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

// RepinIfDrifted re-pins state.json:expected_ship_sha to the on-disk binary at
// binPath WHEN (and only when) its sha256 has drifted from the pin AND the
// running binary's build-commit is provenance-verified (the same gate the boot
// healer uses via RepinShipSHA). This is the ONE shared detect-drift +
// provenance-gate + repin path invoked BOTH at boot recovery AND immediately
// after a successful build phase, so the two never diverge ("never duplicate,
// centralize").
//
//   - no pin / binary absent / sha == pin -> RepinResult{Repinned:false}, nil
//     (no write; provenance is NOT consulted — nothing to authorize)
//   - drift + provenance-verified          -> RepinShipSHA fires ->
//     RepinResult{Repinned:true, NewSHA: <sha>}
//   - drift + provenance-UNVERIFIED         -> RepinResult{Repinned:false} + error;
//     pin left UNTOUCHED (anti-tamper preserved)
func RepinIfDrifted(statePath, binPath, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error) {
	pin, err := readShipPin(statePath)
	if err != nil || pin == "" {
		// No pin (or unreadable/torn state) ⇒ nothing to re-pin; short-circuit
		// before consulting provenance.
		return RepinResult{}, nil
	}
	data, err := os.ReadFile(binPath)
	if err != nil {
		// Binary absent ⇒ no mismatch signal; fail-open no-op.
		return RepinResult{}, nil
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:]) // matches core.ShipSHAMismatch's hashing
	if actual == pin {
		// No drift ⇒ nothing to re-pin; short-circuit BEFORE provenance.
		return RepinResult{}, nil
	}
	// Drift: delegate to the single provenance-gated writer. Unverified provenance
	// ⇒ RepinShipSHA refuses (error, pin untouched); verified ⇒ re-pins in place.
	// NEVER operator-authorized: this path is only ever reached unattended.
	return RepinShipSHA(statePath, actual, runningCommit, pluginVer, prov, false)
}

// readShipPin returns state.json:expected_ship_sha, or "" if the file is
// absent/torn or the key is missing.
func readShipPin(statePath string) (string, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return "", err
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", err
	}
	pin, _ := st["expected_ship_sha"].(string)
	return pin, nil
}
