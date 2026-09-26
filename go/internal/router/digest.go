package router

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Digest folds the artifacts of each completed phase in workspace into RoutingSignals; it is the only
// reader of on-disk handoff shapes. A role is Present only when its phase completed and an artifact
// exists, and a missing or corrupt artifact fails open to Present:false.
func Digest(workspace string, completed []string) (RoutingSignals, error) {
	var sig RoutingSignals
	done := toSet(completed)

	if done["scout"] {
		if raw, ok := readFirstTracked(workspace, &sig.DigestDegraded, "handoff-scout.json"); ok {
			raw = unwrapPayload(raw)
			sig.Scout = extractScout(raw)
			sig.foldGeneric("scout", raw)
		} else {
			sig.Scout = scoutFromReportFallback(workspace, &sig.DigestDegraded)
		}
	}
	if done["triage"] {
		if raw, ok := readFirstTracked(workspace, &sig.DigestDegraded, "handoff-triage.json"); ok {
			raw = unwrapPayload(raw)
			sig.Triage = extractTriage(raw)
			sig.foldGeneric("triage", raw)
		} else {
			sig.Triage = triageFromReportFallback(workspace, &sig.DigestDegraded)
		}
		if decision, ok := digestTriageDecision(workspace, &sig.DigestDegraded); ok {
			sig.Triage.CommittedCount = decision.committedCount
			sig.Triage.commitmentKnown = true
			sig.Triage.UnifiedSize = decision.unifiedSize
			sig.Triage.UnifiedMemberCount = decision.unifiedMemberCount
		}
	}
	if done["build"] {
		if raw, ok := readFirstTracked(workspace, &sig.DigestDegraded, "handoff-build.json", "handoff-builder.json"); ok {
			raw = unwrapPayload(raw)
			sig.Build = extractBuild(raw)
			sig.foldGeneric("build", raw)
		} else {
			sig.Build = buildFromGitFallback(workspace, &sig.DigestDegraded)
		}
	}
	if done["audit"] {
		if raw, ok := readFirstTracked(workspace, &sig.DigestDegraded, "handoff-audit.json", "handoff-auditor.json"); ok {
			raw = unwrapPayload(raw)
			sig.Audit = extractAudit(raw)
			sig.foldGeneric("audit", raw)
		} else {
			sig.Audit = auditFromACSVerdictFallback(workspace, &sig.DigestDegraded)
		}
	}
	for _, phase := range completed {
		sig.foldFailureSentinel(workspace, phase)
	}
	return sig, nil
}

// triageDecisionDigest is what routing reads from triage-decision.json.
type triageDecisionDigest struct {
	committedCount     int
	unifiedSize        string
	unifiedMemberCount int
}

func digestTriageDecision(workspace string, degraded *[]string) (triageDecisionDigest, bool) {
	raw, err := os.ReadFile(filepath.Join(workspace, "triage-decision.json"))
	if os.IsNotExist(err) {
		return triageDecisionDigest{}, false
	}
	if err != nil {
		*degraded = append(*degraded, "triage: decision read: "+err.Error())
		return triageDecisionDigest{}, false
	}
	var decision map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decision); err != nil {
		*degraded = append(*degraded, "triage: decision parse: "+err.Error())
		return triageDecisionDigest{}, false
	}
	topN, ok := decision["top_n"]
	if !ok {
		return triageDecisionDigest{}, false
	}
	topN = bytes.TrimSpace(topN)
	if len(topN) == 0 || topN[0] != '[' {
		*degraded = append(*degraded, "triage: decision top_n must be an array")
		return triageDecisionDigest{}, false
	}
	var tasks []json.RawMessage
	if err := json.Unmarshal(topN, &tasks); err != nil {
		*degraded = append(*degraded, "triage: decision top_n parse: "+err.Error())
		return triageDecisionDigest{}, false
	}
	result := triageDecisionDigest{committedCount: len(tasks)}
	if projection, ok := decision["unified_projection"]; ok {
		var value struct {
			Size        string `json:"size"`
			MemberCount int    `json:"member_count"`
		}
		if err := json.Unmarshal(projection, &value); err != nil || (value.Size != "small" && value.Size != "large") || value.MemberCount < 2 {
			*degraded = append(*degraded, "triage: invalid unified projection")
		} else {
			result.unifiedSize = value.Size
			result.unifiedMemberCount = value.MemberCount
		}
	}
	return result, true
}

// unwrapPayload returns the `payload` of a schema-2 handoff envelope, or raw unchanged for a flat handoff.
// The payload is the authority: the envelope's promoted top-level fields must be a copy of it.
func unwrapPayload(raw []byte) []byte {
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Payload) > 0 {
		return env.Payload
	}
	return raw
}

// foldFailureSentinel surfaces a phase's report failure block as <phase>.failure_class and
// <phase>.defect_count; crash-class failures are the supervisor's to synthesize, not the router's.
func (s *RoutingSignals) foldFailureSentinel(workspace, phase string) {
	fb, ok := phasecontract.ReadFailureBlock(workspace, phase)
	if !ok {
		return
	}
	if s.Generic == nil {
		s.Generic = make(map[string]any, 2)
	}
	// float64 matches the generic plane's JSON-number convention.
	s.Generic[phase+".failure_class"] = fb.Class
	s.Generic[phase+".defect_count"] = float64(len(fb.Defects))
}

// foldGeneric merges a handoff's top-level "signals" into s.Generic, prefixing bare keys with the
// phase; a dotted key is kept as-is so a phase can emit a cross-namespace signal. Last write wins.
func (s *RoutingSignals) foldGeneric(phase string, raw []byte) {
	var doc struct {
		Signals map[string]any `json:"signals"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.Signals) == 0 {
		return
	}
	if s.Generic == nil {
		s.Generic = make(map[string]any, len(doc.Signals))
	}
	for k, v := range doc.Signals {
		if !strings.Contains(k, ".") {
			k = phase + "." + k
		}
		s.Generic[k] = v
	}
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

// readFirstTracked reads the first candidate that exists. A failure other than absence is appended
// to degraded, because the spine gate must tell a read miss from a genuine gap.
func readFirstTracked(dir string, degraded *[]string, candidates ...string) ([]byte, bool) {
	for _, name := range candidates {
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err == nil {
			return raw, true
		}
		if !os.IsNotExist(err) {
			*degraded = append(*degraded, name+": "+err.Error())
		}
	}
	return nil, false
}

// buildFromGitFallback derives BuildSignals from the git change set when no build handoff exists;
// an underivable tree degrades loudly.
func buildFromGitFallback(workspace string, degraded *[]string) BuildSignals {
	root, ok := projectRootFromWorkspace(workspace)
	if !ok {
		*degraded = append(*degraded, "build: workspace path not in <root>/.evolve/runs/cycle-<N> form, cannot derive git fallback")
		return BuildSignals{}
	}
	pkgs, derivable := changedpkgs.FromGitChecked(root, "HEAD")
	if !derivable {
		*degraded = append(*degraded, "build: handoff absent and git-derived changed-package set is underivable (no repo / git failure)")
		return BuildSignals{}
	}
	// pkgs are package patterns, not files, so FilesTouched is a package count.
	return BuildSignals{Present: true, FilesTouched: len(pkgs)}
}

// scoutFromReportFallback derives ScoutSignals from scout-report.md when no scout handoff exists.
func scoutFromReportFallback(workspace string, degraded *[]string) ScoutSignals {
	md, present := readReportFallback(filepath.Join(workspace, phasecontract.ArtifactName("scout")), "scout", degraded)
	if !present {
		return ScoutSignals{}
	}
	return ScoutSignals{
		Present:         true,
		GoalType:        reportHeaderValue(md, HeaderGoalType),
		DeliverableKind: NormalizeDeliverableKind(reportHeaderValue(md, HeaderDeliverableKind)),
	}
}

// Report header keys the kernel reads and the scout and triage personas write.
const (
	HeaderGoalType        = "goal_type:"
	HeaderDeliverableKind = "deliverable_kind:"
	HeaderCycleSize       = "cycle_size_estimate:"
)

// readReportFallback returns a non-empty report's body. An empty or absent report is not present
// and not degraded; any other read failure degrades loudly.
func readReportFallback(path, role string, degraded *[]string) (string, bool) {
	raw, err := os.ReadFile(path)
	switch {
	case err == nil && len(raw) > 0:
		return string(raw), true
	case err == nil:
		return "", false // an empty report did not deliver
	case os.IsNotExist(err):
		return "", false
	default:
		*degraded = append(*degraded, role+": report fallback read: "+err.Error())
		return "", false
	}
}

// triageFromReportFallback derives TriageSignals from triage-report.md when no triage handoff exists.
// The size is not validated here: the budget multiplier treats an unknown size as 1.0.
func triageFromReportFallback(workspace string, degraded *[]string) TriageSignals {
	md, present := readReportFallback(filepath.Join(workspace, "triage-report.md"), "triage", degraded)
	if !present {
		return TriageSignals{}
	}
	return TriageSignals{
		Present:         true,
		CycleSize:       reportHeaderValue(md, HeaderCycleSize),
		DeliverableKind: NormalizeDeliverableKind(reportHeaderValue(md, HeaderDeliverableKind)),
	}
}

// reportHeaderValue returns the trimmed value of the first line starting with prefix, or "".
func reportHeaderValue(md, prefix string) string {
	for _, line := range strings.Split(md, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// auditFromACSVerdictFallback derives AuditSignals from acs-verdict.json when no audit handoff exists.
// A FAIL verdict flows through, so the spine's audit anchor refuses ship.
func auditFromACSVerdictFallback(workspace string, degraded *[]string) AuditSignals {
	raw, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json"))
	if err != nil {
		if !os.IsNotExist(err) {
			*degraded = append(*degraded, "audit: acs-verdict fallback read: "+err.Error())
		}
		return AuditSignals{}
	}
	a := extractAudit(raw)
	if !a.Present {
		*degraded = append(*degraded, "audit: acs-verdict.json present but unparseable (fallback)")
		return a
	}
	if a.Verdict == "" {
		// A verdict-less artifact is an unsatisfiable anchor; report it as degraded, not as a clean gap.
		*degraded = append(*degraded, "audit: acs-verdict.json has no verdict field (schema drift?) — degraded, not clean")
		return AuditSignals{}
	}
	return a
}

// projectRootFromWorkspace inverts core.RunWorkspacePath's <root>/.evolve/runs/cycle-<N> layout.
func projectRootFromWorkspace(workspace string) (string, bool) {
	dir := filepath.Clean(workspace)
	if !strings.HasPrefix(filepath.Base(dir), "cycle-") {
		return "", false
	}
	runsDir := filepath.Dir(dir)
	if filepath.Base(runsDir) != "runs" {
		return "", false
	}
	evolveDir := filepath.Dir(runsDir)
	if filepath.Base(evolveDir) != ".evolve" {
		return "", false
	}
	return filepath.Dir(evolveDir), true
}

func extractScout(raw []byte) ScoutSignals {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return ScoutSignals{}
	}
	s := ScoutSignals{Present: true}
	_ = json.Unmarshal(top["cycle_size_estimate"], &s.CycleSizeEstimate)
	_ = json.Unmarshal(top["goal_type"], &s.GoalType)
	var kind string
	_ = json.Unmarshal(top["deliverable_kind"], &kind)
	s.DeliverableKind = NormalizeDeliverableKind(kind)
	_ = json.Unmarshal(top["carryover_count"], &s.CarryoverCount)
	_ = json.Unmarshal(top["backlog_size"], &s.BacklogSize)
	for k := range top {
		// itemN_* blocks measure scope breadth.
		if strings.HasPrefix(k, "item") && hasDigitAfterPrefix(k, "item") {
			s.ItemCount++
		}
	}
	return s
}

// hasDigitAfterPrefix reports whether the byte after prefix is a digit: "item3_foo" yes, "items" no.
func hasDigitAfterPrefix(s, prefix string) bool {
	if len(s) <= len(prefix) {
		return false
	}
	c := s[len(prefix)]
	return c >= '0' && c <= '9'
}

func extractTriage(raw []byte) TriageSignals {
	var d struct {
		CycleSize    string   `json:"cycle_size"`
		CycleSizeEst string   `json:"cycle_size_estimate"`
		PhaseSkip    []string `json:"phase_skip"`
		Kind         string   `json:"deliverable_kind"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return TriageSignals{}
	}
	size := d.CycleSize
	if size == "" {
		size = d.CycleSizeEst
	}
	return TriageSignals{CycleSize: size, PhaseSkip: d.PhaseSkip, DeliverableKind: NormalizeDeliverableKind(d.Kind), Present: true}
}

func extractBuild(raw []byte) BuildSignals {
	var d struct {
		Verdict   string `json:"verdict"`
		DiffLOC   int    `json:"diff_loc"`
		ACSResult struct {
			Green      int `json:"green"`
			Red        int `json:"red"`
			Total      int `json:"total"`
			ThisCycle  int `json:"this_cycle"`
			Regression int `json:"regression"`
		} `json:"acs_result"`
		Thrusts []struct {
			Severity      string   `json:"severity"`
			FilesModified []string `json:"files_modified"`
			FilesNew      []string `json:"files_new"`
		} `json:"thrusts"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return BuildSignals{}
	}
	b := BuildSignals{
		Verdict:       d.Verdict,
		ACSGreen:      d.ACSResult.Green,
		ACSRed:        d.ACSResult.Red,
		ACSTotal:      d.ACSResult.Total,
		ACSThisCycle:  d.ACSResult.ThisCycle,
		ACSRegression: d.ACSResult.Regression,
		DiffLOC:       d.DiffLOC,
		Present:       true,
	}
	files := map[string]bool{}
	for _, th := range d.Thrusts {
		if sev := ParseSeverity(th.Severity); sev > b.SeverityMax {
			b.SeverityMax = sev
		}
		for _, f := range th.FilesModified {
			files[f] = true
		}
		for _, f := range th.FilesNew {
			files[f] = true
		}
	}
	b.FilesTouched = len(files)
	return b
}

func extractAudit(raw []byte) AuditSignals {
	var d struct {
		Verdict    string  `json:"verdict"`
		Confidence float64 `json:"confidence"`
		RedCount   int     `json:"red_count"`
		Defects    []struct {
			Severity string `json:"severity"`
		} `json:"defects"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return AuditSignals{}
	}
	a := AuditSignals{
		Verdict:           d.Verdict,
		Confidence:        d.Confidence,
		RedCount:          d.RedCount,
		DefectsBySeverity: map[Severity]int{},
		Present:           true,
	}
	for _, df := range d.Defects {
		a.DefectsBySeverity[ParseSeverity(df.Severity)]++
	}
	return a
}
