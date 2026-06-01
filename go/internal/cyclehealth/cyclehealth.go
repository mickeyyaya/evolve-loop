// Package cyclehealth performs an 11-signal integrity check on a
// completed cycle's workspace and writes the findings to
// <workspace>/cycle-health.json. The orchestrator and Scout read the
// file before the next phase; any ANOMALY in a non-WARN-only signal
// halts the cycle so the integrity breach is investigated.
//
// The 11 signals (each emits zero or more anomalies):
//
//  1. ledger_completeness  — every required role appears in ledger.jsonl
//  2. ledger_timestamps    — entries are monotonic, no future timestamps
//  3. workspace_artifacts  — required reports exist (scout/build/audit)
//  4. artifact_checksums   — SHA256 of every artifact matches its ledger entry
//  5. challenge_tokens     — each subagent invocation has a unique token
//  6. velocity             — wall-clock per phase within reasonable bounds
//  7. substance            — artifact bodies non-trivial (> 100 chars)
//  8. canary_files         — no orphan canary files left from previous cycles
//  9. hash_chain           — ledger prev_hash chain unbroken
//
// 10. cost_envelope        — per-phase cost ≤ EVOLVE_PHASE_COST_CEILING
// 11. duplicate_ledger     — no two ledger entries with same SHA
//
// Per the skill docs: "Any ANOMALY = halt." Operators bypass via
// EVOLVE_SKIP_CYCLE_HEALTH=1 (logged loudly).
//
// v12.1 Phase 2A port. CLI: `evolve cycle-health <cycle-N> <workspace>`.
package cyclehealth

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Severity classifies a single anomaly's blocking force.
type Severity string

const (
	SeverityWarn  Severity = "warn"  // advisory; loop continues
	SeverityFatal Severity = "fatal" // HALT; investigate before next cycle
)

// Anomaly is a single integrity finding.
type Anomaly struct {
	Signal   string   `json:"signal"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
}

// Report is the on-disk JSON shape (the public contract for Scout and
// the orchestrator). Healthy cycles emit an empty Anomalies slice.
type Report struct {
	Cycle        int       `json:"cycle"`
	Workspace    string    `json:"workspace"`
	GeneratedAt  time.Time `json:"generated_at"`
	SignalsRun   []string  `json:"signals_run"`
	Anomalies    []Anomaly `json:"anomalies"`
	OverallFatal bool      `json:"overall_fatal"` // true when any anomaly has SeverityFatal
}

// Options configures Check. Cycle and Workspace are required.
type Options struct {
	Cycle     int
	Workspace string
	NowFn     func() time.Time
}

// Check runs the 11 signals and writes cycle-health.json. Returns the
// Report whether or not anomalies were found; the caller decides
// whether to HALT based on Report.OverallFatal.
func Check(opts Options) (Report, error) {
	if opts.Cycle <= 0 {
		return Report{}, fmt.Errorf("cyclehealth: Cycle must be > 0")
	}
	if opts.Workspace == "" {
		return Report{}, fmt.Errorf("cyclehealth: Workspace required")
	}
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}

	report := Report{
		Cycle:       opts.Cycle,
		Workspace:   opts.Workspace,
		GeneratedAt: nowFn(),
		SignalsRun:  signalNames(),
	}

	// Each signal appends zero or more Anomaly entries.
	signals := []signalCheck{
		checkWorkspaceArtifacts,
		checkArtifactSubstance,
		checkLedgerCompleteness,
		checkLedgerTimestamps,
		checkLedgerDuplicates,
		checkHashChain,
		checkArtifactChecksums,
		checkChallengeTokens,
		checkVelocity,
		checkCanaryFiles,
		checkCostEnvelope,
		checkPhaseLatency,
		checkSelfHealEvents,
	}
	for _, sc := range signals {
		report.Anomalies = append(report.Anomalies, sc(opts)...)
	}

	for _, a := range report.Anomalies {
		if a.Severity == SeverityFatal {
			report.OverallFatal = true
			break
		}
	}

	if err := writeReport(opts.Workspace, report); err != nil {
		return report, fmt.Errorf("cyclehealth: write report: %w", err)
	}
	return report, nil
}

// signalCheck is the per-signal contract. Each returns the anomalies
// it found (possibly empty); never errors — file-not-found and
// parse-failure are surfaced as anomalies, not Go errors.
type signalCheck func(opts Options) []Anomaly

func signalNames() []string {
	return []string{
		"workspace_artifacts",
		"artifact_substance",
		"ledger_completeness",
		"ledger_timestamps",
		"duplicate_ledger",
		"hash_chain",
		"artifact_checksums",
		"challenge_tokens",
		"velocity",
		"canary_files",
		"cost_envelope",
		"phase_latency",
		"self_heal_events",
	}
}

// --- Signal implementations (kept small; each does one thing) ---

// requiredArtifacts is the minimum file set a completed cycle must
// produce. Missing files are fatal because downstream phases depend
// on them.
var requiredArtifacts = []string{"scout-report.md", "build-report.md", "audit-report.md"}

func checkWorkspaceArtifacts(opts Options) []Anomaly {
	var out []Anomaly
	for _, name := range requiredArtifacts {
		p := filepath.Join(opts.Workspace, name)
		if _, err := os.Stat(p); err != nil {
			out = append(out, Anomaly{
				Signal: "workspace_artifacts", Severity: SeverityFatal,
				Message: fmt.Sprintf("required artifact missing: %s", name),
			})
		}
	}
	return out
}

func checkArtifactSubstance(opts Options) []Anomaly {
	var out []Anomaly
	for _, name := range requiredArtifacts {
		p := filepath.Join(opts.Workspace, name)
		b, err := os.ReadFile(p)
		if err != nil {
			continue // missing-artifact already covered by checkWorkspaceArtifacts
		}
		if len(strings.TrimSpace(string(b))) < 100 {
			out = append(out, Anomaly{
				Signal: "artifact_substance", Severity: SeverityFatal,
				Message: fmt.Sprintf("artifact too short to be substantive: %s (%d chars)", name, len(b)),
			})
		}
	}
	return out
}

// ledgerEntry is the subset of ledger.jsonl fields cyclehealth inspects.
type ledgerEntry struct {
	Role      string  `json:"role"`
	Phase     string  `json:"phase"`
	Timestamp int64   `json:"timestamp"`
	Token     string  `json:"token"`
	SHA       string  `json:"sha"`
	PrevHash  string  `json:"prev_hash"`
	EntryHash string  `json:"entry_hash"`
	CostUSD   float64 `json:"cost_usd"`
	Cycle     int     `json:"cycle"`
	Kind      string  `json:"kind,omitempty"`
}

// requiredRoles is the set of subagent roles whose ledger entries are
// expected for a completed cycle. Missing roles are fatal.
var requiredRoles = []string{"scout", "builder", "auditor"}

func loadLedger(workspace string) ([]ledgerEntry, error) {
	// The ledger may live at <workspace>/ledger.jsonl or at the project
	// root. Check the workspace-local path first (cycle-scoped tests
	// keep their own ledger); fall back to a sibling path otherwise.
	candidates := []string{
		filepath.Join(workspace, "ledger.jsonl"),
		filepath.Join(filepath.Dir(workspace), "ledger.jsonl"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var entries []ledgerEntry
		for _, line := range strings.Split(string(b), "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var e ledgerEntry
			if err := json.Unmarshal([]byte(line), &e); err == nil {
				entries = append(entries, e)
			}
		}
		return entries, nil
	}
	return nil, fmt.Errorf("no ledger.jsonl found")
}

func checkLedgerCompleteness(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return []Anomaly{{
			Signal: "ledger_completeness", Severity: SeverityFatal,
			Message: err.Error(),
		}}
	}
	seenRoles := map[string]bool{}
	for _, e := range entries {
		if e.Cycle == opts.Cycle {
			seenRoles[e.Role] = true
		}
	}
	var out []Anomaly
	for _, role := range requiredRoles {
		if !seenRoles[role] {
			out = append(out, Anomaly{
				Signal: "ledger_completeness", Severity: SeverityFatal,
				Message: fmt.Sprintf("ledger missing role: %s", role),
			})
		}
	}
	return out
}

func checkLedgerTimestamps(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	nowFn := opts.NowFn
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn().Unix()
	var prev int64
	for i, e := range entries {
		if e.Cycle != opts.Cycle {
			continue
		}
		if e.Timestamp > now+60 {
			out = append(out, Anomaly{
				Signal: "ledger_timestamps", Severity: SeverityFatal,
				Message: fmt.Sprintf("entry %d has future timestamp %d (now=%d)", i, e.Timestamp, now),
			})
		}
		if prev != 0 && e.Timestamp < prev {
			out = append(out, Anomaly{
				Signal: "ledger_timestamps", Severity: SeverityWarn,
				Message: fmt.Sprintf("entry %d timestamp %d goes backward from %d", i, e.Timestamp, prev),
			})
		}
		prev = e.Timestamp
	}
	return out
}

func checkLedgerDuplicates(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	seen := map[string]int{}
	var out []Anomaly
	for _, e := range entries {
		if e.Cycle != opts.Cycle || e.EntryHash == "" {
			continue
		}
		seen[e.EntryHash]++
	}
	for hash, count := range seen {
		if count > 1 {
			out = append(out, Anomaly{
				Signal: "duplicate_ledger", Severity: SeverityFatal,
				Message: fmt.Sprintf("entry_hash %s appears %d times", hash[:8], count),
			})
		}
	}
	return out
}

func checkHashChain(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	var prevHash string
	for i, e := range entries {
		if e.Cycle != opts.Cycle {
			continue
		}
		if prevHash != "" && e.PrevHash != "" && e.PrevHash != prevHash {
			out = append(out, Anomaly{
				Signal: "hash_chain", Severity: SeverityFatal,
				Message: fmt.Sprintf("entry %d prev_hash mismatch (got %s, expected %s)", i, shortHash(e.PrevHash), shortHash(prevHash)),
			})
		}
		if e.EntryHash != "" {
			prevHash = e.EntryHash
		}
	}
	return out
}

func checkArtifactChecksums(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	for _, e := range entries {
		if e.Cycle != opts.Cycle || e.SHA == "" {
			continue
		}
		// SHA is the artifact's content hash; the ledger records it
		// per phase. Re-hash to confirm the file wasn't mutated post-
		// audit. Artifact name = <phase>-report.md by convention.
		p := filepath.Join(opts.Workspace, e.Phase+"-report.md")
		actual, err := sha256File(p)
		if err != nil {
			continue
		}
		if actual != e.SHA {
			out = append(out, Anomaly{
				Signal: "artifact_checksums", Severity: SeverityFatal,
				Message: fmt.Sprintf("%s mutated post-audit (sha %s, ledger %s)", filepath.Base(p), shortHash(actual), shortHash(e.SHA)),
			})
		}
	}
	return out
}

func checkChallengeTokens(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	seen := map[string]int{}
	var out []Anomaly
	for _, e := range entries {
		if e.Cycle != opts.Cycle || e.Token == "" {
			continue
		}
		seen[e.Token]++
	}
	for token, count := range seen {
		if count > 1 {
			out = append(out, Anomaly{
				Signal: "challenge_tokens", Severity: SeverityFatal,
				Message: fmt.Sprintf("challenge token %s reused %d times", shortHash(token), count),
			})
		}
	}
	return out
}

const maxPhaseDurationSec = 1800 // 30 min — beyond this, phase likely stuck

func checkVelocity(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	// Pair consecutive entries within the same cycle; warn when the
	// gap between them exceeds maxPhaseDurationSec.
	var prev *ledgerEntry
	for i := range entries {
		e := entries[i]
		if e.Cycle != opts.Cycle {
			continue
		}
		if prev != nil && e.Timestamp-prev.Timestamp > maxPhaseDurationSec {
			out = append(out, Anomaly{
				Signal: "velocity", Severity: SeverityWarn,
				Message: fmt.Sprintf("phase gap %s→%s = %ds (> %ds)", prev.Role, e.Role, e.Timestamp-prev.Timestamp, maxPhaseDurationSec),
			})
		}
		prev = &e
	}
	return out
}

func checkCanaryFiles(opts Options) []Anomaly {
	entries, err := os.ReadDir(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "canary-") && !strings.HasSuffix(name, fmt.Sprintf("cycle-%d", opts.Cycle)) {
			out = append(out, Anomaly{
				Signal: "canary_files", Severity: SeverityWarn,
				Message: fmt.Sprintf("orphan canary from another cycle: %s", name),
			})
		}
	}
	return out
}

const defaultCostCeilingUSD = 5.00

func checkCostEnvelope(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	for _, e := range entries {
		if e.Cycle != opts.Cycle {
			continue
		}
		if e.CostUSD > defaultCostCeilingUSD {
			out = append(out, Anomaly{
				Signal: "cost_envelope", Severity: SeverityWarn,
				Message: fmt.Sprintf("%s phase cost %.2f > ceiling %.2f", e.Phase, e.CostUSD, defaultCostCeilingUSD),
			})
		}
	}
	return out
}

type phaseTimingEntry struct {
	Phase        string  `json:"phase"`
	DurationMS   int64   `json:"duration_ms"`
	Verdict      string  `json:"verdict"`
	CostUSD      float64 `json:"cost_usd"`
	AttemptCount int     `json:"attempt_count"`
}

func checkPhaseLatency(opts Options) []Anomaly {
	p := filepath.Join(opts.Workspace, "phase-timing.json")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var entries []phaseTimingEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return []Anomaly{
			{
				Signal: "phase_latency", Severity: SeverityWarn,
				Message: fmt.Sprintf("failed to parse phase-timing.json: %v", err),
			},
		}
	}

	ceilingSec := 900 // 15 min default
	if val := os.Getenv("EVOLVE_PHASE_LATENCY_CEILING_S"); val != "" {
		if num, err := strconv.Atoi(val); err == nil && num > 0 {
			ceilingSec = num
		}
	}
	ceilingMS := int64(ceilingSec) * 1000

	var out []Anomaly
	for _, entry := range entries {
		if entry.DurationMS > ceilingMS {
			out = append(out, Anomaly{
				Signal: "phase_latency", Severity: SeverityWarn,
				Message: fmt.Sprintf("%s phase latency %dms (> %dms)", entry.Phase, entry.DurationMS, ceilingMS),
			})
		}
	}
	return out
}

func checkSelfHealEvents(opts Options) []Anomaly {
	entries, err := loadLedger(opts.Workspace)
	if err != nil {
		return nil
	}
	var out []Anomaly
	for _, e := range entries {
		if e.Cycle != opts.Cycle {
			continue
		}
		if e.Kind == "phase_retry" || e.Kind == "backfill" {
			out = append(out, Anomaly{
				Signal:   "self_heal_events",
				Severity: SeverityWarn,
				Message:  fmt.Sprintf("%s event occurred in phase %s", e.Kind, e.Phase),
			})
		}
	}
	return out
}

// --- Helpers ---

func sha256File(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func shortHash(s string) string {
	if len(s) <= 8 {
		return s
	}
	return s[:8]
}

func writeReport(workspace string, r Report) error {
	out := filepath.Join(workspace, "cycle-health.json")
	tmp := fmt.Sprintf("%s.tmp.%d", out, os.Getpid())
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, out)
}
