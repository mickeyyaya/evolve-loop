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

type BlockerBreakerConfig struct {
	GuardClassCeiling           int
	IdenticalFingerprintCeiling int
	UnexplainedCeiling          int
	AckedFingerprints           map[string]bool
	ConsecutiveFailuresCeiling  int
	LaneDeferredCycles          map[int]bool
}

type ResolvedFingerprint struct {
	Fingerprint string `json:"fingerprint"`
	ResolvedAt  string `json:"resolved_at"`
	ResolvedBy  string `json:"resolved_by"`
}

const resolvedFingerprintsFile = "resolved-fingerprints.json"

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
	buf, err := marshalIndentJSON(records, "", "  ")
	if err != nil {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	if err := writeArtifactAtomically(path, buf); err != nil {
		return fmt.Errorf("resolved-fingerprints.json: %w", err)
	}
	return nil
}

var consumptionFingerprintRe = regexp.MustCompile(`fingerprint\s*=?\s*"?([A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+\|[A-Za-z0-9_.\-]+)"?`)

func ParseConsumptionFingerprint(text string) (string, bool) {
	m := consumptionFingerprintRe.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

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

type BlockerVerdict struct {
	Halt        bool
	Rule        string
	Fingerprint string
	Count       int
	Reason      string
}

const guardAbortClass = "guard-abort"

func isUnexplainedDigest(d FailureDigest) bool {
	return d.Unexplained || (d.PreClass == "unknown" && strings.HasPrefix(d.Fingerprint, "|unknown|"))
}

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
		if at, run, ok := consecutiveRun(cycles, cfg.ConsecutiveFailuresCeiling, cfg.LaneDeferredCycles); ok {
			d := counted[cycles[at]]
			return BlockerVerdict{
				Halt: true, Rule: "consecutive-failures", Fingerprint: d.Fingerprint, Count: run,
				Reason: fmt.Sprintf("%d consecutive failed cycles ending at cycle %d (ceiling %d) with mixed failure identities — a batch that cannot ship %d cycles in a row is pipeline-degraded regardless of fingerprint; stop, deep-dive the failures individually, fix, then resume (operator directive 2026-08-10)", run, d.Cycle, cfg.ConsecutiveFailuresCeiling, cfg.ConsecutiveFailuresCeiling),
			}
		}
	}
	return BlockerVerdict{}
}

func consecutiveRun(sortedCycles []int, ceiling int, transparent map[int]bool) (at, run int, ok bool) {
	for i, c := range sortedCycles {
		if i > 0 && adjacentOver(sortedCycles[i-1], c, transparent) {
			run++
		} else {
			run = 1
		}
		if run >= ceiling {
			return i, run, true
		}
	}
	return 0, 0, false
}

func CollectBatchFailureDigests(evolveDir string, fromCycle int) []FailureDigest {
	entries, err := os.ReadDir(filepath.Join(evolveDir, "runs"))
	if err != nil {
		return nil
	}
	var out []FailureDigest
	for _, e := range entries {
		if _, inBatch := batchCycleNumber(e, fromCycle); !inBatch {
			continue
		}
		raw, rerr := os.ReadFile(filepath.Join(evolveDir, "runs", e.Name(), "failure-digest.json"))
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

func batchCycleNumber(e os.DirEntry, fromCycle int) (int, bool) {
	name := e.Name()
	if !e.IsDir() || !strings.HasPrefix(name, "cycle-") {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(name, "cycle-"))
	if err != nil || n < fromCycle {
		return 0, false
	}
	return n, true
}

func adjacentOver(prev, next int, transparent map[int]bool) bool {
	for n := prev + 1; n < next; n++ {
		if !transparent[n] {
			return false
		}
	}
	return true
}
