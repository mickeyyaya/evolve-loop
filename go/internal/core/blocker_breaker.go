package core

// blocker_breaker.go — mid-batch pipeline-blocker breaker (ADR-0072 extension,
// operator directive 2026-07-22: a pipeline blocker must be fixed directly,
// never passed to following cycles). The ADR-0072 floor halts on FORGED
// verdicts; this breaker halts on the other blocker signature — the same
// failure identity recurring across a batch's cycles, which honest-looking
// per-cycle FAILs never surface on their own (batch-5 burned six cycles on one
// class; the 862–899 storm burned 37 on byte-identical defect strings).
//
// Two deterministic rules over the S1 failure digests (failure_digest.go),
// evaluated batch-scoped by the loop after every iteration:
//
//	Rule A "guard-class"           — guard-abort digests ≥ GuardClassCeiling.
//	                                 A guard abort is pipeline machinery
//	                                 failing by construction, never task-legit.
//	Rule B "identical-fingerprint" — one exact fingerprint ≥
//	                                 IdenticalFingerprintCeiling. Identical
//	                                 failure identities cannot be distinct
//	                                 honest defects.
//
// Same-task repeats are S5 quarantine's job (task_retry_ceiling) — the breaker
// is task-agnostic so a healthy batch of many DIFFERENT honest rejections
// (batch-2's shape) never trips it. A zero ceiling disables its rule (the
// policy escape hatch, mirroring the positive-overrides-win threshold merge).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// BlockerBreakerConfig carries the policy ceilings (policy.FailureThresholds
// GuardClassHaltCeiling / IdenticalFingerprintHaltCeiling at the composition
// root). Zero disables the rule.
type BlockerBreakerConfig struct {
	GuardClassCeiling           int
	IdenticalFingerprintCeiling int
	// UnexplainedCeiling halts when this many digests carry NO machine-
	// readable failure reason (the degenerate empty-evidence bucket) — a
	// diagnosability breakdown, deliberately named apart from the identical-
	// fingerprint rule (batch-6: three DIFFERENT failures shared one empty
	// fingerprint).
	UnexplainedCeiling int
	// AckedFingerprints excludes acknowledged fingerprints from Rule B's
	// identical-fingerprint count (the .evolve/resolved-fingerprints.json
	// ledger — LoadResolvedFingerprints/AppendResolvedFingerprint). This is
	// the fix for the cycle-1329 recurrence: a fingerprint whose root cause
	// was already diagnosed and consumed kept re-tripping the breaker on
	// every relaunch because the breaker had no memory across invocations.
	// The exclusion is scoped to ONE named fingerprint at a time, never a
	// blanket Rule B disable — a different, unacked fingerprint still halts
	// at the ceiling unchanged. Nil/empty = no exclusions (zero value,
	// byte-identical behavior for every pre-existing caller).
	AckedFingerprints map[string]bool
	// ConsecutiveFailuresCeiling halts when this many CYCLES fail
	// back-to-back (cycle numbers n, n+1, …, no PASS between) REGARDLESS of
	// fingerprint identity (operator directive 2026-08-10: the 2026-08-09
	// batch burned 10 failed cycles / 0 ships before the identity-keyed rule
	// tripped — varied failure modes evaded it for 7 extra cycles). Acked
	// digests break the streak: a batch resumed to verify a fix must not
	// insta-halt on the history it is verifying. 0 disables. Evaluated LAST
	// so the specific rules above, whose reasons carry actionable repro
	// hints, name the halt when they also trip.
	ConsecutiveFailuresCeiling int
}

// ResolvedFingerprint is one record in the ack ledger
// (.evolve/resolved-fingerprints.json) — an append-only JSON array an
// operator (or, eventually, transactional inbox consumption) writes to mark
// a failure fingerprint's root cause diagnosed and fixed, so the blocker
// breaker stops re-halting on it.
type ResolvedFingerprint struct {
	Fingerprint string `json:"fingerprint"`
	ResolvedAt  string `json:"resolved_at"`
	ResolvedBy  string `json:"resolved_by"`
}

// resolvedFingerprintsFile is the ledger's filename under evolveDir.
const resolvedFingerprintsFile = "resolved-fingerprints.json"

// LoadResolvedFingerprints reads the ack ledger and returns the set of
// acknowledged fingerprints. A missing file returns an empty set with NO
// error — fail-open, mirroring CollectBatchFailureDigests' own tolerance for
// a healthy run that never wrote one.
func LoadResolvedFingerprints(evolveDir string) (map[string]bool, error) {
	raw, err := os.ReadFile(filepath.Join(evolveDir, resolvedFingerprintsFile))
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]bool{}, nil
		}
		return nil, fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	var records []ResolvedFingerprint
	if err := json.Unmarshal(raw, &records); err != nil {
		return nil, fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	out := make(map[string]bool, len(records))
	for _, r := range records {
		if r.Fingerprint != "" {
			out[r.Fingerprint] = true
		}
	}
	return out, nil
}

// AppendResolvedFingerprint atomically appends one ack record to the ledger,
// creating it if absent. Read-modify-write followed by a tmp+rename atomic
// write (the project's shell-convention `mv "${f}.tmp.$$"` mirrored in Go:
// os.Rename is atomic on the same filesystem) — no reader ever observes a
// partially-written ledger.
func AppendResolvedFingerprint(evolveDir, fingerprint, resolvedBy string, resolvedAt time.Time) error {
	if strings.TrimSpace(fingerprint) == "" {
		return fmt.Errorf("AppendResolvedFingerprint: fingerprint must not be empty")
	}
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	path := filepath.Join(evolveDir, resolvedFingerprintsFile)
	var records []ResolvedFingerprint
	if raw, err := os.ReadFile(path); err == nil {
		if uerr := json.Unmarshal(raw, &records); uerr != nil {
			return fmt.Errorf("resolved-fingerprints.json: %w", uerr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	records = append(records, ResolvedFingerprint{
		Fingerprint: fingerprint,
		ResolvedAt:  resolvedAt.UTC().Format(time.RFC3339),
		ResolvedBy:  resolvedBy,
	})
	buf, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	return nil
}

// consumptionFingerprintRe anchors on the literal `fingerprint` token
// (never a bare pipe-delimited substring — a `docs/a|b|c.md` path or any
// other triplet-shaped text with no `fingerprint` keyword must not match)
// followed by an optional `=`, an optional opening quote, and the
// pipe|pipe|pipe triplet shape every failure fingerprint takes
// (failure_digest.go). Matches both consumed_by's unquoted
// `fingerprint ship|unknown|76d0f4fca190` and notes' quoted
// `fingerprint "ship|unknown|76d0f4fca190"` shapes with one pattern.
var consumptionFingerprintRe = regexp.MustCompile(`fingerprint\s*=?\s*"?([A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+)"?`)

// ParseConsumptionFingerprint extracts a failure fingerprint from free text
// (a pipeline-defect item's consumed_by narrative or auto-filed notes
// field). Fails closed (ok=false) when no `fingerprint <triplet>` token is
// present — it never guesses from a pipe-delimited substring alone.
func ParseConsumptionFingerprint(text string) (string, bool) {
	m := consumptionFingerprintRe.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ConsumePipelineDefectFingerprint is the single writer path for
// transactional inbox consumption acking the blocker-breaker ledger: it
// extracts the fingerprint from consumedBy, falling back to notes when
// consumedBy carries none, and appends it to resolved-fingerprints.json via
// AppendResolvedFingerprint. Returns an error — and writes NOTHING to the
// ledger — when neither field carries a parseable fingerprint, so a P0
// consumed with no diagnosable fingerprint is never silently swallowed into
// the ledger as an empty/garbage entry.
func ConsumePipelineDefectFingerprint(evolveDir, consumedBy, notes, resolvedBy string, resolvedAt time.Time) (string, error) {
	fp, ok := ParseConsumptionFingerprint(consumedBy)
	if !ok {
		fp, ok = ParseConsumptionFingerprint(notes)
	}
	if !ok {
		return "", fmt.Errorf("ConsumePipelineDefectFingerprint: no fingerprint token found in consumed_by or notes")
	}
	if err := AppendResolvedFingerprint(evolveDir, fp, resolvedBy, resolvedAt); err != nil {
		return "", err
	}
	return fp, nil
}

// BlockerVerdict is the breaker's decision. Halt=true means the batch must
// stop and escalate (P0 pipeline-repair) instead of dispatching another cycle
// into the same wall.
type BlockerVerdict struct {
	Halt        bool
	Rule        string // "guard-class" | "identical-fingerprint" | "unexplained-failures" | "consecutive-failures"
	Fingerprint string // Rule B: the recurring identity; Rule A: representative
	Count       int
	Reason      string
}

// guardAbortClass is the failure_digest pre-class bucket that is never
// task-legit (statemap severing, tree-guard aborts).
const guardAbortClass = "guard-abort"

// isUnexplainedDigest reports a digest whose fingerprint asserts NO defect
// identity. Two shapes: the degenerate empty-evidence digest (no reason
// artifact — phase and pre-class degraded to unknown), and the self-marked
// content-free digest (reasons were empty or pure agent-graded router
// boilerplate; batch-14: three DISTINCT auditor findings shared one
// boilerplate fingerprint and false-tripped the identical rule). These MUST
// NOT count as "identical" defects — distinct failures collapse into these
// buckets by construction; the UnexplainedCeiling rule owns them under its
// honest diagnosability-breakdown name.
func isUnexplainedDigest(d FailureDigest) bool {
	return d.Unexplained || (d.PreClass == "unknown" && strings.HasPrefix(d.Fingerprint, "|unknown|"))
}

// EvaluateBlockerBreaker applies the two rules over a batch's failure digests.
// Pure and deterministic: same digests + config in, same verdict out.
func EvaluateBlockerBreaker(digests []FailureDigest, cfg BlockerBreakerConfig) BlockerVerdict {
	if cfg.GuardClassCeiling > 0 {
		var guard []FailureDigest
		for _, d := range digests {
			if d.PreClass == guardAbortClass {
				guard = append(guard, d)
			}
		}
		if len(guard) >= cfg.GuardClassCeiling {
			return BlockerVerdict{
				Halt: true, Rule: "guard-class", Fingerprint: guard[0].Fingerprint, Count: len(guard),
				Reason: fmt.Sprintf("%d %s-class failures in one batch (ceiling %d) — guard aborts are pipeline machinery failing, never task defects", len(guard), guardAbortClass, cfg.GuardClassCeiling),
			}
		}
	}
	if cfg.UnexplainedCeiling > 0 {
		var unexplained int
		for _, d := range digests {
			if isUnexplainedDigest(d) {
				unexplained++
			}
		}
		if unexplained >= cfg.UnexplainedCeiling {
			return BlockerVerdict{
				Halt: true, Rule: "unexplained-failures", Count: unexplained,
				Reason: fmt.Sprintf("%d failures in one batch produced no machine-readable failure reason (ceiling %d) — a diagnosability breakdown: fix the missing reason-writers, then diagnose the underlying failures individually", unexplained, cfg.UnexplainedCeiling),
			}
		}
	}
	if cfg.IdenticalFingerprintCeiling > 0 {
		counts := map[string]int{}
		for _, d := range digests {
			if d.Fingerprint == "" || isUnexplainedDigest(d) {
				continue
			}
			if cfg.AckedFingerprints[d.Fingerprint] {
				continue
			}
			counts[d.Fingerprint]++
			if counts[d.Fingerprint] >= cfg.IdenticalFingerprintCeiling {
				return BlockerVerdict{
					Halt: true, Rule: "identical-fingerprint", Fingerprint: d.Fingerprint, Count: counts[d.Fingerprint],
					Reason: fmt.Sprintf("failure fingerprint %q recurred %d× in one batch (ceiling %d) — identical failure identities cannot be distinct honest defects", d.Fingerprint, counts[d.Fingerprint], cfg.IdenticalFingerprintCeiling),
				}
			}
		}
	}
	if cfg.ConsecutiveFailuresCeiling > 0 {
		counted := map[int]FailureDigest{}
		for _, d := range digests {
			if cfg.AckedFingerprints[d.Fingerprint] {
				continue
			}
			counted[d.Cycle] = d
		}
		cycles := make([]int, 0, len(counted))
		for c := range counted {
			cycles = append(cycles, c)
		}
		sort.Ints(cycles)
		run := 0
		for i, c := range cycles {
			if i > 0 && c == cycles[i-1]+1 {
				run++
			} else {
				run = 1
			}
			if run >= cfg.ConsecutiveFailuresCeiling {
				d := counted[c]
				return BlockerVerdict{
					Halt: true, Rule: "consecutive-failures", Fingerprint: d.Fingerprint, Count: run,
					Reason: fmt.Sprintf("%d consecutive failed cycles ending at cycle %d (ceiling %d) with mixed failure identities — a batch that cannot ship %d cycles in a row is pipeline-degraded regardless of fingerprint; stop, deep-dive the failures individually, fix, then resume (operator directive 2026-08-10)", run, d.Cycle, cfg.ConsecutiveFailuresCeiling, cfg.ConsecutiveFailuresCeiling),
				}
			}
		}
	}
	return BlockerVerdict{}
}

// CollectBatchFailureDigests reads every <evolveDir>/runs/cycle-N/
// failure-digest.json with N >= fromCycle. Missing or malformed digests are
// skipped silently — a PASS cycle writes none, and one corrupt artifact must
// not disable the breaker for the rest of the batch.
func CollectBatchFailureDigests(evolveDir string, fromCycle int) []FailureDigest {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var out []FailureDigest
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !strings.HasPrefix(name, "cycle-") {
			continue
		}
		n, cerr := strconv.Atoi(strings.TrimPrefix(name, "cycle-"))
		if cerr != nil || n < fromCycle {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(evolveDir, "runs", name, "failure-digest.json"))
		if rerr != nil {
			continue
		}
		var d FailureDigest
		if json.Unmarshal(raw, &d) != nil {
			continue
		}
		out = append(out, d)
	}
	return out
}
