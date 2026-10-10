package subagent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/subagent/subagentrun"
)

type AbnormalEvent struct {
	EventType       string
	Severity        string
	Details         string
	RemediationHint string
	SourcePhase     string
}

func AppendAbnormalEvent(workspace string, ev AbnormalEvent, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}
	if info, err := os.Stat(workspace); err != nil || !info.IsDir() {
		return nil
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
		return nil
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(line + "\n"); err != nil {
		return nil
	}
	return nil
}

type FanoutLedgerEntry struct {
	Cycle          int
	Agent          string
	ChallengeToken string
	GitHEAD        string
	TreeStateSHA   string
	WorkerNames    []string
	WorkerCount    int
	ExitCode       int
	AggregatePath  string
	QualityTier    string
}

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

	workersJSON, err := json.Marshal(e.WorkerNames)
	if err != nil {
		return fmt.Errorf("subagent/helpers: marshal workers: %w", err)
	}

	quality := e.QualityTier
	if quality == "" {
		quality = "unknown"
	}

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

const ledgerZeroSeed = subagentrun.LedgerZeroSeed

func readChainLink(ledgerPath string) (prevHash string, entrySeq int, err error) {
	return subagentrun.ChainLink(ledgerPath)
}

func sha256Hex(s string) string { return subagentrun.SHA256Hex(s) }

func jsonStringEscape(s string) string { return subagentrun.JSONStringEscape(s) }
