// Package router is the deterministic phase-routing kernel: it digests what phases write and decides
// which phases run ("model proposes, kernel disposes"). It is a leaf: core imports it, so it must
// never import core.
package router

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

// Severity is an ordinal defect severity, so triggers can compare with >=.
type Severity int

// Severity levels, lowest first.
const (
	SevNone Severity = iota
	SevLow
	SevMedium
	SevHigh
	SevCritical
)

// ParseSeverity maps a severity word to its ordinal; an unknown word is SevNone, so it never escalates routing.
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

// String returns the canonical severity word.
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

// RoutingSignals is the objective digest of this cycle's phase artifacts; it never carries an LLM's
// self-assessed confidence.
type RoutingSignals struct {
	Scout  ScoutSignals
	Triage TriageSignals
	Build  BuildSignals
	Audit  AuditSignals

	// Generic holds namespaced <phase>.<key> signals for fields the typed structs do not cover.
	Generic map[string]any

	// DigestDegraded lists reads that failed for a reason other than absence. While it is
	// non-empty a Present:false may be a read miss, so the spine gate stays fail-open.
	DigestDegraded []string
}

// GenericValue returns the generic signal for field; JSON numbers arrive as float64.
func (s RoutingSignals) GenericValue(field string) (any, bool) {
	v, ok := s.Generic[field]
	return v, ok
}

// ScoutSignals are the routing-relevant fields of scout's handoff or report.
type ScoutSignals struct {
	CycleSizeEstimate string // "trivial|small|medium|large"
	GoalType          string // a phase-registry goal_recipes key; "" = undeclared
	DeliverableKind   string // "code|document"; "" = undeclared
	ItemCount         int    // itemN_* blocks: scope breadth
	CarryoverCount    int
	BacklogSize       int
	Present           bool
}

// TriageSignals are the routing-relevant fields of triage's handoff, report and decision.
type TriageSignals struct {
	CycleSize          string   // refines scout's estimate
	PhaseSkip          []string // PSMAS recommendation, additive only
	DeliverableKind    string   // authoritative over scout's; "" = undeclared
	CommittedCount     int      // tasks in triage-decision.json top_n
	UnifiedSize        string   // "small|large", set only for a validated unified commitment
	UnifiedMemberCount int
	Present            bool
	commitmentKnown    bool // separates an explicit empty top_n from a missing decision
}

// HasEmptyTriageCommitment reports whether triage authoritatively committed no
// tasks. A missing or unreadable decision is unknown and therefore not empty.
func (s RoutingSignals) HasEmptyTriageCommitment() bool {
	return s.Triage.Present && s.Triage.commitmentKnown && s.Triage.CommittedCount == 0
}

// BuildSignals are the routing-relevant fields of build's handoff.
type BuildSignals struct {
	Verdict       string
	ACSGreen      int
	ACSRed        int // failing predicates
	ACSTotal      int
	ACSThisCycle  int
	ACSRegression int
	SeverityMax   Severity // highest thrusts[].severity
	FilesTouched  int      // distinct files across thrusts; a package count from the git fallback
	DiffLOC       int
	Present       bool
}

// AuditSignals are the routing-relevant fields of audit's handoff or acs-verdict.json.
type AuditSignals struct {
	Verdict           string
	Confidence        float64
	RedCount          int
	DefectsBySeverity map[Severity]int
	Present           bool
}

// CycleSize returns triage's size, else scout's estimate, else "" (callers treat it as non-trivial).
func (s RoutingSignals) CycleSize() string {
	if s.Triage.Present && s.Triage.CycleSize != "" {
		return s.Triage.CycleSize
	}
	if s.Scout.Present {
		return s.Scout.CycleSizeEstimate
	}
	return ""
}

// DeliverableKindCode and DeliverableKindDocument are the two deliverable kinds a cycle can declare.
// See ADR-0099.
const (
	DeliverableKindCode     = config.DeliverableKindCode
	DeliverableKindDocument = config.DeliverableKindDocument
)

// NormalizeDeliverableKind returns v's kind, or "" for any other word so it can never release a pinned phase.
func NormalizeDeliverableKind(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case DeliverableKindCode:
		return DeliverableKindCode
	case DeliverableKindDocument:
		return DeliverableKindDocument
	}
	return ""
}

// DeliverableKind returns the declared kind, else "code": the conservative default that keeps the tdd pin.
func (s RoutingSignals) DeliverableKind() string {
	if k, ok := s.DeclaredDeliverableKind(); ok {
		return k
	}
	return DeliverableKindCode
}

// DeclaredDeliverableKind returns the kind a report declared (triage over scout) and whether one did.
func (s RoutingSignals) DeclaredDeliverableKind() (string, bool) {
	if s.Triage.Present && s.Triage.DeliverableKind != "" {
		return s.Triage.DeliverableKind, true
	}
	if s.Scout.Present && s.Scout.DeliverableKind != "" {
		return s.Scout.DeliverableKind, true
	}
	return "", false
}
