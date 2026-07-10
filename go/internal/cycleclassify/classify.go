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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// hangClassifierFn reports whether the exit-transport-hang reclassifier is
// enabled. Default returns false. Production code sets this via SetHangClassifier
// at startup from policy.ClassifyConfig().HangClassifier. Tests may swap the
// function var directly (same pattern as gitLogFn).
var hangClassifierFn = func() bool { return false }

// SetHangClassifier wires the hang-classifier toggle from policy. Called once
// at loop startup so Classify picks up the operator's preference without an
// os.Getenv read on every call.
func SetHangClassifier(enabled bool) {
	hangClassifierFn = func() bool { return enabled }
}

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

// MarkerQuotaLikelyEmptyOutput is the Result.Marker set when the empty-output
// pass (Workstream D2) reclassifies a would-be integrity-breach as recoverable
// infrastructure. The dispatcher matches this exact marker to QUOTA-PAUSE the
// batch (rc=5, auto-resume) instead of burning the next cycle into the same
// quota wall. Exported so cmd_loop can branch on it without re-deriving it.
const MarkerQuotaLikelyEmptyOutput = "quota-likely-empty-output"

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
	reportData, _ := os.ReadFile(report)
	// A missing report is no longer an immediate-breach short-circuit: a
	// mid-cycle quota abort never writes one, and pass 6 below recovers that
	// case from per-phase artifacts. The pattern passes harmlessly miss on
	// empty data; nothing else here needs reportData to be non-empty.

	// Pass 0 (ADR-0039 §7): a phase that self-reported a structured failure
	// class (sentinel v2) is the authority on WHY it failed — the regex
	// passes below are heuristics over prose. Only classes that normalize
	// into the canonical taxonomy are trusted; an out-of-taxonomy agent
	// string falls through to the regex passes (never UnknownClassification,
	// never blind trust).
	if cls, ok := classifyFromSentinels(workspace); ok {
		return cls
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
	// Gap #6: hang-classifier two-factor reclassification.
	// When the orchestrator wrote SHIPPED verdict + a matching cycle
	// commit exists on main, the cycle actually succeeded — the parent
	// just hung post-artifact. Reclassify as exit-transport-hang so the
	// failure adapter doesn't treat this as a 7d-retention breach.
	// Enabled via policy.ClassifyConfig().HangClassifier (replaces EVOLVE_HANG_CLASSIFIER).
	// Source: archive/legacy/scripts/dispatch/evolve-loop-dispatch.sh:611-634.
	if hangClassifierFn() {
		if cls, ok := detectHangShipped(workspace, reportData); ok {
			return cls
		}
	}
	// Pass 6: last-resort empty-output → quota-likely. Runs after all pattern
	// passes so it can never mask a classifiable failure. See detectEmptyOutputSession.
	if src, ok := detectEmptyOutputSession(workspace); ok {
		return Result{Class: ClassInfrastructure, Marker: MarkerQuotaLikelyEmptyOutput, Source: src}
	}
	// Report exists but no pattern matched → breach.
	return Result{Class: ClassIntegrityBreach}
}

// classifyFromSentinels scans the workspace's phase reports (sorted glob —
// deterministic when several phases self-reported) for a FAIL/WARN verdict
// sentinel carrying a failure block, and returns its class normalized through
// failurelog.NormalizeLegacy. failurelog.Record re-normalizes idempotently
// (canonical values pass through), so the canonical string is wire-safe.
func classifyFromSentinels(workspace string) (Result, bool) {
	reports, err := globFn(filepath.Join(workspace, "*-report.md"))
	if err != nil {
		return Result{}, false
	}
	sort.Strings(reports)
	for _, path := range reports {
		switch filepath.Base(path) {
		case "orchestrator-report.md", "retrospective-report.md":
			// The supervisor's report is prose for the regex passes; the
			// retrospective is learning ABOUT a failure, not the failure.
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		// FAIL only: a phase's FAIL IS the cycle's failure, but a WARN is
		// not necessarily why the cycle stopped — a later infra crash must
		// keep winning (the pass-ordering invariant: infrastructure beats
		// audit-fail). WARNs fall through to the regex passes.
		s, ok := phasecontract.ParseVerdictSentinelFull(string(data))
		if !ok || s.Verdict != "FAIL" || s.Failure == nil || s.Failure.Class == "" {
			continue
		}
		norm := failurelog.NormalizeLegacy(s.Failure.Class)
		if norm == failurelog.UnknownClassification {
			continue // out-of-taxonomy → regex passes decide
		}
		return Result{
			Class:  Classification(norm),
			Marker: s.Failure.Class,
			Source: filepath.Base(path),
		}, true
	}
	return Result{}, false
}

// detectEmptyOutputSession reports whether some phase was launched (its
// <agent>-stdout.log exists) but produced no model output — empty/whitespace
// stdout AND zero assistant events in the matching <agent>-events.ndjson. That
// pairing is the subscription-quota-wall signature (claude -p exits empty when
// the quota is exhausted). Requiring the stdout.log to EXIST is the guard that
// separates this from a true silent skip (where the phase never ran, so no log
// was created) — the latter must stay an integrity-breach.
func detectEmptyOutputSession(workspace string) (source string, ok bool) {
	logs, err := globFn(filepath.Join(workspace, "*-stdout.log"))
	if err != nil {
		return "", false
	}
	sort.Strings(logs) // deterministic Source when several phases are empty
	for _, logPath := range logs {
		data, readErr := os.ReadFile(logPath)
		if readErr != nil {
			continue // unreadable: can't assert "launched but empty"
		}
		if len(bytes.TrimSpace(data)) != 0 {
			continue // produced output → not a quota wall
		}
		// Empty stdout. Confirm zero assistant events in the paired stream so a
		// log that was merely truncated (but events captured output) isn't
		// misread as quota.
		agent := strings.TrimSuffix(filepath.Base(logPath), "-stdout.log")
		if hasAssistantEvents(filepath.Join(workspace, agent+"-events.ndjson")) {
			continue
		}
		return filepath.Base(logPath), true
	}
	return "", false
}

// hasAssistantEvents reports whether eventsPath contains at least one
// assistant_text envelope (i.e. the model produced output). A missing or
// unreadable file ⇒ false (no assistant output observed), consistent with the
// empty-session signature.
//
// On scanner.Err() (e.g. a line exceeding maxScannerBufBytes) the function
// returns TRUE — conservatively assuming output was present so the caller does
// not falsely flag a large-output truncation as a quota wall. Mirrors the
// safety-bias in scanEventsForInfra.
func hasAssistantEvents(eventsPath string) bool {
	f, err := os.Open(eventsPath)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<10), maxScannerBufBytes)
	for scanner.Scan() {
		line := scanner.Bytes()
		// Tolerant of JSON formatting variants: phasestream emits compact
		// `"kind":"assistant_text"` today, but a pretty-printed `"kind": "...`
		// must not be missed if a downstream re-serializer ever inserts space.
		if bytes.Contains(line, []byte(`"kind":"assistant_text"`)) ||
			bytes.Contains(line, []byte(`"kind": "assistant_text"`)) {
			return true
		}
	}
	if scanner.Err() != nil {
		return true // truncation: assume output present rather than risk a false quota-pause
	}
	return false
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

// gitLogFn is a test seam — production runs `git log --grep="cycle N"
// --format=%H main` in the caller's cwd and reports whether any matching commit
// exists. Tests substitute a stub to drive the branch without spinning up a
// fixture repo. The default delegates to gitLogMatchesCycle, which is
// unit-testable via the gitexec seam (see classify_git_test.go).
var gitLogFn = func(cycleNum string) bool {
	return gitLogMatchesCycle(context.Background(), gitexec.Default(""), cycleNum)
}

// gitLogMatchesCycle reports whether any commit on main matches "cycle N",
// running git through the injectable gitexec seam. Any non-zero exit or error
// (e.g. not a git repo) yields false — matching the original .Output() form,
// which returned false on err.
func gitLogMatchesCycle(ctx context.Context, g gitexec.Git, cycleNum string) bool {
	out, err := g.Output(ctx, "log", "--grep=cycle "+cycleNum, "--format=%H", "main")
	if err != nil {
		return false
	}
	return out != ""
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
// needs: the kind discriminator, the emitting phase, plus the marker and the
// raw excerpt the normalizer recorded. Excerpt + phase drive the cycle-641/642
// prompt-echo veto (isPromptEchoSelfReport).
type infraEventEnvelope struct {
	Kind   string `json:"kind"`
	Source struct {
		Phase string `json:"phase"`
	} `json:"source"`
	Data struct {
		Marker  string `json:"marker"`
		Excerpt string `json:"excerpt"`
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
		if m, found := firstInfraMarker(workspace, log); found {
			return filepath.Base(log), m, true
		}
	}
	return "", "", false
}

// isPromptEchoSelfReport reports whether an infra_failure event for phase is a
// self-echo of that phase's OWN prompt on an otherwise-successful phase, and so
// must NOT drive an infrastructure verdict (cycle-641/642 fix-of-record; retro
// recommendation #2 — deliverable-PASS + clean-exit are source-of-truth, a bare
// keyword echo cannot override them). All three must hold:
//
//  1. the event excerpt is a verbatim substring of <phase>-prompt.txt — the
//     agent quoting its own instruction text (e.g. an Adversarial Reviewer's
//     exploit checklist "...missing rate limits."), not a runtime banner;
//  2. the phase's deliverable <phase>-report.md carries a PASS verdict sentinel;
//  3. the phase's driver exited 0 in llm-calls.ndjson.
//
// Any missing/unreadable artifact fails the check CLOSED (no veto), so a genuine
// runtime infra signal — non-zero exit, or an excerpt absent from the prompt —
// still classifies as infrastructure. An empty phase or excerpt never vetoes.
func isPromptEchoSelfReport(workspace, phase, excerpt string) bool {
	excerpt = strings.TrimSpace(excerpt)
	if phase == "" || excerpt == "" {
		return false
	}
	// (1) excerpt echoes the injected prompt text.
	prompt, err := os.ReadFile(filepath.Join(workspace, phase+"-prompt.txt"))
	if err != nil || !strings.Contains(string(prompt), excerpt) {
		return false
	}
	// (2) deliverable declares PASS.
	report, err := os.ReadFile(filepath.Join(workspace, phase+"-report.md"))
	if err != nil {
		return false
	}
	if s, ok := phasecontract.ParseVerdictSentinelFull(string(report)); !ok || s.Verdict != "PASS" {
		return false
	}
	// (3) driver exited 0.
	return driverExitedZero(workspace, phase)
}

// driverExitedZero reports whether phase recorded a zero exit_code in
// llm-calls.ndjson. The LAST matching record wins (a retry's final attempt is
// authoritative). Missing file / no record / absent exit_code ⇒ false (a clean
// exit is unproven, so never veto). Malformed lines are skipped.
func driverExitedZero(workspace, phase string) bool {
	f, err := os.Open(filepath.Join(workspace, "llm-calls.ndjson"))
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	found, zero := false, false
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<10), maxScannerBufBytes)
	for scanner.Scan() {
		var rec struct {
			Phase    string `json:"phase"`
			ExitCode *int   `json:"exit_code"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}
		if rec.Phase != phase || rec.ExitCode == nil {
			continue
		}
		found, zero = true, *rec.ExitCode == 0
	}
	return found && zero
}

// firstInfraMarker returns the marker of the first kind==infra_failure
// envelope in logPath that is NOT a prompt-echo self-report (see
// isPromptEchoSelfReport), or ok=false when none is found / the file can't be
// read. A cheap substring pre-check skips the JSON parse for the common
// non-infra lines. workspace is needed to resolve the per-phase artifacts the
// echo veto reads.
func firstInfraMarker(workspace, logPath string) (marker string, ok bool) {
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
		if ev.Kind != "infra_failure" {
			continue
		}
		if isPromptEchoSelfReport(workspace, ev.Source.Phase, ev.Data.Excerpt) {
			continue // agent quoting its own prompt on a PASS/exit-0 phase — not a runtime infra signal
		}
		return ev.Data.Marker, true
	}
	// Mirror cyclecost.parseEventsLog: a scan error (e.g. a line exceeding
	// maxScannerBufBytes) yields no infra signal rather than a partial one.
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return "", false
}
