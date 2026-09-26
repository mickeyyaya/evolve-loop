// Package cycleclassify reads a finished or aborted cycle workspace and returns
// the canonical failure classification behind the loop's retry-or-stop decision.
// See docs/architecture/packages/internal-cycleclassify.md.
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

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/failurelog"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/llmcalls"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasetiming"
)

var hangClassifierFn = func() bool { return false }

// SetHangClassifier sets the exit-transport-hang reclassifier toggle from policy at loop startup.
func SetHangClassifier(enabled bool) {
	hangClassifierFn = func() bool { return enabled }
}

// Classification is a failure class whose string values stay wire-compatible with the legacy bash classifier.
type Classification string

const (
	// ClassInfrastructure covers sandbox EPERM, rate limits, timeouts and network errors; it is recoverable.
	ClassInfrastructure Classification = "infrastructure"
	// ClassShipGateConfig means the audit passed but the post-audit ship gate refused.
	ClassShipGateConfig Classification = "ship-gate-config"
	// ClassAuditFail means the cycle ran but the auditor's verdict was FAIL or WARN.
	ClassAuditFail Classification = "audit-fail"
	// ClassBuildFail means the builder could not turn the tests GREEN.
	ClassBuildFail Classification = "build-fail"
	// ClassIntegrityBreach means nothing explained the failure; the loop treats it as a kernel breach and stops.
	ClassIntegrityBreach Classification = "integrity-breach"
	// ClassExitTransportHang means the cycle shipped but its parent process hung afterwards; only the enabled hang classifier sets it.
	ClassExitTransportHang Classification = "exit-transport-hang"
	// ClassPhaseRefusal means the last recorded phase outcome is a FAIL carrying a diagnostic code, a task-attributable refusal.
	ClassPhaseRefusal Classification = "phase-refusal"
)

// MarkerQuotaLikelyEmptyOutput marks an empty-output session reclassified as infrastructure; the loop quota-pauses on it.
const MarkerQuotaLikelyEmptyOutput = "quota-likely-empty-output"

// Case-insensitive ports of the legacy `grep -qiE` patterns. `.` stops at a newline,
// so a marker must sit on one line, as it did for the line-by-line grep.
var (
	reInfrastructure = regexp.MustCompile(`(?i)INFRASTRUCTURE FAILURE|sandbox-exec.*Operation not permitted|sandbox_apply.*permitted|EPERM|rate.?limit|429.*Too Many|529.*Overloaded|connection.refused|ETIMEDOUT|operation timed out`)
	reShipGate       = regexp.MustCompile(`(?i)SHIP_GATE_DENIED|ship-?gate.*(rejected|denied|exited)|integrity.?fail.*Auditor exited`)
	reAuditFail      = regexp.MustCompile(`(?i)Verdict.*FAIL|Verdict.*WARN|verdict.*: *fail`)
	reBuildFail      = regexp.MustCompile(`(?i)Build status.*FAIL|tests.*RED|builder.*failed`)
)

// Result is a classification plus the marker and source file that triggered it.
type Result struct {
	Class Classification `json:"class"`
	// Marker is the matched text that drove the verdict; empty for integrity-breach.
	Marker string `json:"marker,omitempty"`
	// Source is the workspace-relative file holding Marker; empty for integrity-breach.
	Source string `json:"source,omitempty"`
	// Detail, Subject and Phase are set only for ClassPhaseRefusal: the diagnostic's message, the item it names and the refusing phase.
	Detail  string `json:"detail,omitempty"`
	Subject string `json:"subject,omitempty"`
	Phase   string `json:"phase,omitempty"`
}

// Classify returns the classification of the cycle workspace .evolve/runs/cycle-<N>/, or integrity-breach when nothing explains the failure.
func Classify(workspace string) Result {
	report := filepath.Join(workspace, "orchestrator-report.md")
	// A missing report is not yet a breach: a mid-cycle quota abort writes none, and the empty-output pass recovers it.
	reportData, _ := os.ReadFile(report)

	// The record is read first so the sentinel pass is recency-aware: a classed FAIL
	// sentinel outranks it only when it comes from the phase the cycle stopped on.
	record, recordOK := classifyFromRecord(workspace)
	if cls, ok := classifyFromSentinels(workspace); ok && (!recordOK || sentinelPhase(cls.Source) == record.Phase) {
		return cls
	}
	// A coded refusal precedes the infra passes: an earlier infra marker did not stop the cycle, and none can follow it.
	if recordOK {
		return record
	}

	if m := reInfrastructure.Find(reportData); m != nil {
		return Result{Class: ClassInfrastructure, Marker: string(m), Source: "orchestrator-report.md"}
	}
	if src, marker, ok := scanEventsForInfra(workspace); ok {
		return Result{Class: ClassInfrastructure, Marker: marker, Source: src}
	}
	// Ship gate precedes audit-fail because a SHIP_GATE_DENIED report can mention the verdict in passing.
	if m := reShipGate.Find(reportData); m != nil {
		return Result{Class: ClassShipGateConfig, Marker: string(m), Source: "orchestrator-report.md"}
	}
	if m := reAuditFail.Find(reportData); m != nil {
		return Result{Class: ClassAuditFail, Marker: string(m), Source: "orchestrator-report.md"}
	}
	if m := reBuildFail.Find(reportData); m != nil {
		return Result{Class: ClassBuildFail, Marker: string(m), Source: "orchestrator-report.md"}
	}
	if hangClassifierFn() {
		if cls, ok := detectHangShipped(workspace, reportData); ok {
			return cls
		}
	}
	// Last, so it can never mask a classifiable failure.
	if src, ok := detectEmptyOutputSession(workspace); ok {
		return Result{Class: ClassInfrastructure, Marker: MarkerQuotaLikelyEmptyOutput, Source: src}
	}
	return Result{Class: ClassIntegrityBreach}
}

// classifyFromSentinels returns the first FAIL sentinel's failure class, in report-name
// order, that normalizes into the canonical taxonomy.
func classifyFromSentinels(workspace string) (Result, bool) {
	reports, err := globFn(filepath.Join(workspace, "*-report.md"))
	if err != nil {
		return Result{}, false
	}
	sort.Strings(reports)
	for _, path := range reports {
		switch filepath.Base(path) {
		case "orchestrator-report.md", "retrospective-report.md":
			// The orchestrator report is prose for the regex passes; a retrospective is about a failure, not the failure.
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		// FAIL only: a WARN is not necessarily why the cycle stopped, so a later infra crash keeps winning.
		s, ok := phasecontract.ParseVerdictSentinelFull(string(data))
		if !ok || s.Verdict != "FAIL" || s.Failure == nil || s.Failure.Class == "" {
			continue
		}
		norm := failurelog.NormalizeLegacy(s.Failure.Class)
		if norm == failurelog.UnknownClassification {
			continue
		}
		return Result{
			Class:  Classification(norm),
			Marker: s.Failure.Class,
			Source: filepath.Base(path),
		}, true
	}
	return Result{}, false
}

// detectEmptyOutputSession finds a launched phase (its stdout.log exists) with an empty log and no
// assistant events, the quota-wall signature. A missing log means the phase never ran: still a breach.
func detectEmptyOutputSession(workspace string) (source string, ok bool) {
	logs, err := globFn(filepath.Join(workspace, "*-stdout.log"))
	if err != nil {
		return "", false
	}
	sort.Strings(logs)
	for _, logPath := range logs {
		data, readErr := os.ReadFile(logPath)
		if readErr != nil {
			continue
		}
		if len(bytes.TrimSpace(data)) != 0 {
			continue
		}
		// A truncated log whose events still captured output is not a quota wall.
		agent := strings.TrimSuffix(filepath.Base(logPath), "-stdout.log")
		if hasAssistantEvents(filepath.Join(workspace, agent+"-events.ndjson")) {
			continue
		}
		return filepath.Base(logPath), true
	}
	return "", false
}

// hasAssistantEvents reports whether eventsPath holds an assistant_text envelope. A missing file is
// false; a scan error is true, so a large-output truncation is never read as a quota wall.
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
		// The spaced form tolerates a re-serializer that pretty-prints the envelope.
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

// detectHangShipped reclassifies a cycle whose report verdict says shipped and whose
// "cycle N" commit is on main: the cycle succeeded and only the parent process hung.
func detectHangShipped(workspace string, reportData []byte) (Result, bool) {
	if !shippedAfterVerdict(reportData) {
		return Result{}, false
	}
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

// shippedAfterVerdict reports whether the first non-empty line after "## Verdict" contains "shipped", ignoring case.
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

// gitLogFn is a test seam over gitLogMatchesCycle, run in the process working directory.
var gitLogFn = func(cycleNum string) bool {
	return gitLogMatchesCycle(context.Background(), gitexec.Default(""), cycleNum)
}

// gitLogMatchesCycle reports whether a commit on main mentions "cycle N"; any git error is false.
func gitLogMatchesCycle(ctx context.Context, g gitexec.Git, cycleNum string) bool {
	out, err := g.Output(ctx, "log", "--grep=cycle "+cycleNum, "--format=%H", "main")
	if err != nil {
		return false
	}
	return out != ""
}

// globFn is a test seam for the Glob error branch, which a literal pattern cannot reach.
var globFn = filepath.Glob

// maxScannerBufBytes caps one events line, generously because a result envelope can embed a large
// payload (as in cyclecost). Tests shrink it to reach the overflow branch.
var maxScannerBufBytes = 1 << 24

// infraEventEnvelope is the subset of a phasestream envelope that the infra scan and the prompt-echo veto read.
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

// scanEventsForInfra returns the first infra_failure marker across the sorted *-events.ndjson files,
// and the file it came from. Unreadable or malformed files are skipped.
func scanEventsForInfra(workspace string) (source, marker string, ok bool) {
	logs, err := globFn(filepath.Join(workspace, "*-events.ndjson"))
	if err != nil {
		return "", "", false
	}
	sort.Strings(logs)
	for _, log := range logs {
		if m, found := firstInfraMarker(workspace, log); found {
			return filepath.Base(log), m, true
		}
	}
	return "", "", false
}

// isPromptEchoSelfReport reports whether an infra_failure excerpt only echoes the phase's own prompt
// on a phase that passed and exited 0. Any missing artifact fails closed, so genuine infra still counts.
func isPromptEchoSelfReport(workspace, phase, excerpt string) bool {
	excerpt = strings.TrimSpace(excerpt)
	if phase == "" || excerpt == "" {
		return false
	}
	prompt, err := os.ReadFile(filepath.Join(workspace, phase+"-prompt.txt"))
	if err != nil || !strings.Contains(string(prompt), excerpt) {
		return false
	}
	report, err := os.ReadFile(filepath.Join(workspace, phasecontract.ArtifactFilename(phase)))
	if err != nil {
		return false
	}
	if s, ok := phasecontract.ParseVerdictSentinelFull(string(report)); !ok || s.Verdict != "PASS" {
		return false
	}
	return driverExitedZero(workspace, phase)
}

// driverExitedZero reports whether the phase's last llm-calls.ndjson record exited 0. The last
// record wins because a retry's final attempt owns the exit; no record leaves it unproven, so false.
func driverExitedZero(workspace, phase string) bool {
	result, err := llmcalls.ReadWorkspace(workspace)
	if err != nil && len(result.Records) == 0 {
		return false
	}
	found, zero := false, false
	for _, rec := range result.Records {
		if rec.Phase != phase || rec.ExitCode == nil {
			continue
		}
		found, zero = true, *rec.ExitCode == 0
	}
	return found && zero
}

// firstInfraMarker returns the first infra_failure marker in logPath that is not a prompt-echo self-report.
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
			continue
		}
		return ev.Data.Marker, true
	}
	// A scan error yields no infra signal rather than a partial one, as in cyclecost.parseEventsLog.
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return "", false
}

// classifyFromRecord reports a phase-refusal when the last phase-timing.json outcome is a FAIL with a coded error diagnostic.
func classifyFromRecord(workspace string) (Result, bool) {
	entries, err := phasetiming.Read(workspace)
	if err != nil || len(entries) == 0 {
		return Result{}, false
	}
	last := entries[len(entries)-1]
	if last.Verdict != "FAIL" {
		return Result{}, false
	}
	codes := cyclestate.ErrorCodes(last.Diagnostics)
	if len(codes) == 0 {
		return Result{}, false
	}
	res := Result{Class: ClassPhaseRefusal, Marker: codes[0], Source: phasetiming.FileName, Phase: last.Phase}
	for _, d := range last.Diagnostics {
		if d.Severity == cyclestate.SeverityError && d.Code == codes[0] {
			res.Detail, res.Subject = d.Message, d.Subject
			break
		}
	}
	return res, true
}

func sentinelPhase(source string) string {
	return strings.TrimSuffix(filepath.Base(source), "-report.md")
}
