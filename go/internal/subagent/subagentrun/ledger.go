package subagentrun

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/ledgertip"
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
		JSONStringEscape(e.ArtifactSHA256),
		JSONStringEscape(e.ChallengeToken),
		JSONStringEscape(e.GitHEAD),
		JSONStringEscape(e.TreeStateSHA),
		entrySeq,
		JSONStringEscape(prevHash),
		JSONStringEscape(e.QualityTier),
	)
}

func (d *Dispatcher) appendLedger(path string, e ledgerEntry) (op string, err error) {
	ts := d.now().UTC().Format("2006-01-02T15:04:05Z")
	return AppendChainedLine(path, func(entrySeq int, prevHash string) string {
		return renderLedgerLine(e, ts, entrySeq, prevHash)
	}, d.openLedger)
}

func AppendChainedLine(path string, render func(entrySeq int, prevHash string) string, open func(string) (io.WriteCloser, error)) (op string, err error) {
	if open == nil {
		open = openAppend
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "mkdir", err
	}
	real := realLedgerPath(path)
	release, err := flock.Lock(filepath.Join(filepath.Dir(real), "ledger.lock"))
	if err != nil {
		return "flock", err
	}
	defer release()
	tipPath := filepath.Join(filepath.Dir(real), "ledger.tip")
	prevHash, entrySeq, err := tipChainLink(tipPath, real)
	if err != nil {
		return "chain_link", err
	}
	line := render(entrySeq, prevHash)
	f, err := open(real)
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
	return replaceTip(tipPath, ledgertip.Format(entrySeq, SHA256Hex(line)))
}

func tipChainLink(tipPath, ledgerPath string) (prevHash string, entrySeq int, err error) {
	raw, err := os.ReadFile(tipPath)
	if errors.Is(err, fs.ErrNotExist) {
		return ChainLink(ledgerPath)
	}
	if err != nil {
		return "", 0, err
	}
	seqText, hash, ok := strings.Cut(strings.TrimSpace(string(raw)), ":")
	seq, convErr := strconv.Atoi(seqText)
	if !ok || convErr != nil || hash == "" {
		return "", 0, fmt.Errorf("malformed ledger tip %s: %q", tipPath, raw)
	}
	return hash, seq + 1, nil
}

func realLedgerPath(path string) string {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}

func replaceTip(tipPath, tip string) (op string, err error) {
	tmp := fmt.Sprintf("%s.tmp.%d", tipPath, os.Getpid())
	if err := os.WriteFile(tmp, []byte(tip), 0o644); err != nil {
		_ = os.Remove(tmp)
		return "tip_tmp", err
	}
	if err := os.Rename(tmp, tipPath); err != nil {
		_ = os.Remove(tmp)
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
