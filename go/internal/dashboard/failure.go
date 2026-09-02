package dashboard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Workspace artifact names the failure panel reads. The writers are named so a
// reader can audit the join: failure-decision.json (retro, validated by
// core/failure_decision.go), disposition.json (retro, gated by
// core/disposition_gate.go), audit-fail-reason.json (coherence floor,
// core/system_failure.go), failure-digest.json (core/failure_digest.go). The
// audit report's name and its round archives come from the phase registry
// (phasecontract.ArtifactFilename / RoundArchiveFilename / ParseRoundArchive)
// — the same functions the writer core/audit_round_artifacts.go uses.
const (
	failureDecisionFile = "failure-decision.json"
	dispositionFile     = "disposition.json"
	auditFailReasonFile = "audit-fail-reason.json"
	failureDigestFile   = "failure-digest.json"
)

// auditReportName / buildReportName are the registry-derived report files.
var (
	auditReportName = phasecontract.ArtifactFilename(string(core.PhaseAudit))
	buildReportName = phasecontract.ArtifactFilename(string(core.PhaseBuild))
)

type failureDecision struct {
	Category string `json:"category"`
	Level    string `json:"level"`
	Action   string `json:"action"`
	FixType  string `json:"fix_type"`
}

type disposition struct {
	Fingerprint string `json:"fingerprint"`
	Legitimacy  string `json:"legitimacy"`
	Urgency     string `json:"urgency"`
	RootCause   struct {
		Layer   string `json:"layer"`
		Summary string `json:"summary"`
	} `json:"root_cause"`
	Salvage struct {
		Pointer string `json:"pointer"`
	} `json:"salvage"`
}

type auditFailReason struct {
	Reasons []string `json:"reasons"`
}

type failureDigest struct {
	Fingerprint string `json:"fingerprint"`
	PreClass    string `json:"pre_class"`
}

// readFailure assembles the "what went wrong" panel from the workspace, falling
// back to the committed dossier's failure record when the workspace is gone.
// Returns nil when there is nothing to report (a PASS, or no artifacts at all).
// An unreadable or torn artifact is a warning, never silently "absent".
func readFailure(ws string, d *dossier.Dossier) (*Failure, []string) {
	f := &Failure{}
	var warnings []string
	found := false
	if d != nil && d.Failure != nil {
		f.Fingerprint, f.PreClass, f.GateReasons = d.Failure.Fingerprint, d.Failure.PreClass, d.Failure.Reasons
		found = true
	}
	var fd failureDecision
	if readJSONWarn(filepath.Join(ws, failureDecisionFile), &fd, &warnings) {
		f.Category, f.Level, f.Action, f.FixType = fd.Category, fd.Level, fd.Action, fd.FixType
		found = true
	}
	var dp disposition
	if readJSONWarn(filepath.Join(ws, dispositionFile), &dp, &warnings) {
		f.Legitimacy, f.Urgency = dp.Legitimacy, dp.Urgency
		f.Layer, f.RootCause, f.Salvage = dp.RootCause.Layer, dp.RootCause.Summary, dp.Salvage.Pointer
		if f.Fingerprint == "" {
			f.Fingerprint = dp.Fingerprint
		}
		found = true
	}
	var ar auditFailReason
	if readJSONWarn(filepath.Join(ws, auditFailReasonFile), &ar, &warnings) && len(ar.Reasons) > 0 {
		f.GateReasons = ar.Reasons
		found = true
	}
	var dg failureDigest
	if readJSONWarn(filepath.Join(ws, failureDigestFile), &dg, &warnings) {
		if f.Fingerprint == "" {
			f.Fingerprint = dg.Fingerprint
		}
		if f.PreClass == "" {
			f.PreClass = dg.PreClass
		}
		found = true
	}
	rounds, w := readAuditRounds(ws)
	warnings = append(warnings, w...)
	if len(rounds) > 0 {
		f.Rounds = rounds
		f.Findings = append([]Finding(nil), rounds[len(rounds)-1].Findings...)
		sortFindings(f.Findings)
		found = true
	}
	if !found {
		return nil, warnings
	}
	return f, warnings
}

// readAuditRounds lists the workspace for the audit report's round archives
// (phasecontract.ParseRoundArchive — indices are NOT contiguous: a dispatch
// that died before writing its report archives nothing while the counter
// still advances), sorts them by index, and appends the live final report as
// the round after the highest archived index. Each round's delta is computed
// against the previous one that exists. Unreadable files are warnings.
func readAuditRounds(ws string) ([]AuditRound, []string) {
	entries, err := os.ReadDir(ws)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, []string{fmt.Sprintf("audit rounds %s: %v", filepath.Base(ws), err)}
	}
	type archived struct {
		index int
		name  string
	}
	var archives []archived
	for _, e := range entries {
		if n, ok := phasecontract.ParseRoundArchive(e.Name(), auditReportName); ok && !e.IsDir() {
			archives = append(archives, archived{n, e.Name()})
		}
	}
	sort.Slice(archives, func(i, j int) bool { return archives[i].index < archives[j].index })

	var rounds []AuditRound
	var warnings []string
	var prev []Finding
	add := func(index int, name string) {
		buf, err := os.ReadFile(filepath.Join(ws, name))
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				warnings = append(warnings, fmt.Sprintf("%s: %v", name, err))
			}
			return
		}
		findings := parseFindings(string(buf))
		resolved, fresh, carried := diffRounds(prev, findings)
		rounds = append(rounds, AuditRound{Round: index, Verdict: reportVerdict(string(buf)),
			Findings: findings, Resolved: resolved, New: fresh, Carried: carried})
		prev = findings
	}
	last := 0
	for _, a := range archives {
		add(a.index, a.name)
		last = a.index
	}
	add(last+1, auditReportName)
	return rounds, warnings
}

// reportVerdict mirrors phases/audit.extractAuditVerdict's order: the
// machine-readable sentinel is authoritative; the shared prose grammar is the
// fallback for reports written against older templates.
func reportVerdict(markdown string) string {
	if v, ok := phasecontract.ParseVerdictSentinel(markdown); ok {
		return v
	}
	return parseVerdict(markdown)
}
