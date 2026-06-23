package routingtest

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolveloop/go/internal/router"
)

// scoutPresent etc. decide which roles the fixture "produced". A role is present
// when any of its fields is set; this keeps the dual renderings (Signals /
// HandoffFiles) in lock-step.
func (s SignalSpec) scoutPresent() bool {
	return s.CycleSize != "" || s.ScoutItemCount > 0 || s.ScoutCarryover > 0 || s.ScoutBacklog > 0
}
func (s SignalSpec) triagePresent() bool { return s.TriageSize != "" }
func (s SignalSpec) buildPresent() bool {
	return s.BuildVerdict != "" || s.ACSRed > 0 || s.ACSGreen > 0 || s.ACSRegression > 0 ||
		s.SeverityMax != "" || s.FilesTouched > 0 || s.DiffLOC > 0
}
func (s SignalSpec) auditPresent() bool {
	return s.AuditVerdict != "" || s.AuditConf > 0 || s.AuditRedCount > 0
}

// Signals renders the fixture as the pure router.RoutingSignals digest.
func (s SignalSpec) Signals() router.RoutingSignals {
	var sig router.RoutingSignals
	if s.scoutPresent() {
		sig.Scout = router.ScoutSignals{
			CycleSizeEstimate: s.CycleSize,
			ItemCount:         s.ScoutItemCount,
			CarryoverCount:    s.ScoutCarryover,
			BacklogSize:       s.ScoutBacklog,
			Present:           true,
		}
	}
	if s.triagePresent() {
		sig.Triage = router.TriageSignals{CycleSize: s.TriageSize, Present: true}
	}
	if s.buildPresent() {
		sig.Build = router.BuildSignals{
			Verdict:       s.BuildVerdict,
			ACSGreen:      s.ACSGreen,
			ACSRed:        s.ACSRed,
			ACSRegression: s.ACSRegression,
			SeverityMax:   router.ParseSeverity(s.SeverityMax),
			FilesTouched:  s.FilesTouched,
			DiffLOC:       s.DiffLOC,
			Present:       true,
		}
	}
	if s.auditPresent() {
		sig.Audit = router.AuditSignals{
			Verdict:           s.AuditVerdict,
			Confidence:        s.AuditConf,
			RedCount:          s.AuditRedCount,
			DefectsBySeverity: map[router.Severity]int{},
			Present:           true,
		}
	}
	return sig
}

// HandoffFiles renders the fixture as on-disk handoff JSON, matching the shapes
// router.Digest extracts. Only present roles emit a file (mirrors fail-open).
func (s SignalSpec) HandoffFiles() map[string]string {
	out := map[string]string{}
	if s.scoutPresent() {
		m := map[string]interface{}{}
		if s.CycleSize != "" {
			m["cycle_size_estimate"] = s.CycleSize
		}
		if s.ScoutCarryover > 0 {
			m["carryover_count"] = s.ScoutCarryover
		}
		if s.ScoutBacklog > 0 {
			m["backlog_size"] = s.ScoutBacklog
		}
		// Digest counts keys "item<digit>..." for ItemCount.
		for i := 1; i <= s.ScoutItemCount; i++ {
			m[fmt.Sprintf("item%d_scope", i)] = "x"
		}
		out["handoff-scout.json"] = mustJSON(m)
	}
	if s.triagePresent() {
		out["handoff-triage.json"] = mustJSON(map[string]interface{}{"cycle_size": s.TriageSize})
	}
	if s.buildPresent() {
		acs := map[string]interface{}{
			"green": s.ACSGreen, "red": s.ACSRed, "regression": s.ACSRegression,
		}
		m := map[string]interface{}{"verdict": s.BuildVerdict, "acs_result": acs}
		if s.DiffLOC > 0 {
			m["diff_loc"] = s.DiffLOC
		}
		// One thrust carries SeverityMax + FilesTouched (digest takes max severity
		// and len(union(files))).
		if s.SeverityMax != "" || s.FilesTouched > 0 {
			files := make([]string, 0, s.FilesTouched)
			for i := 0; i < s.FilesTouched; i++ {
				files = append(files, fmt.Sprintf("f%d.go", i))
			}
			m["thrusts"] = []map[string]interface{}{
				{"severity": s.SeverityMax, "files_modified": files},
			}
		}
		out["handoff-build.json"] = mustJSON(m)
	}
	if s.auditPresent() {
		out["handoff-audit.json"] = mustJSON(map[string]interface{}{
			"verdict": s.AuditVerdict, "confidence": s.AuditConf, "red_count": s.AuditRedCount,
		})
	}
	return out
}

// WrappedHandoffFiles renders the fixture as the canonical ADR-0050 Phase-3
// payload-wrapped envelope (schema_version 2): each flat handoff JSON from
// HandoffFiles becomes the `payload` of an outer wrapper. router.Digest must
// yield identical RoutingSignals from these as from the flat HandoffFiles — the
// wrapped/flat equivalence invariant that lets the unified envelope replace the
// flat one without changing any routing decision.
func (s SignalSpec) WrappedHandoffFiles() map[string]string {
	flat := s.HandoffFiles()
	out := make(map[string]string, len(flat))
	for name, body := range flat {
		phase := strings.TrimSuffix(strings.TrimPrefix(name, "handoff-"), ".json")
		out[name] = mustJSON(map[string]interface{}{
			"schema_version": 2,
			"phase":          phase,
			"payload":        json.RawMessage(body),
			"signals":        map[string]interface{}{},
		})
	}
	return out
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}
