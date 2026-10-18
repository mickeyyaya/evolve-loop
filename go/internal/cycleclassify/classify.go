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

func SetHangClassifier(enabled bool) {
	hangClassifierFn = func() bool { return enabled }
}

type Classification string

const (
	ClassInfrastructure    Classification = "infrastructure"
	ClassShipGateConfig    Classification = "ship-gate-config"
	ClassAuditFail         Classification = "audit-fail"
	ClassBuildFail         Classification = "build-fail"
	ClassIntegrityBreach   Classification = "integrity-breach"
	ClassExitTransportHang Classification = "exit-transport-hang"
	ClassPhaseRefusal      Classification = "phase-refusal"
)

const MarkerQuotaLikelyEmptyOutput = "quota-likely-empty-output"

var (
	reInfrastructure = regexp.MustCompile(`(?i)INFRASTRUCTURE FAILURE|sandbox-exec.*Operation not permitted|sandbox_apply.*permitted|EPERM|rate.?limit|429.*Too Many|529.*Overloaded|connection.refused|ETIMEDOUT|operation timed out`)
	reShipGate       = regexp.MustCompile(`(?i)SHIP_GATE_DENIED|ship-?gate.*(rejected|denied|exited)|integrity.?fail.*Auditor exited`)
	reAuditFail      = regexp.MustCompile(`(?i)Verdict.*FAIL|Verdict.*WARN|verdict.*: *fail`)
	reBuildFail      = regexp.MustCompile(`(?i)Build status.*FAIL|tests.*RED|builder.*failed`)
)

type Result struct {
	Class   Classification `json:"class"`
	Marker  string         `json:"marker,omitempty"`
	Source  string         `json:"source,omitempty"`
	Detail  string         `json:"detail,omitempty"`
	Subject string         `json:"subject,omitempty"`
	Phase   string         `json:"phase,omitempty"`
}

func Classify(workspace string) Result {
	report := filepath.Join(workspace, "orchestrator-report.md")
	reportData, _ := os.ReadFile(report)

	record, recordOK := classifyFromRecord(workspace)
	if recordOK && cyclestate.RefusalDisposition(record.Marker).OutranksSentinel {
		return record
	}
	if cls, ok := classifyFromSentinels(workspace); ok && (!recordOK || sentinelPhase(cls.Source) == record.Phase) {
		return cls
	}
	if recordOK {
		return record
	}

	if m := reInfrastructure.Find(reportData); m != nil {
		return Result{Class: ClassInfrastructure, Marker: string(m), Source: "orchestrator-report.md"}
	}
	if src, marker, ok := scanEventsForInfra(workspace); ok {
		return Result{Class: ClassInfrastructure, Marker: marker, Source: src}
	}
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
	if src, ok := detectEmptyOutputSession(workspace); ok {
		return Result{Class: ClassInfrastructure, Marker: MarkerQuotaLikelyEmptyOutput, Source: src}
	}
	return Result{Class: ClassIntegrityBreach}
}

func classifyFromSentinels(workspace string) (Result, bool) {
	reports, err := globFn(filepath.Join(workspace, "*-report.md"))
	if err != nil {
		return Result{}, false
	}
	sort.Strings(reports)
	for _, path := range reports {
		switch filepath.Base(path) {
		case "orchestrator-report.md", "retrospective-report.md":
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
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
		agent := strings.TrimSuffix(filepath.Base(logPath), "-stdout.log")
		if hasAssistantEvents(filepath.Join(workspace, agent+"-events.ndjson")) {
			continue
		}
		return filepath.Base(logPath), true
	}
	return "", false
}

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
		if bytes.Contains(line, []byte(`"kind":"assistant_text"`)) ||
			bytes.Contains(line, []byte(`"kind": "assistant_text"`)) {
			return true
		}
	}
	return scanner.Err() != nil
}

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

var gitLogFn = func(cycleNum string) bool {
	return gitLogMatchesCycle(context.Background(), gitexec.Default(""), cycleNum)
}

func gitLogMatchesCycle(ctx context.Context, g gitexec.Git, cycleNum string) bool {
	pattern := "cycle " + regexp.QuoteMeta(cycleNum) + "([^0-9]|$)"
	out, err := g.Output(ctx, "log", "--extended-regexp", "--grep="+pattern, "--format=%H", "main")
	if err != nil {
		return false
	}
	return out != ""
}

var globFn = filepath.Glob

var maxScannerBufBytes = 1 << 24

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
	if err := scanner.Err(); err != nil {
		return "", false
	}
	return "", false
}

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
