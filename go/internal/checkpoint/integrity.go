package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseblock"
)

// upsertIntegrity returns a NEW chain with d appended, or with the existing
// entry for d.Phase replaced (idempotent regenerate/retrigger). It never
// mutates the input slice. Pure.
func upsertIntegrity(existing []phaseblock.Digest, d phaseblock.Digest) []phaseblock.Digest {
	out := make([]phaseblock.Digest, 0, len(existing)+1)
	replaced := false
	for _, e := range existing {
		if e.Phase == d.Phase {
			out = append(out, d)
			replaced = true
			continue
		}
		out = append(out, e)
	}
	if !replaced {
		out = append(out, d)
	}
	return out
}

// RecordPhaseIntegrity appends/upserts a phase's integrity digest into the
// checkpoint block of cycle-state.json, under the shared sidecar lock,
// atomically (temp + rename). Under fleet each lane's `path` is its OWN per-run
// file (core.ResolveCycleStatePath / ipcenv.CycleStateFileKey), so concurrent
// lanes' RecordPhaseIntegrity calls operate on DISTINCT files — no cross-lane
// clobber; flock still serializes same-file (intra-lane) goroutines (flock.go:8-9)
// so a read-modify-write never loses a peer's update. Every other state +
// checkpoint field is preserved.
func RecordPhaseIntegrity(path string, d phaseblock.Digest) error {
	return flock.WithPathLock(path, func() error {
		return recordIntegrityWithHooks(defaultHooks(), path, d)
	})
}

// recordIntegrityWithHooks is the lock-free inner (the hooks seam drives each
// error branch in tests). Callers MUST already hold flock.WithPathLock(path).
func recordIntegrityWithHooks(h hooks, path string, d phaseblock.Digest) error {
	b, err := h.readFile(path)
	if err != nil {
		return fmt.Errorf("checkpoint: read state: %w", err)
	}
	var state map[string]any
	if err := h.jsonUnmarshal(b, &state); err != nil {
		return fmt.Errorf("checkpoint: parse state: %w", err)
	}
	cp, _ := state["checkpoint"].(map[string]any)
	if cp == nil {
		cp = map[string]any{"enabled": true, "reason": string(ReasonPhaseComplete)}
	}
	merged := upsertIntegrity(decodeIntegrity(h, cp["phaseIntegrity"]), d)
	asAny, err := roundTripToAny(h, merged)
	if err != nil {
		return fmt.Errorf("checkpoint: encode integrity: %w", err)
	}
	cp["phaseIntegrity"] = asAny
	state["checkpoint"] = cp

	out, err := h.jsonMarshal(state)
	if err != nil {
		return fmt.Errorf("checkpoint: marshal state: %w", err)
	}
	// Distinct temp suffix (not the shared ".tmp" applyWithHooks uses): writers
	// already serialize under the same sidecar lock, but a unique name makes a
	// cross-writer temp collision structurally impossible, not merely guarded.
	tmp := path + ".integrity.tmp"
	if err := h.writeFile(tmp, out, 0o644); err != nil {
		return fmt.Errorf("checkpoint: write tmp: %w", err)
	}
	if err := h.rename(tmp, path); err != nil {
		_ = h.remove(tmp)
		return fmt.Errorf("checkpoint: rename: %w", err)
	}
	return nil
}

// readExistingIntegrity reads the recorded chain from path's checkpoint block,
// best-effort (missing file / garbled → nil). The phase-complete chokepoint
// calls this under the lock so its Compose write CARRIES the chain instead of
// clobbering it. Caller must already hold flock.WithPathLock(path).
func readExistingIntegrity(path string) []phaseblock.Digest {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s struct {
		Checkpoint struct {
			PhaseIntegrity []phaseblock.Digest `json:"phaseIntegrity"`
		} `json:"checkpoint"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil
	}
	return s.Checkpoint.PhaseIntegrity
}

// decodeIntegrity reads the existing phaseIntegrity (an untyped map slice) back
// into typed digests. A missing/garbled value yields an empty chain rather than
// an error — capture is best-effort and never blocks the cycle. The
// tamper-evident record of record is the append-only hash-chained LEDGER anchor
// (the second SSOT, ADR-0065); a security consumer verifies chain continuity
// there, so a reset of this resume-facing copy cannot silently erase the truth.
func decodeIntegrity(h hooks, v any) []phaseblock.Digest {
	if v == nil {
		return nil
	}
	b, err := h.jsonMarshal(v)
	if err != nil {
		return nil
	}
	var out []phaseblock.Digest
	if err := h.jsonUnmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

// roundTripToAny normalizes typed digests into the untyped map form the state
// document holds, so a single json.Marshal of the whole state stays canonical.
func roundTripToAny(h hooks, chain []phaseblock.Digest) (any, error) {
	b, err := h.jsonMarshal(chain)
	if err != nil {
		return nil, err
	}
	var out any
	if err := h.jsonUnmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
