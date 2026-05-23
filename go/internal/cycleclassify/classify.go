// Package cycleclassify ports classify_cycle_failure from
// legacy/scripts/dispatch/evolve-loop-dispatch.sh:548-637. Given a
// cycle workspace it inspects orchestrator-report.md plus per-role
// *-stdout.log/*-stderr.log files and returns one of five canonical
// classifications. The result feeds the failure-adapter (M3) and
// drives the cmd_loop dispatcher policy decision (RETRY vs STOP).
//
// Determinism: classification is order-sensitive. Infrastructure beats
// ship-gate-config beats audit-fail beats build-fail. The order
// matters because a ship-gate-deny report can mention the audit verdict
// in passing, and we want the more specific (and lower-severity)
// ship-gate-config label rather than the broad audit-fail label.
package cycleclassify

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Classification is the typed verdict the classifier returns. The
// string values are wire-compatible with the bash classifier so the
// state.json:failedApproaches[].classification field stays unchanged
// across the Go cutover.
type Classification string

const (
	// ClassInfrastructure — sandbox EPERM, rate limit (429/529), timeout,
	// network errors. Recoverable; deterministic across retries.
	ClassInfrastructure Classification = "infrastructure"
	// ClassShipGateConfig — audit declared PASS but ship-gate refused
	// (v8.27.0). Distinct from audit-fail because the audit itself
	// succeeded; the rejection is in the post-audit gate config/logic.
	ClassShipGateConfig Classification = "ship-gate-config"
	// ClassAuditFail — cycle ran but Auditor verdict was FAIL/WARN.
	ClassAuditFail Classification = "audit-fail"
	// ClassBuildFail — Builder couldn't turn tests GREEN.
	ClassBuildFail Classification = "build-fail"
	// ClassIntegrityBreach — report missing or unclassifiable. Treat as
	// STOP: this is the kernel breach signal (silent skip).
	ClassIntegrityBreach Classification = "integrity-breach"
)

// Patterns are pre-compiled at package init. Each regex is the
// case-insensitive (?i) variant of the bash `grep -qiE` pattern.
var (
	reInfrastructure = regexp.MustCompile(`(?i)INFRASTRUCTURE FAILURE|sandbox-exec.*Operation not permitted|sandbox_apply.*permitted|EPERM|rate.?limit|429.*Too Many|529.*Overloaded|connection.refused|ETIMEDOUT|operation timed out`)
	reShipGate       = regexp.MustCompile(`(?i)SHIP_GATE_DENIED|ship-?gate.*(rejected|denied|exited)|integrity.?fail.*Auditor exited`)
	reAuditFail      = regexp.MustCompile(`(?i)Verdict.*FAIL|Verdict.*WARN|verdict.*: *fail`)
	reBuildFail      = regexp.MustCompile(`(?i)Build status.*FAIL|tests.*RED|builder.*failed`)
)

// Result carries the classification + the marker that triggered it.
// Marker is useful for the cmd_loop log line and for tests assertion.
type Result struct {
	Class  Classification `json:"class"`
	// Marker is the regex hit substring that drove the verdict. Empty
	// for integrity-breach (no marker matched).
	Marker string `json:"marker,omitempty"`
	// Source is the file path where the marker was found, or "" for
	// integrity-breach. Relative to workspace.
	Source string `json:"source,omitempty"`
}

// Classify scans the cycle workspace and returns the resolved
// classification. The workspace path is .evolve/runs/cycle-<N>/ —
// caller is responsible for constructing it.
//
// Scanning order per cycle:
//
//  1. orchestrator-report.md — infra patterns
//  2. *-stdout.log + *-stderr.log — infra patterns (catches API 529s
//     that landed in memo-stdout.log per cycle-61 forensics)
//  3. orchestrator-report.md — ship-gate, audit-fail, build-fail
//
// Returns ClassIntegrityBreach with empty Marker/Source when
// orchestrator-report.md is missing, OR when it exists but no pattern
// hits.
func Classify(workspace string) Result {
	report := filepath.Join(workspace, "orchestrator-report.md")
	reportData, err := os.ReadFile(report)
	if err != nil {
		return Result{Class: ClassIntegrityBreach}
	}

	// Pass 1: infrastructure in orchestrator-report.md.
	if m := reInfrastructure.Find(reportData); m != nil {
		return Result{Class: ClassInfrastructure, Marker: string(m), Source: "orchestrator-report.md"}
	}
	// Pass 2: infrastructure in per-role stdout/stderr logs. v10.x
	// catches API 529 / sandbox EPERM noise that lives in side logs.
	if logs, err := listLogs(workspace); err == nil {
		for _, log := range logs {
			data, err := os.ReadFile(log)
			if err != nil {
				continue
			}
			if m := reInfrastructure.Find(data); m != nil {
				return Result{
					Class:  ClassInfrastructure,
					Marker: string(m),
					Source: filepath.Base(log),
				}
			}
		}
	}
	// Pass 3: post-audit gate. Tested before audit-fail because a
	// SHIP_GATE_DENIED report can also mention the verdict in passing.
	if m := reShipGate.Find(reportData); m != nil {
		return Result{Class: ClassShipGateConfig, Marker: string(m), Source: "orchestrator-report.md"}
	}
	// Pass 4: audit verdict.
	if m := reAuditFail.Find(reportData); m != nil {
		return Result{Class: ClassAuditFail, Marker: string(m), Source: "orchestrator-report.md"}
	}
	// Pass 5: builder couldn't get to GREEN.
	if m := reBuildFail.Find(reportData); m != nil {
		return Result{Class: ClassBuildFail, Marker: string(m), Source: "orchestrator-report.md"}
	}
	// Report exists but no pattern matched → breach.
	return Result{Class: ClassIntegrityBreach}
}

// globFn is a test seam for the filepath.Glob error branch. A literal
// pattern like "*-stdout.log" cannot fail under filepath.Glob in
// practice, but tests can swap globFn to drive the defensive error
// path that real-world Glob implementations might surface on exotic
// filesystems.
var globFn = filepath.Glob

// listLogs returns sorted *-stdout.log + *-stderr.log paths in
// workspace. Stable order makes Classify deterministic when the same
// pattern hits two logs.
func listLogs(workspace string) ([]string, error) {
	stdoutMatches, err := globFn(filepath.Join(workspace, "*-stdout.log"))
	if err != nil {
		return nil, err
	}
	stderrMatches, err := globFn(filepath.Join(workspace, "*-stderr.log"))
	if err != nil {
		return nil, err
	}
	all := append(stdoutMatches, stderrMatches...)
	sort.Strings(all)
	return all, nil
}
