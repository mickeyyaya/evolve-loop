// Package directives implements the runtime operator-directives cascade: a
// global layer (all loops) and a per-loop layer (keyed by runscope lane) that a
// loop re-reads at the START of each cycle and injects into every phase agent's
// prompt. The main session edits the files; all loops converge on the next cycle
// boundary without restart. See
// docs/superpowers/specs/2026-06-20-runtime-directives-cascade-design.md.
//
// This package is intentionally PURE and environment-agnostic: Load takes
// explicit paths and never reads os.Getenv/LookupEnv/Environ (the composition
// root resolves home dir + lane via Resolve and passes paths in). Directives are
// guidance prose only — they cannot weaken the binary gates, which remain the
// sole hard trust boundary.
package directives

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// safetyPreamble heads the rendered block so agents weigh directives correctly:
// global directives are authoritative for safety, and guidance never overrides
// the binary gates.
const safetyPreamble = "_Runtime operator guidance, snapshotted at cycle start. " +
	"Global directives are authoritative for safety and may only be tightened, never weakened. " +
	"Binary gates (EGPS, ship-gate, policy floor) remain the only hard trust boundary — guidance does not override a gate._"

// Layer is one resolved directives source. Body == "" means the layer is absent
// (the file was missing, unreadable, or empty) — a fail-open, never-blocking
// outcome.
type Layer struct {
	Name string // "global" or "loop:<lane>"
	Path string // the path it was resolved from (for diagnostics)
	Body string // trimmed file contents; "" = absent
}

// Set is the merged, cycle-snapshotted directives a loop injects into agents.
type Set struct {
	Global  Layer
	PerLoop Layer
	Merged  string // rendered "## Operator Directives" block; "" = nothing to inject
	Version string // sha256 hex of Merged (stamped to the ledger); "" when Merged == ""
}

// Load reads the two explicit paths, renders the merged block, and computes the
// version. It is pure and environment-agnostic: it performs only file reads and
// never consults the system environment. A missing/unreadable/empty file yields
// an absent Layer (fail-open) — Load never blocks a cycle. lane labels the
// per-loop section and is supplied by the caller (not read from env).
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

// Resolve derives the directives file paths from an explicit home directory and
// runscope lane. The composition root supplies homeDir (os.UserHomeDir) and lane
// (runscope); Resolve reads no environment so it stays unit-testable.
func Resolve(homeDir, lane string) (globalPath, perLoopPath string) {
	base := filepath.Join(homeDir, ".claude", "evolve")
	return filepath.Join(base, "directives.md"), filepath.Join(base, "loops", lane+".md")
}

// sanitizeLane strips line breaks from a caller-supplied lane so it cannot inject
// extra lines or fake headings when interpolated into the rendered markdown block
// (keeps the injected directives' structure well-formed even if the lane is odd).
func sanitizeLane(lane string) string {
	return strings.NewReplacer("\r", "", "\n", "").Replace(lane)
}

// readLayer reads one layer, failing open to an absent Layer on any error or an
// empty path/file.
func readLayer(name, path string) Layer {
	layer := Layer{Name: name, Path: path}
	if path == "" {
		return layer
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return layer // fail-open: treat unreadable as absent
	}
	layer.Body = strings.TrimSpace(string(b))
	return layer
}

// render builds the injected block from the present layers. Both absent → "".
// Precedence is expressed by labeled concatenation (global first, then per-loop)
// plus the safety preamble — global safety directives are authoritative.
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
