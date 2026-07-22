package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/changedpkgs"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Digest is the observe→digest boundary: the ONLY code that knows the on-disk
// handoff JSON shapes. It reads the handoff artifact of each completed phase
// from workspace and folds the objective fields into RoutingSignals.
//
// Fail-open contract: a missing or unparseable artifact yields Present:false
// and zero-value signals — a corrupt handoff must never FORCE an optional phase.
// A role's signals are marked Present only when its phase is in `completed` AND
// a real artifact exists on disk (artifact-backed, so the kernel's spine gate
// cannot be satisfied by a fabricated completed-list alone).
//
// Naming tolerance: build/builder and audit/auditor handoff filenames coexist
// across cycle ages; Digest tries each candidate in order.
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
	// ADR-0039 §7: lift each completed phase's structured failure context
	// (report-sentinel v2 failure block) onto the generic plane — this is
	// what lets failure-phase insertion be DATA-driven via insert_when.
	for _, phase := range completed {
		sig.foldFailureSentinel(workspace, phase)
	}
	return sig, nil
}

// unwrapPayload returns the inner `payload` object bytes of the canonical
// ADR-0050 Phase-3 envelope (schema_version 2: a wrapper carrying the exact
// per-phase payload bytes plus promoted top-level verdict/signals/failure), or
// the input unchanged when there is no payload wrapper — the Postel-compatible
// flat fallback. This keeps Digest reading byte-identically whether a handoff is
// written flat (legacy/today) or payload-wrapped (the unified envelope), which
// is the golden-equivalence invariant the shadow stage relies on.
//
// AUTHORITY CONTRACT: the inner payload is the single source of truth. Digest
// (and foldGeneric) read the UNWRAPPED payload, so the wrapper's promoted
// top-level signals/verdict/failure are defined to be a COPY of the payload's,
// never an independent source — a wrapper that carried signals absent from its
// payload would have them ignored. The envelope writer (Phase 3.4+) must uphold
// this; TestDigest_PayloadWrapped_FoldsInnerSignals pins the read side.
func unwrapPayload(raw []byte) []byte {
	var env struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err == nil && len(env.Payload) > 0 {
		return env.Payload
	}
	return raw
}

// foldFailureSentinel surfaces a phase's self-reported failure as
// <phase>.failure_class + <phase>.defect_count generic signals, via the ONE
// shared reader (phasecontract.ReadFailureBlock). Fail-open: no report/block
// is a no-op — crash-class failures are the supervisor's to synthesize, not
// the router's.
func (s *RoutingSignals) foldFailureSentinel(workspace, phase string) {
	fb, ok := phasecontract.ReadFailureBlock(workspace, phase)
	if !ok {
		return
	}
	if s.Generic == nil {
		s.Generic = make(map[string]any, 2)
	}
	// float64 matches the generic plane's JSON-number convention
	// (GenericValue doc: numeric callers assert float64).
	s.Generic[phase+".failure_class"] = fb.Class
	s.Generic[phase+".defect_count"] = float64(len(fb.Defects))
}

// foldGeneric merges a handoff's uniform top-level "signals" object into
// sig.Generic, namespacing bare keys with the phase (keys already containing a
// "." are taken as-is, letting a phase emit a cross-namespace signal). This is
// the uniform signal plane that makes user-phase signals routable without a
// bespoke extractor. Absent/unparseable "signals" is a no-op (fail-open).
// Collisions are last-write-wins (Digest folds in phase order); built-ins never
// collide since they don't use the dotted-key form.
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

// readFirstTracked is the sole file-reader for the ANCHOR handoffs: a read failure that
// is NOT a clean absence (EISDIR, permission, transient IO) is appended to
// degraded — the R5 read-miss vs genuine-gap distinction the spine gate keys
// on. Absence stays silent (Present:false is the signal).
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

// buildFromGitFallback derives BuildSignals from git when neither
// handoff-build.json nor handoff-builder.json is present on disk — both have
// been extinct since ~cycle 215 (warnship_apicover_ci_gap, 3rd recurrence),
// which otherwise leaves sig.Build silently zero-value on every real cycle.
// It reuses changedpkgs.FromGitChecked — the same helper
// internal/phases/audit.changedPackagesForAudit already uses — rather than
// re-implementing git-diff logic. A git-underivable tree (no repo, git
// failure) degrades LOUDLY via DigestDegraded instead of silently returning
// Present:false with no trace, mirroring the read-miss vs genuine-gap
// distinction readFirstTracked already applies to handoff read errors.
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
	// pkgs are deduped package patterns, not raw file paths (FromGitChecked's
	// contract), so FilesTouched here is a package-count floor rather than the
	// handoff's exact file tally — still non-zero on any real change, which is
	// what closes the silent-gap hole this fallback exists for.
	return BuildSignals{Present: true, FilesTouched: len(pkgs)}
}

// scoutFromReportFallback derives ScoutSignals from the artifact scout
// actually writes every cycle — scout-report.md — when handoff-scout.json is
// absent (handoffs have been extinct since ~cycle 215; buildFromGitFallback
// closed the same gap for build). Presence is the only signal the spine floor
// gates on, so the richer handoff fields stay zero — the router's digest
// degrades in richness, never in floor truth. A missing report is a CLEAN
// absence (Present:false, no degrade entry — the enforce gate's fail-closed
// signal); a report that exists but cannot be read is a read-miss and degrades
// LOUDLY (R5), matching readFirstTracked's distinction.
func scoutFromReportFallback(workspace string, degraded *[]string) ScoutSignals {
	info, err := os.Stat(filepath.Join(workspace, "scout-report.md"))
	switch {
	case err == nil && info.Size() > 0:
		return ScoutSignals{Present: true}
	case err == nil:
		return ScoutSignals{} // empty report: the phase did not really deliver
	case os.IsNotExist(err):
		return ScoutSignals{} // clean absence
	default:
		*degraded = append(*degraded, "scout: report fallback stat: "+err.Error())
		return ScoutSignals{}
	}
}

// triageFromReportFallback derives TriageSignals from triage-report.md — the
// artifact triage actually writes every cycle — when handoff-triage.json is
// absent (handoffs extinct since ~cycle 215; same gap scoutFromReportFallback
// closed for scout). Richer than scout's: it extracts the report header's
// `cycle_size_estimate: <size>` line so RoutingSignals.CycleSize() carries a
// real value on the live path (ADR-0076 slice A's budget signal). No size
// vocabulary validation here — the multiplier lookup treats unknown sizes as
// 1.0, so tolerance is safe and single-sourced at the consumer. Absence and
// read-miss semantics mirror scoutFromReportFallback exactly.
func triageFromReportFallback(workspace string, degraded *[]string) TriageSignals {
	raw, err := os.ReadFile(filepath.Join(workspace, "triage-report.md"))
	switch {
	case err == nil && len(raw) > 0:
		return TriageSignals{Present: true, CycleSize: reportHeaderValue(string(raw), "cycle_size_estimate:")}
	case err == nil:
		return TriageSignals{} // empty report: the phase did not really deliver
	case os.IsNotExist(err):
		return TriageSignals{} // clean absence
	default:
		*degraded = append(*degraded, "triage: report fallback read: "+err.Error())
		return TriageSignals{}
	}
}

// reportHeaderValue returns the trimmed value of the first report line starting
// with prefix (e.g. "cycle_size_estimate:"), or "" when no line matches.
func reportHeaderValue(md, prefix string) string {
	for _, line := range strings.Split(md, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), prefix); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// auditFromACSVerdictFallback derives AuditSignals from acs-verdict.json — the
// DETERMINISTIC verdict artifact generateACSVerdict writes every audited cycle
// — when handoff-audit.json is absent. Its top-level verdict/red_count keys
// are exactly what extractAudit reads, so the extraction is shared, and a FAIL
// verdict flows through honestly (the spine's audit anchor then refuses ship —
// that is the floor working, not a gap). Corrupt-but-present degrades LOUDLY
// (fail-open at enforce); genuinely absent stays a clean absence.
func auditFromACSVerdictFallback(workspace string, degraded *[]string) AuditSignals {
	raw, err := os.ReadFile(filepath.Join(workspace, "acs-verdict.json"))
	if err != nil {
		if !os.IsNotExist(err) {
			*degraded = append(*degraded, "audit: acs-verdict fallback read: "+err.Error())
		}
		return AuditSignals{} // clean absence (or loudly-degraded read-miss)
	}
	a := extractAudit(raw)
	if !a.Present {
		*degraded = append(*degraded, "audit: acs-verdict.json present but unparseable (fallback)")
		return a
	}
	if a.Verdict == "" {
		// Parses but carries no verdict — a legacy/schema-drifted artifact
		// (the 2c0559a5 e2e red: Present:true + Verdict:"" is an UNSATISFIABLE
		// audit anchor that read as a clean absence and hard-blocked at
		// enforce). Schema drift is a DEGRADED read, never a clean gap.
		*degraded = append(*degraded, "audit: acs-verdict.json has no verdict field (schema drift?) — degraded, not clean")
		return AuditSignals{}
	}
	return a
}

// projectRootFromWorkspace inverts core.RunWorkspacePath's
// <root>/.evolve/runs/cycle-<N> layout without importing internal/core (router
// is a leaf package by design — see this file's package doc).
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
	_ = json.Unmarshal(top["carryover_count"], &s.CarryoverCount)
	_ = json.Unmarshal(top["backlog_size"], &s.BacklogSize)
	for k := range top {
		// itemN_* blocks measure scope breadth (cycle-56: item1_..item6_).
		if strings.HasPrefix(k, "item") && hasDigitAfterPrefix(k, "item") {
			s.ItemCount++
		}
	}
	return s
}

// hasDigitAfterPrefix reports whether the rune right after prefix is a digit,
// so "item3_foo" counts but "items" or "itemize" does not.
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
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		return TriageSignals{}
	}
	size := d.CycleSize
	if size == "" {
		size = d.CycleSizeEst
	}
	return TriageSignals{CycleSize: size, PhaseSkip: d.PhaseSkip, Present: true}
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
