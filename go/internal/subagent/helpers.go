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

var abnormalEventWrite = func(f *os.File, s string) (int, error) { return f.WriteString(s) }

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
	if _, err := abnormalEventWrite(f, line+"\n"); err != nil {
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

var marshalWorkers = json.Marshal

func WriteFanoutLedgerEntry(ledgerPath string, e FanoutLedgerEntry, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}

	artifactSHA := ""
	if e.AggregatePath != "" {
		if data, err := os.ReadFile(e.AggregatePath); err == nil {
			sum := sha256.Sum256(data)
			artifactSHA = hex.EncodeToString(sum[:])
		}
	}

	workersJSON, err := marshalWorkers(e.WorkerNames)
	if err != nil {
		return fmt.Errorf("subagent/helpers: marshal workers: %w", err)
	}

	quality := e.QualityTier
	if quality == "" {
		quality = "unknown"
	}

	ts := now().UTC().Format("2006-01-02T15:04:05Z")
	render := func(entrySeq int, prevHash string) string {
		return fmt.Sprintf(
			`{"ts":"%s","cycle":%d,"role":"%s","kind":"agent_fanout","exit_code":%d,`+
				`"artifact_path":"%s","artifact_sha256":"%s","challenge_token":"%s",`+
				`"git_head":"%s","tree_state_sha":"%s","worker_count":%d,"workers":%s,`+
				`"entry_seq":%d,"prev_hash":"%s","quality_tier":"%s"}`,
			jsonStringEscape(ts),
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
			jsonStringEscape(prevHash),
			jsonStringEscape(quality),
		)
	}
	if op, err := subagentrun.AppendChainedLine(ledgerPath, render, nil); err != nil {
		return fmt.Errorf("subagent/helpers: ledger %s: %w", op, err)
	}
	return nil
}

func jsonStringEscape(s string) string { return subagentrun.JSONStringEscape(s) }
