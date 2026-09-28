package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/recurrence"
)

// FailureDigest is the stable failure identity written to
// <workspace>/failure-digest.json and cross-checked by VerifyDisposition.
type FailureDigest struct {
	Cycle       int    `json:"cycle"`
	Fingerprint string `json:"fingerprint"`
	PreClass    string `json:"pre_class"`
	Recurrence  int    `json:"recurrence"`
	// Unexplained marks a digest whose reason set carries no distinguishing
	// content (empty, or exactly the content-free agent-graded router line).
	Unexplained bool `json:"unexplained,omitempty"`
}

// RecurrenceCounter is the minimal read view of the recurrence ledger the
// assembler consults (satisfied by *recurrence.Ledger via Count(string) int). The
// count is READ THROUGH this seam, never fabricated.
type RecurrenceCounter interface {
	Count(string) int
}

// auditFailReason mirrors the coherence-floor schema of audit-fail-reason.json.
type auditFailReason struct {
	SchemaVersion int      `json:"schema_version"`
	Phase         string   `json:"phase"`
	Reasons       []string `json:"reasons"`
}

// preClassRule maps a set of lowercase keyword needles to a pre-class bucket.
// Rules are evaluated in order; the FIRST rule with any matching needle wins, so
// the ordering encodes precedence when a reason could touch several classes.
type preClassRule struct {
	bucket  string
	needles []string
}

// preClassRules classify a failure reason into a coarse bucket from real
// reason text. Order = precedence.
var preClassRules = []preClassRule{
	{"infra-error", []string{"infra teardown", "quota", "bridge", "teardown"}},
	{"guard-abort", []string{"statemap severed", "guard aborted", "guard abort", "statemap"}},
	{"gate-block", []string{"egps", "floor blocked", "red_count", "gate block", "blocked ship", "handoff floor", "deliverable rejected", "deterministic check", "repo-contract", "would red main", "scanner pack"}},
	{"verdict-fail", []string{"failed to compile", "predicate", "verdict", "acs"}},
}

// classifyPreClass returns the bucket for a joined, lowercased reason string, or
// "unknown" when no rule matches (the fail-soft / novel-failure default).
func classifyPreClass(reasonLower string) string {
	for _, rule := range preClassRules {
		for _, n := range rule.needles {
			if strings.Contains(reasonLower, n) {
				return rule.bucket
			}
		}
	}
	return "unknown"
}

// AssembleFailureDigest reads <workspace>/audit-fail-reason.json fail-soft
// (absent/malformed degrades to "unknown", never aborts), derives a stable
// phase-composed fingerprint and pre-class bucket, reads the recurrence count
// through rc, writes the digest atomically, and returns it. Only a write
// failure is returned as an error.
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
	}
	if rc != nil {
		digest.Recurrence = rc.Count(digest.Fingerprint)
	}

	b, err := json.Marshal(digest)
	if err != nil {
		return digest, fmt.Errorf("marshal failure digest: %w", err)
	}
	if err := writeArtifactAtomically(filepath.Join(workspace, "failure-digest.json"), b); err != nil {
		return digest, fmt.Errorf("write failure-digest.json: %w", err)
	}
	return digest, nil
}

// readAuditFailReason returns ("", nil) when the artifact is absent or
// malformed, so the caller degrades to "unknown" rather than aborting.
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

// fingerprint composes "<phase>|<preClass>|<hash>". Phase is both a prefix
// and a hash input, so two failures differing only in phase never collapse to
// one id.
func fingerprint(phase, preClass string, reasons []string) string {
	normalized := make([]string, 0, len(reasons))
	for _, r := range reasons {
		normalized = append(normalized, normalizeReasonForFingerprint(r))
	}
	sum := sha256.Sum256([]byte(phase + "\x00" + preClass + "\x00" + strings.Join(normalized, "\x00")))
	return phase + "|" + preClass + "|" + hex.EncodeToString(sum[:])[:12]
}

// narrativeVerdictToken matches the audit phase's narrative=<verdict> token
// (phases/audit/audit.go).
var narrativeVerdictToken = regexp.MustCompile(`narrative=(?:PASS|FAIL|WARN|SKIPPED)\b`)

// goTestDurationToken matches go-test timing chrome. Decimal seconds only —
// an integer-second token like "-timeout 300s" is configuration and stays
// identity-bearing.
var goTestDurationToken = regexp.MustCompile(`\b\d+\.\d+s\b`)

// cycleNumberToken matches the cycle-numbered tokens per-cycle artifact paths
// bake into reason text, and the bare "cycle N" of prose reasons; same shape
// as the carryover unit's cycleTokenRE (internal/core/carryover/identity.go).
// Only the number folds, so two different artifacts in one cycle dir stay two
// defects.
var cycleNumberToken = regexp.MustCompile(`(?i)\bcycle[ -]\d+(?:-\d+)*`)

// attemptDenominatorToken matches a retry loop's attempt index. The keyword
// is preserved via ${1} so an "attempt" reason and a "retry" reason cannot
// collapse into each other.
var attemptDenominatorToken = regexp.MustCompile(`(?i)\b(attempt|retry)\s+\d+\s*(?:/|of)\s*\d+`)

func withoutDetails(reasons []string) []string {
	out := make([]string, len(reasons))
	for i, r := range reasons {
		out[i] = cyclestate.WithoutDetail(r)
	}
	return out
}

// normalizeReasonForFingerprint projects a reason onto its defect identity.
// Display and identity are two projections of the one reason string: the
// digest, the dossier and audit-fail-reason.json all keep the reason
// verbatim — only the hash input is normalized.
func normalizeReasonForFingerprint(reason string) string {
	reason = cyclestate.WithoutDetail(reason)
	reason = narrativeVerdictToken.ReplaceAllString(reason, "narrative=<verdict>")
	reason = cycleNumberToken.ReplaceAllString(reason, "cycle-N")
	reason = attemptDenominatorToken.ReplaceAllString(reason, "${1} <n>/<n>")
	return goTestDurationToken.ReplaceAllString(reason, "<dur>")
}

// ensureFailureDigest is best-effort: a nil recurrence ledger degrades to
// recurrence 0, and a digest write failure only WARNs. fallbackPhase and
// fallbackReason are written into audit-fail-reason.json only when no floor
// already wrote one; a floor-written artifact always wins.
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
	// Same candidate order as ReadFailureBlock: the registry artifact name
	// first (tdd's report is test-report.md), then the conventional name.
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
		break // a readable report without bullets settles this layer
	}
	if ids := BoundTaskIDs(workspace); len(ids) > 0 {
		return "tasks=" + strings.Join(ids, ",")
	}
	return ""
}

// freeTextCycleTokens matches cycle-numbered chrome inside prose defect text;
// the in-text counterpart of the audit phase's anchored egpsRedIDCycleTokens.
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

// causeHead is tail-kept when over budget, unlike defectHead: an error chain
// grows prefix-first, so the distinguishing content lives at the end.
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

// agentGradedRouterReason is the one template for the dispatch loop's
// "verdict FAIL routed to retro" fallback reason, shared with
// isBoilerplateRouterReason so the writer and the content-free detector can
// never drift apart.
func agentGradedRouterReason(phase string) string {
	return fmt.Sprintf("phase %s verdict FAIL routed to retro (agent-graded; see the %s report artifact)", phase, phase)
}

// abnormalEpilogueReason is the one template for the abnormal-exit epilogue's
// fallback reason. Every abort in the same phase renders identically, so this
// template asserts no identity.
func abnormalEpilogueReason(phase string) string {
	return "cycle aborted in phase " + phase + " (abnormal-exit epilogue)"
}

// boilerplateReasonRes match EXACTLY the bare fallback templates — anchored
// both ends, so a reason carrying any appended distinguisher or real error
// text is content-bearing.
var boilerplateReasonRes = []*regexp.Regexp{
	regexp.MustCompile(`^phase \S+ verdict FAIL routed to retro \(agent-graded; see the \S+ report artifact\)$`),
	regexp.MustCompile(`^cycle aborted in phase \S+ \(abnormal-exit epilogue\)$`),
}

// isBoilerplateRouterReason reports whether one reason is a bare fallback
// template with no distinguishing content.
func isBoilerplateRouterReason(r string) bool {
	r = strings.TrimSpace(r)
	for _, re := range boilerplateReasonRes {
		if re.MatchString(r) {
			return true
		}
	}
	return false
}

// reasonsAreContentFree reports whether a reason set asserts NO defect
// identity: empty, or every line is the bare router boilerplate.
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

// agentGradedFailReason composes the dispatch loop's fallback fail-reason for
// an agent-graded FAIL: the router line plus the strongest per-failure
// distinguisher the workspace offers.
func agentGradedFailReason(phase, workspace string) string {
	reason := agentGradedRouterReason(phase)
	if d := verdictFailDistinguisher(phase, workspace); d != "" {
		reason += " " + d
	}
	return reason
}
