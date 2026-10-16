// Package directives loads the operator-directives cascade, a global and a per-loop layer, and renders the block injected into phase prompts.
// See docs/architecture/packages/internal-directives.md.
package directives

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

const safetyPreamble = "_Runtime operator guidance, snapshotted at cycle start. " +
	"Global directives are authoritative for safety and may only be tightened, never weakened. " +
	"Binary gates (EGPS, ship-gate, policy floor) remain the only hard trust boundary — guidance does not override a gate._"

type Layer struct {
	Name string
	Path string
	Body string
}

type Set struct {
	Global  Layer
	PerLoop Layer
	Merged  string
	Version string
}

func Load(globalPath, perLoopPath, lane string) Set {
	lane = sanitizeLane(lane)
	g := readLayer("global", globalPath)
	l := readLayer("loop:"+lane, perLoopPath)
	merged := render(g, l, lane)
	set := Set{Global: g, PerLoop: l, Merged: merged}
	if merged != "" {
		sum := sha256.Sum256([]byte(merged))
		set.Version = hex.EncodeToString(sum[:])
	}
	return set
}

func Resolve(homeDir, lane string) (globalPath, perLoopPath string) {
	base := filepath.Join(homeDir, ".claude", "evolve")
	return filepath.Join(base, "directives.md"), filepath.Join(base, "loops", lane+".md")
}

func sanitizeLane(lane string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(lane)
}

func readLayer(name, path string) Layer {
	layer := Layer{Name: name, Path: path}
	if path == "" {
		return layer
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return layer
	}
	layer.Body = strings.TrimSpace(string(b))
	return layer
}

func render(global, perLoop Layer, lane string) string {
	if global.Body == "" && perLoop.Body == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Operator Directives\n\n")
	b.WriteString(safetyPreamble)
	b.WriteString("\n")
	if global.Body != "" {
		b.WriteString("\n### Global (all loops)\n\n")
		b.WriteString(global.Body)
		b.WriteString("\n")
	}
	if perLoop.Body != "" {
		b.WriteString("\n### This loop (" + lane + ")\n\n")
		b.WriteString(perLoop.Body)
		b.WriteString("\n")
	}
	return b.String()
}
