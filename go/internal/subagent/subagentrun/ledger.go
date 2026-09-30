package subagentrun

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ledgerEntry struct {
	Cycle          int
	Role           string
	Model          string
	ExitCode       int
	DurationS      string
	ArtifactPath   string
	ArtifactSHA256 string
	ChallengeToken string
	GitHEAD        string
	TreeStateSHA   string
	QualityTier    string
	RunID          string
}

const LedgerZeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

func renderLedgerLine(e ledgerEntry, ts string, entrySeq int, prevHash string) string {
	runIDField := ""
	if e.RunID != "" {
		runIDField = `"run_id":"` + JSONStringEscape(e.RunID) + `",`
	}
	return fmt.Sprintf(
		`{"ts":"%s","cycle":%d,%s"role":"%s","kind":"agent_subprocess","model":"%s","exit_code":%d,`+
			`"duration_s":"%s","artifact_path":"%s","artifact_sha256":"%s","challenge_token":"%s",`+
			`"git_head":"%s","tree_state_sha":"%s","entry_seq":%d,"prev_hash":"%s","quality_tier":"%s","cli_resolution":null}`,
		JSONStringEscape(ts),
		e.Cycle,
		runIDField,
		JSONStringEscape(e.Role),
		JSONStringEscape(e.Model),
		e.ExitCode,
		JSONStringEscape(e.DurationS),
		JSONStringEscape(e.ArtifactPath),
		e.ArtifactSHA256,
		JSONStringEscape(e.ChallengeToken),
		JSONStringEscape(e.GitHEAD),
		JSONStringEscape(e.TreeStateSHA),
		entrySeq,
		prevHash,
		JSONStringEscape(e.QualityTier),
	)
}

func (d *Dispatcher) appendLedger(path string, e ledgerEntry) (op string, err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "mkdir", err
	}
	prevHash, entrySeq, err := ChainLink(path)
	if err != nil {
		return "chain_link", err
	}
	line := renderLedgerLine(e, d.now().UTC().Format("2006-01-02T15:04:05Z"), entrySeq, prevHash)
	f, err := d.openLedger(path)
	if err != nil {
		return "open", err
	}
	if _, err := io.WriteString(f, line+"\n"); err != nil {
		_ = f.Close()
		return "write", err
	}
	if err := f.Close(); err != nil {
		return "close", err
	}
	tipPath := filepath.Join(filepath.Dir(path), "ledger.tip")
	tip := fmt.Sprintf("%d:%s\n", entrySeq, SHA256Hex(line))
	tmp := tipPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(tip), 0o644); err != nil {
		_ = os.Remove(tmp)
		return "tip_tmp", err
	}
	if err := os.Rename(tmp, tipPath); err != nil {
		return "tip_rename", err
	}
	return "", nil
}

func openAppend(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

func ChainLink(ledgerPath string) (prevHash string, entrySeq int, err error) {
	prevHash = LedgerZeroSeed
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
	prevHash = SHA256Hex(last)
	entrySeq = len(lines)
	return prevHash, entrySeq, nil
}

func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func JSONStringEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

func QualityTier(budgetNative, permissionScoping bool) string {
	if budgetNative && permissionScoping {
		return "full"
	}
	if !budgetNative && !permissionScoping {
		return "degraded"
	}
	return "hybrid"
}
