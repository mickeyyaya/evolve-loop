// Package router is the deterministic phase-routing kernel: it digests what phases write and decides
// which phases run ("model proposes, kernel disposes"). It is a leaf: core imports it, so it must
// never import core.
package router

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
)

type Severity int

const (
	SevNone Severity = iota
	SevLow
	SevMedium
	SevHigh
	SevCritical
)

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

type RoutingSignals struct {
	Scout  ScoutSignals
	Triage TriageSignals
	Build  BuildSignals
	Audit  AuditSignals

	Generic map[string]any

	DigestDegraded []string
}

func (s RoutingSignals) GenericValue(field string) (any, bool) {
	v, ok := s.Generic[field]
	return v, ok
}

type ScoutSignals struct {
	CycleSizeEstimate string
	GoalType          string
	DeliverableKind   string
	ItemCount         int
	CarryoverCount    int
	BacklogSize       int
	ArtifactBytes     int
	Present           bool
}

type TriageSignals struct {
	CycleSize          string
	PhaseSkip          []string
	DeliverableKind    string
	CommittedCount     int
	UnifiedSize        string
	UnifiedMemberCount int
	Present            bool
	commitmentKnown    bool
}

func (s RoutingSignals) HasEmptyTriageCommitment() bool {
	return s.Triage.Present && s.Triage.commitmentKnown && s.Triage.CommittedCount == 0
}

type BuildSignals struct {
	Verdict       string
	ACSGreen      int
	ACSRed        int
	ACSTotal      int
	ACSThisCycle  int
	ACSRegression int
	SeverityMax   Severity
	FilesTouched  int
	DiffLOC       int
	Present       bool
}

type AuditSignals struct {
	Verdict           string
	Confidence        float64
	RedCount          int
	DefectsBySeverity map[Severity]int
	Present           bool
}

func (s RoutingSignals) CycleSize() string {
	if s.Triage.Present && s.Triage.CycleSize != "" {
		return s.Triage.CycleSize
	}
	if s.Scout.Present {
		return s.Scout.CycleSizeEstimate
	}
	return ""
}

const (
	DeliverableKindCode     = config.DeliverableKindCode
	DeliverableKindDocument = config.DeliverableKindDocument
)

func NormalizeDeliverableKind(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case DeliverableKindCode:
		return DeliverableKindCode
	case DeliverableKindDocument:
		return DeliverableKindDocument
	}
	return ""
}

func (s RoutingSignals) DeliverableKind() string {
	if k, ok := s.DeclaredDeliverableKind(); ok {
		return k
	}
	return DeliverableKindCode
}

func (s RoutingSignals) DeclaredDeliverableKind() (string, bool) {
	if s.Triage.Present && s.Triage.DeliverableKind != "" {
		return s.Triage.DeliverableKind, true
	}
	if s.Scout.Present && s.Scout.DeliverableKind != "" {
		return s.Scout.DeliverableKind, true
	}
	return "", false
}
