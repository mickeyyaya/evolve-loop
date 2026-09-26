package core

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

// BlockerBreakerConfig carries the breaker's policy ceilings; a zero ceiling disables its rule.
type BlockerBreakerConfig struct {
	GuardClassCeiling           int
	IdenticalFingerprintCeiling int
	// UnexplainedCeiling halts on this many digests that carry no machine-readable failure reason.
	UnexplainedCeiling int
	// AckedFingerprints excludes each acknowledged fingerprint from Rule B's count, never Rule B as a whole.
	AckedFingerprints map[string]bool
	// ConsecutiveFailuresCeiling halts on this many back-to-back failed cycles, whatever their fingerprints.
	ConsecutiveFailuresCeiling int
}

// ResolvedFingerprint is one record in the append-only ack ledger, .evolve/resolved-fingerprints.json.
type ResolvedFingerprint struct {
	Fingerprint string `json:"fingerprint"`
	ResolvedAt  string `json:"resolved_at"`
	ResolvedBy  string `json:"resolved_by"`
}

const resolvedFingerprintsFile = "resolved-fingerprints.json"

// LoadResolvedFingerprints returns the acknowledged fingerprints; a missing ledger is an empty set, not an error.
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

// AppendResolvedFingerprint appends one ack record through a tmp+rename write, so no reader sees a partial ledger.
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

// consumptionFingerprintRe requires the literal `fingerprint` token before the triplet, so a pipe-delimited path never matches.
var consumptionFingerprintRe = regexp.MustCompile(`fingerprint\s*=?\s*"?([A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+)"?`)

// ParseConsumptionFingerprint extracts a failure fingerprint from free text; it fails closed without a `fingerprint` token.
func ParseConsumptionFingerprint(text string) (string, bool) {
	m := consumptionFingerprintRe.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// ConsumePipelineDefectFingerprint acks the fingerprint named in consumedBy or notes, and writes nothing if neither names one.
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

// BlockerVerdict is the breaker's decision; Halt stops the batch and escalates instead of dispatching another cycle.
type BlockerVerdict struct {
	Halt        bool
	Rule        string // "guard-class" | "identical-fingerprint" | "unexplained-failures" | "consecutive-failures"
	Fingerprint string // Rule B: the recurring identity; Rule A: representative
	Count       int
	Reason      string
}

// guardAbortClass is the pre-class of guard aborts, which are pipeline machinery failing and never task defects.
const guardAbortClass = "guard-abort"

// isUnexplainedDigest reports a digest whose fingerprint asserts no defect identity. Distinct failures collapse into
// such buckets by construction, so they never count as identical; the unexplained rule owns them.
func isUnexplainedDigest(d FailureDigest) bool {
	return d.Unexplained || (d.PreClass == "unknown" && strings.HasPrefix(d.Fingerprint, "|unknown|"))
}

// EvaluateBlockerBreaker applies the breaker's rules to a batch's failure digests; the first rule that trips names the halt.
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
	// Evaluated last, so a more specific rule names the halt when both trip.
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

// CollectBatchFailureDigests reads runs/cycle-N/failure-digest.json for N >= fromCycle, skipping missing or malformed ones.
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
