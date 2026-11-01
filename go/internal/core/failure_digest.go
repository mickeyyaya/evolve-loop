package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

var failureDigestMarshal = json.Marshal

type FailureDigest struct {
	Cycle            int      `json:"cycle"`
	Fingerprint      string   `json:"fingerprint"`
	PreClass         string   `json:"pre_class"`
	Recurrence       int      `json:"recurrence"`
	Unexplained      bool     `json:"unexplained,omitempty"`
	Checks           []string `json:"checks,omitempty"`
	ClassFingerprint string   `json:"class_fingerprint,omitempty"`
	ClassLayer       string   `json:"class_layer,omitempty"`
	ClassRecurrence  int      `json:"class_recurrence,omitempty"`
	ClassRecordedAt  string   `json:"class_recorded_at,omitempty"`
	ClassRoute       string   `json:"class_route,omitempty"`
	ClassRouteReason string   `json:"class_route_reason,omitempty"`
}

type RecurrenceCounter interface {
	Count(string) int
}

type auditFailReason struct {
	SchemaVersion int      `json:"schema_version"`
	Phase         string   `json:"phase"`
	Reasons       []string `json:"reasons"`
}

type preClassRule struct {
	bucket  string
	needles []string
}

var preClassRules = []preClassRule{
	{dossier.PreClassInfraError, []string{"infra teardown", "quota", "bridge", "teardown"}},
	{dossier.PreClassGuardAbort, []string{"statemap severed", "guard aborted", "guard abort", "statemap"}},
	{dossier.PreClassGateBlock, []string{"egps", "floor blocked", "red_count", "gate block", "blocked ship", "handoff floor", "deliverable rejected", "deterministic check", "repo-contract", "would red main", "scanner pack"}},
	{dossier.PreClassVerdictFail, []string{"failed to compile", "predicate", "verdict", "acs"}},
}

func classifyPreClass(reasonLower string) string {
	for _, rule := range preClassRules {
		for _, n := range rule.needles {
			if strings.Contains(reasonLower, n) {
				return rule.bucket
			}
		}
	}
	return dossier.PreClassUnknown
}

func AssembleFailureDigest(cycle int, workspace string, rc RecurrenceCounter) (FailureDigest, error) {
	phase, reasons := readAuditFailReason(workspace)
	reasons = withoutDetails(reasons)
	joined := strings.ToLower(strings.Join(reasons, "\n"))
	preClass := classifyPreClass(joined)

	digest := FailureDigest{
		Cycle:       cycle,
		Fingerprint: fingerprint(phase, preClass, reasons),
		PreClass:    preClass,
		Unexplained: reasonsAreContentFree(reasons),
		Checks:      failingChecks(reasons),
	}
	if rc != nil {
		digest.Recurrence = rc.Count(digest.Fingerprint)
	}
	digest = withRecordedClass(workspace, digest)
	return digest, persistFailureDigest(workspace, digest)
}

func persistFailureDigest(workspace string, digest FailureDigest) error {
	b, err := failureDigestMarshal(digest)
	if err != nil {
		return fmt.Errorf("marshal failure digest: %w", err)
	}
	if err := writeArtifactAtomically(filepath.Join(workspace, "failure-digest.json"), b); err != nil {
		return fmt.Errorf("write failure-digest.json: %w", err)
	}
	return writeFailureDigestRecord(workspace, b)
}

func failureDigestRecordPath(workspace string) (string, bool) {
	ws := filepath.Clean(workspace)
	runs := filepath.Dir(ws)
	suffix, isCycle := strings.CutPrefix(filepath.Base(ws), "cycle-")
	cycle, err := strconv.Atoi(suffix)
	if err != nil || !isCycle || strconv.Itoa(cycle) != suffix || filepath.Base(runs) != "runs" || filepath.Base(filepath.Dir(runs)) != ".evolve" {
		return "", false
	}
	return paths.FailureDigestRecordPath(filepath.Dir(runs), cycle), true
}

func writeFailureDigestRecord(workspace string, b []byte) error {
	path, ok := failureDigestRecordPath(workspace)
	if !ok {
		return nil
	}
	err := os.MkdirAll(filepath.Dir(path), 0o755)
	if err == nil {
		err = writeArtifactAtomically(path, b)
	}
	if err != nil {
		return fmt.Errorf("write the harness digest record: %w", err)
	}
	return nil
}

func readAuditFailReason(workspace string) (phase string, reasons []string) {
	raw, err := os.ReadFile(filepath.Join(workspace, "audit-fail-reason.json"))
	if err != nil {
		return "", nil
	}
	var a auditFailReason
	if json.Unmarshal(raw, &a) != nil {
		return "", nil
	}
	return a.Phase, a.Reasons
}

func fingerprint(phase, preClass string, reasons []string) string {
	normalized := make([]string, 0, len(reasons))
	for _, r := range reasons {
		normalized = append(normalized, normalizeReasonForFingerprint(r))
	}
	sum := sha256.Sum256([]byte(phase + "\x00" + preClass + "\x00" + strings.Join(normalized, "\x00")))
	return phase + "|" + preClass + "|" + hex.EncodeToString(sum[:])[:12]
}

var narrativeVerdictToken = regexp.MustCompile(`narrative=(?:PASS|FAIL|WARN|SKIPPED)\b`)

var goTestDurationToken = regexp.MustCompile(`\b\d+\.\d+s\b`)

var cycleNumberToken = regexp.MustCompile(`(?i)\bcycle[ -]\d+(?:-\d+)*`)

var attemptDenominatorToken = regexp.MustCompile(`(?i)\b(attempt|retry)\s+\d+\s*(?:/|of)\s*\d+`)

func withoutDetails(reasons []string) []string {
	out := make([]string, len(reasons))
	for i, r := range reasons {
		out[i] = cyclestate.WithoutDetail(r)
	}
	return out
}

func normalizeReasonForFingerprint(reason string) string {
	reason = cyclestate.WithoutDetail(reason)
	reason = narrativeVerdictToken.ReplaceAllString(reason, "narrative=<verdict>")
	reason = cycleNumberToken.ReplaceAllString(reason, "cycle-N")
	reason = attemptDenominatorToken.ReplaceAllString(reason, "${1} <n>/<n>")
	return goTestDurationToken.ReplaceAllString(reason, "<dur>")
}

func (o *Orchestrator) ensureFailureDigest(cycle int, projectRoot, workspace, fallbackPhase, fallbackReason string) {
	if workspace == "" {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: failure digest not written (cycle %d): missing workspace\n", cycle)
		return
	}
	reasonPath := filepath.Join(workspace, "audit-fail-reason.json")
	if _, statErr := os.Stat(reasonPath); statErr != nil && fallbackReason != "" {
		b, merr := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: fallbackPhase, Reasons: []string{fallbackReason}})
		if merr == nil {
			if werr := writeArtifactAtomically(reasonPath, b); werr != nil {
				fmt.Fprintf(os.Stderr, "[orchestrator] WARN: write fallback fail-reason (cycle %d): %v\n", cycle, werr)
			}
		}
	}
	var rc RecurrenceCounter
	if led, lerr := recurrence.Load(filepath.Join(projectRoot, ".evolve", "recurrence-ledger.json")); lerr == nil {
		rc = led
	}
	if _, derr := AssembleFailureDigest(cycle, workspace, rc); derr != nil {
		fmt.Fprintf(os.Stderr, "[orchestrator] WARN: assemble failure digest (cycle %d): %v\n", cycle, derr)
	}
}

func verdictFailDistinguisher(phase, workspace string) string {
	if fb, ok := phasecontract.ReadFailureBlock(workspace, phase); ok {
		for _, d := range fb.Defects {
			if head := defectHead(d); head != "" {
				return "defect=" + head
			}
		}
	}
	candidates := []string{phase + "-report.md"}
	if c, ok := phasecontract.For(phase); ok && c.ArtifactName != candidates[0] {
		candidates = []string{c.ArtifactName, phase + "-report.md"}
	}
	for _, name := range candidates {
		raw, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			t := strings.TrimSpace(line)
			if strings.HasPrefix(t, "- D") || strings.HasPrefix(t, "- **D") {
				return "defect=" + defectHead(t)
			}
		}
		break
	}
	if ids := BoundTaskIDs(workspace); len(ids) > 0 {
		return "tasks=" + strings.Join(ids, ",")
	}
	return ""
}

var freeTextCycleTokens = regexp.MustCompile(`\b(?:[Cc]ycle[-_ ]?\d+|TestC\d+_)`)

func defectHead(d string) string {
	d = strings.TrimSpace(strings.ReplaceAll(d, "\n", " "))
	if d == "" {
		return ""
	}
	d = freeTextCycleTokens.ReplaceAllStringFunc(d, func(m string) string {
		if strings.HasPrefix(m, "TestC") {
			return "TestCN_"
		}
		return "cycle-N"
	})
	if len(d) > 160 {
		d = strings.ToValidUTF8(d[:160], "")
	}
	return d
}

func causeHead(errText string) string {
	s := strings.TrimSpace(strings.ReplaceAll(errText, "\n", " "))
	if s == "" {
		return ""
	}
	s = freeTextCycleTokens.ReplaceAllStringFunc(s, func(m string) string {
		if strings.HasPrefix(m, "TestC") {
			return "TestCN_"
		}
		return "cycle-N"
	})
	if len(s) > 160 {
		s = "…" + strings.ToValidUTF8(s[len(s)-160:], "")
	}
	return s
}

func agentGradedRouterReason(phase string) string {
	return fmt.Sprintf("phase %s verdict FAIL routed to retro (agent-graded; see the %s report artifact)", phase, phase)
}

func abnormalEpilogueReason(phase string) string {
	return "cycle aborted in phase " + phase + " (abnormal-exit epilogue)"
}

var boilerplateReasonRes = []*regexp.Regexp{
	regexp.MustCompile(`^phase \S+ verdict FAIL routed to retro \(agent-graded; see the \S+ report artifact\)$`),
	regexp.MustCompile(`^cycle aborted in phase \S+ \(abnormal-exit epilogue\)$`),
}

func isBoilerplateRouterReason(r string) bool {
	r = strings.TrimSpace(r)
	for _, re := range boilerplateReasonRes {
		if re.MatchString(r) {
			return true
		}
	}
	return false
}

func reasonsAreContentFree(reasons []string) bool {
	if len(reasons) == 0 {
		return true
	}
	for _, r := range reasons {
		if !isBoilerplateRouterReason(r) {
			return false
		}
	}
	return true
}

func agentGradedFailReason(phase, workspace string) string {
	reason := agentGradedRouterReason(phase)
	if d := verdictFailDistinguisher(phase, workspace); d != "" {
		reason += " " + d
	}
	return reason
}
