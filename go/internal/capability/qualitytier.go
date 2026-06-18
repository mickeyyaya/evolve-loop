package capability

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// Probe reports whether a named capability probe (e.g. "claude_on_path") holds.
// It is the seam that replaces _capability-check.sh's `command -v` / `uname`
// checks, so callers — and tests — can inject deterministic probe results.
type Probe func(check string) bool

// DefaultProbe mirrors the probe functions declared in _capability-check.sh:
// binary presence for claude/agy, OS-gated binary presence for the sandbox
// probes. Unknown probe names report false (bash `run_probe` returns non-zero
// for unrecognized checks, recorded as false).
func DefaultProbe(check string) bool {
	switch check {
	case "claude_on_path":
		return onPath("claude")
	case "agy_on_path":
		return onPath("agy")
	case "sandbox_exec_available":
		return runtime.GOOS == "darwin" && onPath("sandbox-exec")
	case "bwrap_available":
		return runtime.GOOS == "linux" && onPath("bwrap")
	default:
		return false
	}
}

func onPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// QualityTier resolves <adaptersDir>/<cli>.capabilities.json to its aggregate
// quality tier — the lowest mode across all declared capabilities, ranked
// none < degraded < hybrid < full. It is the Go port of _capability-check.sh's
// quality_tier computation.
//
// A missing or malformed manifest yields ("unknown", err): the bash caller
// (consensus-dispatch) shells out to _capability-check.sh and treats any
// non-JSON / non-zero result as tier "unknown", which is then excluded by a
// require_min_tier filter of hybrid or above. A nil probe uses DefaultProbe.
func QualityTier(adaptersDir, cli string, probe Probe) (string, error) {
	m, err := loadFullManifest(adaptersDir, cli)
	if err != nil {
		return "unknown", err
	}
	if probe == nil {
		probe = DefaultProbe
	}
	return tierFromManifest(m, probe), nil
}

// fullManifest is the richer parse needed for quality-tier resolution: the
// capabilities map (each entry a fixed-mode string OR an object with a
// default) plus the probe table. Distinct from Manifest, which parses only the
// supports block for the dispatch-plan envelope.
type fullManifest struct {
	capabilities map[string]capabilityDef
	probes       []probeDef
}

type capabilityDef struct {
	fixedMode string // non-empty ⟺ the JSON value was a bare string
	dflt      string // object form's ".default"
}

type probeDef struct {
	check       string
	ifTrueMode  string
	ifFalseMode string // parsed for fidelity only — see resolveMode for why it is dead in the live bash
	appliesTo   []string
}

func loadFullManifest(adaptersDir, cli string) (fullManifest, error) {
	path := filepath.Join(adaptersDir, cli+".capabilities.json")
	body, err := os.ReadFile(path)
	if err != nil {
		return fullManifest{}, fmt.Errorf("capability: read %s: %w", path, err)
	}
	return parseFullManifest(body)
}

func parseFullManifest(body []byte) (fullManifest, error) {
	var doc struct {
		Capabilities map[string]json.RawMessage `json:"capabilities"`
		Probes       []struct {
			Check       string   `json:"check"`
			IfTrueMode  string   `json:"if_true_mode"`
			IfFalseMode string   `json:"if_false_mode"`
			AppliesTo   []string `json:"applies_to"`
		} `json:"probes"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return fullManifest{}, fmt.Errorf("capability: parse manifest: %w", err)
	}
	m := fullManifest{capabilities: make(map[string]capabilityDef, len(doc.Capabilities))}
	for name, raw := range doc.Capabilities {
		def, err := parseCapabilityDef(raw)
		if err != nil {
			return fullManifest{}, fmt.Errorf("capability: %q: %w", name, err)
		}
		m.capabilities[name] = def
	}
	for _, p := range doc.Probes {
		m.probes = append(m.probes, probeDef{
			check:       p.Check,
			ifTrueMode:  p.IfTrueMode,
			ifFalseMode: p.IfFalseMode,
			appliesTo:   p.AppliesTo,
		})
	}
	return m, nil
}

// parseCapabilityDef branches on the JSON value shape: a bare string is a fixed
// mode; an object carries a ".default" resolved against the probes.
func parseCapabilityDef(raw json.RawMessage) (capabilityDef, error) {
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return capabilityDef{}, err
		}
		return capabilityDef{fixedMode: s}, nil
	}
	var obj struct {
		Default string `json:"default"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return capabilityDef{}, err
	}
	return capabilityDef{dflt: obj.Default}, nil
}

// tierFromManifest computes the aggregate quality tier: the lowest capability
// mode. An empty capabilities map yields "full" (low stays at rankFull),
// matching the bash LOW_RANK=3 initial value.
func tierFromManifest(m fullManifest, probe Probe) string {
	low := rankFull
	for name, def := range m.capabilities {
		if r := modeRank(resolveMode(name, def, m.probes, probe)); r < low {
			low = r
		}
	}
	return rankToMode(low)
}

// resolveMode resolves one capability to its mode. Fixed (string) caps return
// directly. Object caps start at the default and, for each applicable probe in
// manifest order, upgrade to if_true_mode when the probe holds (then stop).
//
// PARITY QUIRK: a probe that returns false does NOT apply if_false_mode. The
// live _capability-check.sh reads the probe result with jq `.[k] // "unknown"`,
// and jq's `//` treats false (like null) as empty — collapsing a false result
// to "unknown", which matches neither the ==true nor ==false branch. So
// if_false_mode is dead code in the shell, and a false probe leaves the
// capability at its default. We replicate that exactly (this migration
// preserves behavior) and keep ifFalseMode parsed so the divergence stays
// auditable.
//
// TODO(follow-up, post-migration): the shell bug makes off-PATH
// gemini/codex/agy resolve to default `none` instead of `degraded`; fix the
// intended semantics in one place here, not in the shell that is being deleted.
func resolveMode(cap string, def capabilityDef, probes []probeDef, probe Probe) string {
	if def.fixedMode != "" {
		return def.fixedMode
	}
	mode := def.dflt
	for _, p := range probes {
		if !probeApplies(p, cap) {
			continue
		}
		if probe(p.check) {
			mode = p.ifTrueMode
			break
		}
		// false/unknown → no-op (see PARITY QUIRK above)
	}
	return mode
}

// probeApplies mirrors the bash select: an empty applies_to matches every
// capability; otherwise the capability name must be listed.
func probeApplies(p probeDef, cap string) bool {
	if len(p.appliesTo) == 0 {
		return true
	}
	for _, c := range p.appliesTo {
		if c == cap {
			return true
		}
	}
	return false
}

const (
	rankNone     = 0
	rankDegraded = 1
	rankHybrid   = 2
	rankFull     = 3
)

// modeRank maps a mode label to its rank. Unknown labels rank as none (0),
// matching bash mode_rank's default branch.
func modeRank(mode string) int {
	switch mode {
	case "full":
		return rankFull
	case "hybrid":
		return rankHybrid
	case "degraded":
		return rankDegraded
	default:
		return rankNone
	}
}

func rankToMode(r int) string {
	switch r {
	case rankFull:
		return "full"
	case rankHybrid:
		return "hybrid"
	case rankDegraded:
		return "degraded"
	default:
		return "none"
	}
}
