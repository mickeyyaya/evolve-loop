package phaseio

type ScoutView struct {
	CycleSizeEstimate string
	GoalType          string
	DeliverableKind   string
	ItemCount         int
	CarryoverCount    int
	BacklogSize       int
}

type TriageView struct {
	CycleSize       string
	PhaseSkip       []string
	DeliverableKind string
}

type ExplanationView struct {
	SchemaVersion   int      `json:"schema_version"`
	ContractVersion int      `json:"contract_version"`
	Status          string   `json:"status"`
	Cycle           int      `json:"cycle"`
	Reason          string   `json:"reason"`
	BaseSHA         string   `json:"base_sha,omitempty"`
	DocumentPath    string   `json:"document_path,omitempty"`
	DocumentSHA256  string   `json:"document_sha256,omitempty"`
	DiffSHA256      string   `json:"diff_sha256"`
	MaterialPaths   []string `json:"material_paths,omitempty"`
	AuthoredBaseSHA string   `json:"authored_base_sha,omitempty"`
}

type BuildView struct {
	Verdict       string
	ACSGreen      int
	ACSRed        int
	ACSTotal      int
	ACSThisCycle  int
	ACSRegression int
	SeverityMax   string
	FilesTouched  int
	DiffLOC       int
}

type AuditView struct {
	Verdict           string
	Confidence        float64
	RedCount          int
	DefectsBySeverity map[string]int
}

type HandoffsInit struct {
	Scout    *ScoutView
	Triage   *TriageView
	Build    *BuildView
	Audit    *AuditView
	Generic  map[string]any
	Degraded []string
}

type Handoffs struct {
	scout    *ScoutView
	triage   *TriageView
	build    *BuildView
	audit    *AuditView
	generic  map[string]any
	degraded []string
}

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

func (h Handoffs) Scout() (ScoutView, bool) {
	if h.scout == nil {
		return ScoutView{}, false
	}
	return *h.scout, true
}

func (h Handoffs) Triage() (TriageView, bool) {
	if h.triage == nil {
		return TriageView{}, false
	}
	return *h.triage, true
}

func (h Handoffs) Build() (BuildView, bool) {
	if h.build == nil {
		return BuildView{}, false
	}
	return *h.build, true
}

func (h Handoffs) Audit() (AuditView, bool) {
	if h.audit == nil {
		return AuditView{}, false
	}
	return *h.audit, true
}

func (h Handoffs) Generic(key string) (any, bool) {
	v, ok := h.generic[key]
	return v, ok
}

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
