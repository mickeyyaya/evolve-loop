package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseblock"
)

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

func RecordPhaseIntegrity(path string, d phaseblock.Digest) error {
	return flock.WithPathLock(path, func() error {
		return recordIntegrityWithHooks(defaultHooks(), path, d)
	})
}

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
