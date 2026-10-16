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

type Probe func(check string) bool

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

type fullManifest struct {
	capabilities map[string]capabilityDef
	probes       []probeDef
}

type capabilityDef struct {
	fixedMode string
	dflt      string
}

type probeDef struct {
	check       string
	ifTrueMode  string
	ifFalseMode string
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

func tierFromManifest(m fullManifest, probe Probe) string {
	low := rankFull
	for name, def := range m.capabilities {
		if r := modeRank(resolveMode(name, def, m.probes, probe)); r < low {
			low = r
		}
	}
	return rankToMode(low)
}

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
	}
	return mode
}

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
