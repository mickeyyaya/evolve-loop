package subagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AbnormalEvent mirrors the JSONL schema bash _append_abnormal_event writes
// to workspace/abnormal-events.jsonl. Best-effort; missing workspace dir is
// a no-op (matches bash `[ -d "$_ws" ] || return 0`).
type AbnormalEvent struct {
	EventType       string
	Severity        string
	Details         string
	RemediationHint string
	// SourcePhase is fixed to "subagent-run" in bash. Exposed here so callers
	// who run from a different phase scope can override (e.g. fanout aggregator).
	SourcePhase string
}

// AppendAbnormalEvent writes one event line to <workspace>/abnormal-events.jsonl.
// Returns nil when the workspace doesn't exist (best-effort semantics matching
// bash). Returns an error only when the directory exists but the file write
// fails for a non-skippable reason.
func AppendAbnormalEvent(workspace string, ev AbnormalEvent, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		return nil // bash: silently ignore when workspace dir missing
	}
	sourcePhase := ev.SourcePhase
	if sourcePhase == "" {
		sourcePhase = "subagent-run"
	}
	line := fmt.Sprintf(
		`{"event_type":"%s","timestamp":"%s","source_phase":"%s","severity":"%s","details":"%s","remediation_hint":"%s"}`,
		jsonStringEscape(ev.EventType),
		now().UTC().Format("2006-01-02T15:04:05Z"),
		jsonStringEscape(sourcePhase),
		jsonStringEscape(ev.Severity),
		jsonStringEscape(ev.Details),
		jsonStringEscape(ev.RemediationHint),
	)
	path := filepath.Join(workspace, "abnormal-events.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		// Bash uses `|| true` so failures here are tolerated. Mirror that.
		return nil
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return nil
	}
	return nil
}

// FanoutLedgerEntry is the typed input to WriteFanoutLedgerEntry. Mirrors
// the args of bash _write_fanout_ledger_entry at subagent-run.sh:1635.
type FanoutLedgerEntry struct {
	Cycle          int
	Agent          string
	ChallengeToken string
	GitHEAD        string
	TreeStateSHA   string
	WorkerNames    []string // space-separated in bash; we use a typed slice
	WorkerCount    int
	ExitCode       int
	AggregatePath  string // may be empty when no aggregate produced
	QualityTier    string // default "unknown"
}

// WriteFanoutLedgerEntry appends a single `kind: "agent_fanout"` entry to
// ledger.jsonl + updates ledger.tip atomically. Mirrors bash byte layout:
// fixed JSON field order + hash chain link from the SHA256 of the prior
// line.
func WriteFanoutLedgerEntry(ledgerPath string, e FanoutLedgerEntry, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(filepath.Dir(ledgerPath), 0o755); err != nil {
		return fmt.Errorf("subagent/helpers: mkdir ledger dir: %w", err)
	}

	artifactSHA := ""
	if e.AggregatePath != "" {
		if data, err := os.ReadFile(e.AggregatePath); err == nil {
			sum := sha256.Sum256(data)
			artifactSHA = hex.EncodeToString(sum[:])
		}
	}

	prevHash, entrySeq, err := readChainLink(ledgerPath)
	if err != nil {
		return fmt.Errorf("subagent/helpers: chain link: %w", err)
	}

	// Build workers JSON array preserving order. Bash uses jq -R . | jq -s .
	// which round-trips through string then re-arrays; we just marshal once.
	workersJSON, err := json.Marshal(e.WorkerNames)
	if err != nil {
		return fmt.Errorf("subagent/helpers: marshal workers: %w", err)
	}

	quality := e.QualityTier
	if quality == "" {
		quality = "unknown"
	}

	// Field order MUST match bash jq object construction at
	// subagent-run.sh:1683-1690. Stable order is required because downstream
	// verifiers + ledgerverify chain-link both hash the line.
	line := fmt.Sprintf(
		`{"ts":"%s","cycle":%d,"role":"%s","kind":"agent_fanout","exit_code":%d,`+
			`"artifact_path":"%s","artifact_sha256":"%s","challenge_token":"%s",`+
			`"git_head":"%s","tree_state_sha":"%s","worker_count":%d,"workers":%s,`+
			`"entry_seq":%d,"prev_hash":"%s","quality_tier":"%s"}`,
		jsonStringEscape(now().UTC().Format("2006-01-02T15:04:05Z")),
		e.Cycle,
		jsonStringEscape(e.Agent),
		e.ExitCode,
		jsonStringEscape(e.AggregatePath),
		artifactSHA,
		jsonStringEscape(e.ChallengeToken),
		jsonStringEscape(e.GitHEAD),
		jsonStringEscape(e.TreeStateSHA),
		e.WorkerCount,
		workersJSON,
		entrySeq,
		prevHash,
		jsonStringEscape(quality),
	)

	f, err := os.OpenFile(ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("subagent/helpers: open ledger: %w", err)
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		_ = f.Close()
		return fmt.Errorf("subagent/helpers: write line: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("subagent/helpers: close ledger: %w", err)
	}

	// Update tip atomically: <seq>:<sha256-of-new-line>\n
	tipPath := filepath.Join(filepath.Dir(ledgerPath), "ledger.tip")
	tip := fmt.Sprintf("%d:%s\n", entrySeq, sha256Hex(line))
	tmp := tipPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(tip), 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("subagent/helpers: write tip tmp: %w", err)
	}
	if err := os.Rename(tmp, tipPath); err != nil {
		return fmt.Errorf("subagent/helpers: rename tip: %w", err)
	}
	return nil
}

// --- internal helpers ---

const ledgerZeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// readChainLink mirrors _ledger_chain_link at subagent-run.sh:355. Same
// semantics as the cyclesimulator copy — kept local to avoid widening the
// internal/cyclesimulator export surface.
func readChainLink(ledgerPath string) (prevHash string, entrySeq int, err error) {
	prevHash = ledgerZeroSeed
	entrySeq = 0
	info, statErr := os.Stat(ledgerPath)
	if statErr != nil || info.Size() == 0 {
		return prevHash, entrySeq, nil
	}
	data, rerr := os.ReadFile(ledgerPath)
	if rerr != nil {
		return "", 0, rerr
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return prevHash, entrySeq, nil
	}
	last := lines[len(lines)-1]
	prevHash = sha256Hex(last)
	entrySeq = len(lines)
	return prevHash, entrySeq, nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// jsonStringEscape handles the subset bash escapes (only "). We expand to
// quote + backslash for safety. Newlines are unlikely in event fields; if
// callers do supply them, Go's json.Marshal of a string would be the right
// answer — keeping this for one-line JSONL determinism.
func jsonStringEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}
