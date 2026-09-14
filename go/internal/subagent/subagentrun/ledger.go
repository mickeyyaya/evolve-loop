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

// ledgerEntry is the agent_subprocess record (unexported until the fan-out
// writer folds onto this file — follow-up 16-1). RunID is the CA.5 run
// identity: empty when it cannot be resolved from the run workspace, in which
// case the key is OMITTED from the line (parity with core.LedgerEntry's
// `json:"run_id,omitempty"`), never written as "".
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

// LedgerZeroSeed is prev_hash of the first entry of a chain.
const LedgerZeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// renderLedgerLine renders the one-line JSON record. Field order matches the
// retired bash writer — chain hash determinism depends on it. The run_id
// fragment is passed as an ARGUMENT (%s), never concatenated into the format
// string: fmt does not rescan arguments for verbs, but it does rescan the
// format, so a '%' inside a spliced value would consume the next argument and
// shift every field after it — silently. Argument position makes that
// structurally impossible.
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

// appendLedger is step 14: the chained append of one agent_subprocess line to
// ledger.jsonl and the atomic ledger.tip update (<seq>:<sha256(line)> via a
// .tmp + rename). Every error is returned BARE — the returned text is the
// path's contract — and op names the failing call for the signal: mkdir |
// chain_link | open | write | close | tip_tmp | tip_rename.
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

// openAppend is the production ledger opener.
func openAppend(path string) (io.WriteCloser, error) {
	return os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}

// ChainLink reads the chain link the next entry hashes against: the sha256 of
// the last line and the line count (the zero seed and 0 for an absent or
// empty ledger).
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

// SHA256Hex is the hex sha256 of s.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// JSONStringEscape handles the subset bash escaped (only ") expanded to
// quote + backslash + the three control characters — kept for one-line JSONL
// determinism.
func JSONStringEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// QualityTier maps the capability manifest's support flags to the v8.51.0
// quality_tier label ledger entries carry. full = both supports true;
// degraded = both false; hybrid = one of each.
func QualityTier(budgetNative, permissionScoping bool) string {
	if budgetNative && permissionScoping {
		return "full"
	}
	if !budgetNative && !permissionScoping {
		return "degraded"
	}
	return "hybrid"
}
