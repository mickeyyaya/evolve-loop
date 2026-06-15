package phaseio

// ScoutView is the typed, routing-relevant projection of a scout handoff. It
// mirrors the objective fields router.ScoutSignals folds from handoff-scout.json
// — held here as a dependency-free leaf type so a phase reads upstream scope via
// Upstream().Scout() instead of re-reading the artifact off disk (P4).
type ScoutView struct {
	CycleSizeEstimate string // "trivial|small|medium|large"
	ItemCount         int    // # of itemN_* scope blocks
	CarryoverCount    int    // carryover todos surfaced
	BacklogSize       int    // total queued backlog items
}

// TriageView is the typed projection of a triage handoff.
type TriageView struct {
	CycleSize string   // authoritative size after triage refines scout's estimate
	PhaseSkip []string // PSMAS phase_skip[] recommendation (additive only)
}

// BuildView is the typed projection of a build handoff.
type BuildView struct {
	Verdict       string
	ACSGreen      int
	ACSRed        int // failing-predicate count — objective regression signal
	ACSTotal      int
	ACSThisCycle  int
	ACSRegression int
	SeverityMax   string // max thrusts[].severity as a canonical severity word ("NONE".."CRITICAL")
	FilesTouched  int    // union(thrusts[].files_modified + files_new)
	DiffLOC       int    // lines-of-code changed this build
}

// AuditView is the typed projection of an audit handoff. RedCount is the EGPS
// gate input (3.7 moves the gate's source to Upstream().Audit() without changing
// the gate logic).
type AuditView struct {
	Verdict           string
	Confidence        float64
	RedCount          int
	DefectsBySeverity map[string]int // severity word → count
}

// HandoffsInit is the construction DTO for NewHandoffs. A nil view pointer means
// the phase is absent (the accessor returns ok=false); a present pointer is
// deep-copied into the sealed Handoffs so later mutation of the init cannot leak.
type HandoffsInit struct {
	Scout    *ScoutView
	Triage   *TriageView
	Build    *BuildView
	Audit    *AuditView
	Generic  map[string]any // namespaced <phase>.<key> generic signal plane
	Degraded []string       // anchor-handoff reads that failed for reasons other than absence (R5)
}

// Handoffs is the sealed, read-only typed view of upstream phase outputs piped
// into a phase (the "Upstream" channel of PhaseInput). It replaces ad-hoc disk
// reads: a phase asks for the typed view it needs and distinguishes "absent"
// from "present-but-zero" via the ok return (P5). Build it with NewHandoffs;
// the zero value is a valid empty Handoffs.
type Handoffs struct {
	scout    *ScoutView
	triage   *TriageView
	build    *BuildView
	audit    *AuditView
	generic  map[string]any
	degraded []string
}

// NewHandoffs builds a sealed Handoffs from init, deep-copying every reference
// field so the result is immune to later mutation of the caller's maps/slices.
func NewHandoffs(init HandoffsInit) Handoffs {
	return Handoffs{
		scout:    cloneScout(init.Scout),
		triage:   cloneTriage(init.Triage),
		build:    cloneBuild(init.Build),
		audit:    cloneAudit(init.Audit),
		generic:  cloneAnyMap(init.Generic),
		degraded: cloneStrings(init.Degraded),
	}
}

// Scout returns the scout view and ok=true when a scout handoff was present.
func (h Handoffs) Scout() (ScoutView, bool) {
	if h.scout == nil {
		return ScoutView{}, false
	}
	return *h.scout, true
}

// Triage returns the triage view and ok=true when a triage handoff was present.
func (h Handoffs) Triage() (TriageView, bool) {
	if h.triage == nil {
		return TriageView{}, false
	}
	return *h.triage, true
}

// Build returns the build view and ok=true when a build handoff was present.
func (h Handoffs) Build() (BuildView, bool) {
	if h.build == nil {
		return BuildView{}, false
	}
	return *h.build, true
}

// Audit returns the audit view and ok=true when an audit handoff was present.
func (h Handoffs) Audit() (AuditView, bool) {
	if h.audit == nil {
		return AuditView{}, false
	}
	return *h.audit, true
}

// Generic looks up a namespaced generic signal (e.g. "security.severity_max").
// Values come from encoding/json, so numeric callers must assert float64.
func (h Handoffs) Generic(key string) (any, bool) {
	v, ok := h.generic[key]
	return v, ok
}

// Degraded returns a copy of the read-miss list — anchor handoffs that failed to
// read for reasons other than clean absence. A non-empty list means a downstream
// "absent" signal may be a read miss, so fail-open routing must hold.
func (h Handoffs) Degraded() []string {
	return cloneStrings(h.degraded)
}

func cloneScout(s *ScoutView) *ScoutView {
	if s == nil {
		return nil
	}
	c := *s
	return &c
}

func cloneTriage(t *TriageView) *TriageView {
	if t == nil {
		return nil
	}
	c := *t
	c.PhaseSkip = cloneStrings(t.PhaseSkip)
	return &c
}

func cloneBuild(b *BuildView) *BuildView {
	if b == nil {
		return nil
	}
	c := *b
	return &c
}

func cloneAudit(a *AuditView) *AuditView {
	if a == nil {
		return nil
	}
	c := *a
	c.DefectsBySeverity = cloneIntMap(a.DefectsBySeverity)
	return &c
}

func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = deepCopyAny(v)
	}
	return out
}

// deepCopyAny deep-copies a JSON-shaped value (the form encoding/json produces:
// scalars, map[string]any, []any) so a nested object/array in a generic signal
// cannot be mutated through Handoffs.Generic. Non-container values are returned
// as-is (immutable scalars).
func deepCopyAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return cloneAnyMap(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = deepCopyAny(e)
		}
		return out
	default:
		return v
	}
}

func cloneIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
