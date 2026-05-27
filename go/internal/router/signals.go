// Package router is the deterministic phase-routing kernel for evolve-loop.
// It digests the objective signals each phase already writes to its handoff
// artifact, then computes which optional phases to insert/skip — under the
// "model proposes, kernel disposes" discipline.
//
// Leaf package by design: like internal/failureadapter and internal/config,
// it must NOT import internal/core (core.Orchestrator imports router). Phase
// identifiers cross the boundary as plain strings; core converts at the call site.
package router

import "strings"

// Severity is an ordinal encoding of a defect/thrust severity so the router
// can compare with >= against a configured threshold (e.g. insert tester when
// SeverityMax >= High).
type Severity int

const (
	SevNone Severity = iota
	SevLow
	SevMedium
	SevHigh
	SevCritical
)

// ParseSeverity maps a handoff severity string to its ordinal. Unknown/empty
// strings map to SevNone (fail-low: an unparseable severity never escalates routing).
func ParseSeverity(s string) Severity {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CRITICAL":
		return SevCritical
	case "HIGH":
		return SevHigh
	case "MEDIUM", "MED":
		return SevMedium
	case "LOW":
		return SevLow
	default:
		return SevNone
	}
}

func (s Severity) String() string {
	switch s {
	case SevCritical:
		return "CRITICAL"
	case SevHigh:
		return "HIGH"
	case SevMedium:
		return "MEDIUM"
	case SevLow:
		return "LOW"
	default:
		return "NONE"
	}
}

// RoutingSignals is the normalized, objective digest of the handoff artifacts
// seen so far this cycle. ONLY objective fields the phases emit — never an LLM
// self-assessment of confidence (anti-spec-gaming). Populated by Digest.
type RoutingSignals struct {
	Scout  ScoutSignals
	Triage TriageSignals
	Build  BuildSignals
	Audit  AuditSignals

	// Generic is the uniform signal plane: namespaced <phase>.<key> values
	// folded from each handoff's top-level "signals" block. This is what lets a
	// user-defined phase emit a signal the router can key on without a bespoke
	// typed extractor. Populated by Digest; consumed by resolveField as a
	// fallback for fields not covered by the typed structs above (Stage 2).
	Generic map[string]any
}

// GenericValue returns the namespaced generic signal for field (e.g.
// "security.severity_max"); ok is false when absent. Type note: values come
// from encoding/json, so JSON numbers are float64 (not int) — numeric callers
// must assert float64.
func (s RoutingSignals) GenericValue(field string) (any, bool) {
	v, ok := s.Generic[field]
	return v, ok
}

// ScoutSignals are the routing-relevant fields of handoff-scout.json.
type ScoutSignals struct {
	CycleSizeEstimate string // "trivial|small|medium|large"
	ItemCount         int    // # of itemN_* blocks (scope breadth)
	CarryoverCount    int    // carryover todos surfaced
	BacklogSize       int    // total queued backlog items (breadth of pending work)
	Present           bool
}

// TriageSignals are the routing-relevant fields of triage's handoff.
type TriageSignals struct {
	CycleSize string   // authoritative size after triage refines scout's estimate
	PhaseSkip []string // PSMAS phase_skip[] recommendation (additive only)
	Present   bool
}

// BuildSignals are the routing-relevant fields of handoff-build(er).json.
type BuildSignals struct {
	Verdict       string
	ACSGreen      int
	ACSRed        int // failing-predicate count — objective regression signal
	ACSTotal      int
	ACSThisCycle  int
	ACSRegression int
	SeverityMax   Severity // max thrusts[].severity, ordinal-encoded
	FilesTouched  int      // union(thrusts[].files_modified + files_new)
	DiffLOC       int      // lines-of-code changed this build (top-level diff_loc)
	Present       bool
}

// AuditSignals are the routing-relevant fields of handoff-audit(or).json.
type AuditSignals struct {
	Verdict           string
	Confidence        float64
	RedCount          int
	DefectsBySeverity map[Severity]int
	Present           bool
}

// CycleSize returns the authoritative cycle-size: triage's refinement when
// present, else scout's estimate, else "" (treated as non-trivial by callers).
func (s RoutingSignals) CycleSize() string {
	if s.Triage.Present && s.Triage.CycleSize != "" {
		return s.Triage.CycleSize
	}
	if s.Scout.Present {
		return s.Scout.CycleSizeEstimate
	}
	return ""
}
