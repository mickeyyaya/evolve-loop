// Package cycleclassify ports classify_cycle_failure from
// archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:548-637. Given a
// cycle workspace it inspects orchestrator-report.md plus the unified
// *-events.ndjson stream (ADR-0020) and returns one of five canonical
// classifications. The result feeds the failure-adapter (M3) and
// drives the cmd_loop dispatcher policy decision (RETRY vs STOP).
//
// The infrastructure signal that the legacy classifier found by re-scanning
// raw *-stdout.log/*-stderr.log now comes from the normalizer's
// kind==infra_failure events — one owner of the infra-marker vocabulary.
//
// Determinism: classification is order-sensitive. Infrastructure beats
// ship-gate-config beats audit-fail beats build-fail. The order
// matters because a ship-gate-deny report can mention the audit verdict
// in passing, and we want the more specific (and lower-severity)
// ship-gate-config label rather than the broad audit-fail label.
package cycleclassify

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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
	// ClassExitTransportHang — orchestrator finished (SHIPPED verdict +
	// commit on main) but the parent process hung post-artifact. Set
	// only when EVOLVE_HANG_CLASSIFIER=1. 1h retention (vs 7d for
	// integrity-breach) because the underlying cycle succeeded.
	ClassExitTransportHang Classification = "exit-transport-hang"
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
	Class Classification `json:"class"`
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
//  2. *-events.ndjson — kind==infra_failure (the normalizer's typed infra
//     signal; catches the API 529s that landed in memo-stdout.log per
//     cycle-61 forensics, now sourced from the clean stream)
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
	// Pass 2: infrastructure in the unified events stream (ADR-0020). The
	// normalizer owns the infra-marker vocabulary and emits
	// kind==infra_failure for markers it sees on stdout OR stderr, so
	// cycleclassify just filters that kind rather than re-scanning the raw
	// *-stdout.log/*-stderr.log files. Catches the API 529 / sandbox EPERM
	// signal (cycle-61 forensics) from the clean stream.
	//
	// Note: a transient stream-json rate_limit_event normalizes to
	// kind==rate_limit (non-fatal backoff), deliberately distinct from
	// kind==infra_failure — a recovered rate-limit no longer forces an
	// infrastructure verdict on the retry/stop decision.
	if src, marker, ok := scanEventsForInfra(workspace); ok {
		return Result{Class: ClassInfrastructure, Marker: marker, Source: src}
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
	// Gap #6: EVOLVE_HANG_CLASSIFIER=1 two-factor reclassification.
	// When the orchestrator wrote SHIPPED verdict + a matching cycle
	// commit exists on main, the cycle actually succeeded — the parent
	// just hung post-artifact. Reclassify as exit-transport-hang so the
	// failure adapter doesn't treat this as a 7d-retention breach.
	// Source: archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:611-634.
	if os.Getenv("EVOLVE_HANG_CLASSIFIER") == "1" {
		if cls, ok := detectHangShipped(workspace, reportData); ok {
			return cls
		}
	}
	// Report exists but no pattern matched → breach.
	return Result{Class: ClassIntegrityBreach}
}

// detectHangShipped checks the two-factor invariant for
// exit-transport-hang reclassification:
//
//  1. orchestrator-report.md's first non-empty line after "## Verdict"
//     contains "shipped" (case-insensitive)
//  2. `git log --grep="cycle N" main` finds a matching commit
//
// Both must hold. The cycle number is parsed from the workspace path
// suffix (cycle-N).
//
// `git` subprocess is launched via gitLogFn seam so tests can stub.
func detectHangShipped(workspace string, reportData []byte) (Result, bool) {
	// (1) first-line-after-Verdict contains "shipped"
	if !shippedAfterVerdict(reportData) {
		return Result{}, false
	}
	// (2) git log finds a commit matching "cycle N" on main
	base := filepath.Base(workspace)
	cycleNum := strings.TrimPrefix(base, "cycle-")
	if cycleNum == "" || cycleNum == base {
		return Result{}, false
	}
	if !gitLogFn(cycleNum) {
		return Result{}, false
	}
	return Result{
		Class:  ClassExitTransportHang,
		Marker: fmt.Sprintf("SHIPPED verdict + commit for cycle %s on main", cycleNum),
		Source: "orchestrator-report.md + git log",
	}, true
}

// shippedAfterVerdict returns true when the first non-empty line
// AFTER "## Verdict" in reportData contains "shipped" (case-insensitive).
func shippedAfterVerdict(reportData []byte) bool {
	lines := bytes.Split(reportData, []byte("\n"))
	capturing := false
	for _, line := range lines {
		if !capturing {
			if bytes.HasPrefix(bytes.TrimSpace(line), []byte("## Verdict")) ||
				bytes.HasPrefix(bytes.TrimSpace(line), []byte("##Verdict")) {
				capturing = true
			}
			continue
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		return bytes.Contains(bytes.ToLower(line), []byte("shipped"))
	}
	return false
}

// gitLogFn is a test seam — production runs `git -C . log --grep="cycle N" main`
// and reports whether any matching commit exists. Tests substitute a
// stub to drive the branch without spinning up a fixture repo.
var gitLogFn = func(cycleNum string) bool {
	cmd := exec.Command("git", "log", "--grep=cycle "+cycleNum, "--format=%H", "main")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(bytes.TrimSpace(out)) > 0
}

// globFn is a test seam for the filepath.Glob error branch. A literal
// pattern like "*-events.ndjson" cannot fail under filepath.Glob in
// practice, but tests can swap globFn to drive the defensive error
// path that real-world Glob implementations might surface on exotic
// filesystems.
var globFn = filepath.Glob

// maxScannerBufBytes caps the per-line read buffer. A result envelope can
// embed a large payload on the same line, so the cap is generous (matches
// cyclecost). A var so tests can shrink it to exercise the overflow branch.
var maxScannerBufBytes = 1 << 24 // 16MB

// infraEventEnvelope is the subset of a phasestream envelope this scan
// needs: the kind discriminator plus the marker the normalizer recorded.
type infraEventEnvelope struct {
	Kind string `json:"kind"`
	Data struct {
		Marker string `json:"marker"`
	} `json:"data"`
}

// scanEventsForInfra walks the workspace's *-events.ndjson files in sorted
// order and returns the first kind==infra_failure envelope's marker plus the
// basename of the file it was found in. ok=false when no infra event exists,
// no events files are present, or the glob fails — Classify then continues to
// the report-scan passes. Unreadable or malformed files are skipped, matching
// the best-effort raw-log scan this replaces.
func scanEventsForInfra(workspace string) (source, marker string, ok bool) {
	logs, err := globFn(filepath.Join(workspace, "*-events.ndjson"))
	if err != nil {
		return "", "", false
	}
	sort.Strings(logs) // deterministic Source when two files both carry infra
	for _, log := range logs {
		if m, found := firstInfraMarker(log); found {
			return filepath.Base(log), m, true
		}
	}
	return "", "", false
}

// firstInfraMarker returns the marker of the first kind==infra_failure
// envelope in logPath, or ok=false when none is found / the file can't be
// read. A cheap substring pre-check skips the JSON parse for the common
// non-infra lines.
func firstInfraMarker(logPath string) (marker string, ok bool) {
	f, err := os.Open(logPath)
	if err != nil {
		return "", false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<10), maxScannerBufBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte(`"kind":"infra_failure"`)) {
			continue
		}
		var ev infraEventEnvelope
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Kind == "infra_failure" {
			return ev.Data.Marker, true
		}
	}
	// Mirror cyclecost.parseEventsLog: a scan error (e.g. a line exceeding
	// maxScannerBufBytes) yields no infra signal rather than a partial one.
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return "", false
}
