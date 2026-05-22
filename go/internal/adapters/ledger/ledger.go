// Package ledger implements core.Ledger as a JSONL file with a SHA256
// chain over the raw bytes of each line. Port of
// scripts/observability/verify-ledger-chain.sh.
//
// Files written:
//   - <evolveDir>/ledger.jsonl  — append-only line per entry
//   - <evolveDir>/ledger.tip    — "<seq>:<sha256-of-last-line>"
package ledger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// ZeroSeed is the prev_hash value used by the very first ledger entry.
// Matches the bash convention (64 ASCII '0's).
const ZeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// FileLedger writes/reads <evolveDir>/ledger.jsonl + ledger.tip.
type FileLedger struct {
	ledgerPath string
	tipPath    string
	mu         sync.Mutex
}

// hooks holds injectable seams so tests can drive marshal/I/O error
// branches that are otherwise unreachable on a healthy filesystem.
type ledgerHooks struct {
	marshal func(any) ([]byte, error)
	openF   func(path string, flag int, perm os.FileMode) (*os.File, error)
	write   func(f *os.File, b []byte) (int, error)
	closeF  func(f *os.File) error
	writeF  func(path string, data []byte, perm os.FileMode) error
}

var hooks = ledgerHooks{
	marshal: json.Marshal,
	openF:   os.OpenFile,
	write:   func(f *os.File, b []byte) (int, error) { return f.Write(b) },
	closeF:  func(f *os.File) error { return f.Close() },
	writeF:  os.WriteFile,
}

func withHooks(replacement ledgerHooks, fn func()) {
	prev := hooks
	if replacement.marshal != nil {
		hooks.marshal = replacement.marshal
	}
	if replacement.openF != nil {
		hooks.openF = replacement.openF
	}
	if replacement.write != nil {
		hooks.write = replacement.write
	}
	if replacement.closeF != nil {
		hooks.closeF = replacement.closeF
	}
	if replacement.writeF != nil {
		hooks.writeF = replacement.writeF
	}
	defer func() { hooks = prev }()
	fn()
}

// New constructs a FileLedger rooted at evolveDir.
func New(evolveDir string) *FileLedger {
	return &FileLedger{
		ledgerPath: filepath.Join(evolveDir, "ledger.jsonl"),
		tipPath:    filepath.Join(evolveDir, "ledger.tip"),
	}
}

// Append serializes e (with prev_hash + entry_seq filled in by the
// ledger), appends it to ledger.jsonl, and updates ledger.tip.
// Safe under concurrent goroutines (mutex-guarded).
func (l *FileLedger) Append(_ context.Context, e core.LedgerEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	prevSeq, prevHash, err := l.readTip()
	if err != nil {
		return err
	}
	if prevHash == "" {
		e.PrevHash = ZeroSeed
		e.EntrySeq = 0
	} else {
		e.PrevHash = prevHash
		e.EntrySeq = prevSeq + 1
	}

	line, err := hooks.marshal(e)
	if err != nil {
		return fmt.Errorf("ledger marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(l.ledgerPath), 0o755); err != nil {
		return fmt.Errorf("ledger mkdir: %w", err)
	}
	f, err := hooks.openF(l.ledgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("ledger open: %w", err)
	}
	if _, err := hooks.write(f, append(line, '\n')); err != nil {
		_ = hooks.closeF(f)
		return fmt.Errorf("ledger append: %w", err)
	}
	if err := hooks.closeF(f); err != nil {
		return fmt.Errorf("ledger close: %w", err)
	}

	newHash := sha256Hex(line)
	tip := fmt.Sprintf("%d:%s", e.EntrySeq, newHash)
	if err := hooks.writeF(l.tipPath, []byte(tip), 0o644); err != nil {
		return fmt.Errorf("tip write: %w", err)
	}
	return nil
}

// Verify walks every line, recomputes prev_hash, checks first entry's
// zero-init, checks tip equals SHA256 of the last line, and flags any
// duplicate prev_hash anomalies. Returns core.ErrLedgerChainBroken on
// any inconsistency.
func (l *FileLedger) Verify(_ context.Context) error {
	raw, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // bootstrap state — no entries yet
		}
		return fmt.Errorf("ledger read: %w", err)
	}
	lines := splitLines(raw)
	if len(lines) == 0 {
		return nil
	}

	var lastSeq int
	var lastSha string
	seenPrev := map[string]struct{}{}
	for i, line := range lines {
		var e core.LedgerEntry
		if err := json.Unmarshal(line, &e); err != nil {
			return fmt.Errorf("%w: line %d unmarshal: %v", core.ErrLedgerChainBroken, i, err)
		}
		expected := ZeroSeed
		if i > 0 {
			expected = lastSha
		}
		if e.PrevHash != expected {
			return fmt.Errorf("%w: line %d prev_hash mismatch (have %s want %s)", core.ErrLedgerChainBroken, i, e.PrevHash, expected)
		}
		if _, dup := seenPrev[e.PrevHash]; dup && i > 0 {
			return fmt.Errorf("%w: line %d duplicate prev_hash (concurrent fan-out anomaly)", core.ErrLedgerChainBroken, i)
		}
		seenPrev[e.PrevHash] = struct{}{}
		lastSeq = e.EntrySeq
		lastSha = sha256Hex(line)
	}

	tip, err := os.ReadFile(l.tipPath)
	if err != nil {
		return fmt.Errorf("%w: tip read: %v", core.ErrLedgerChainBroken, err)
	}
	wantTip := fmt.Sprintf("%d:%s", lastSeq, lastSha)
	if string(tip) != wantTip {
		return fmt.Errorf("%w: tip mismatch (have %q want %q)", core.ErrLedgerChainBroken, tip, wantTip)
	}
	return nil
}

// Iter returns a LedgerIterator yielding entries in append order.
func (l *FileLedger) Iter(_ context.Context) (core.LedgerIterator, error) {
	raw, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &lineIter{}, nil
		}
		return nil, fmt.Errorf("ledger read: %w", err)
	}
	return &lineIter{lines: splitLines(raw)}, nil
}

type lineIter struct {
	lines [][]byte
	i     int
}

func (it *lineIter) Next() (core.LedgerEntry, bool, error) {
	if it.i >= len(it.lines) {
		return core.LedgerEntry{}, false, nil
	}
	var e core.LedgerEntry
	if err := json.Unmarshal(it.lines[it.i], &e); err != nil {
		return core.LedgerEntry{}, false, fmt.Errorf("ledger iter line %d: %w", it.i, err)
	}
	it.i++
	return e, true, nil
}

func (it *lineIter) Close() error { return nil }

func (l *FileLedger) readTip() (seq int, sha string, err error) {
	raw, err := os.ReadFile(l.tipPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf("tip read: %w", err)
	}
	parts := splitTip(string(raw))
	if len(parts) != 2 {
		return 0, "", fmt.Errorf("tip malformed: %q", raw)
	}
	if _, scanErr := fmt.Sscanf(parts[0], "%d", &seq); scanErr != nil {
		return 0, "", fmt.Errorf("tip seq parse: %w", scanErr)
	}
	return seq, parts[1], nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, b := range raw {
		if b == '\n' {
			if i > start {
				out = append(out, raw[start:i])
			}
			start = i + 1
		}
	}
	if start < len(raw) {
		out = append(out, raw[start:])
	}
	return out
}

func splitTip(s string) []string {
	// Strip a single trailing newline if present.
	if n := len(s); n > 0 && s[n-1] == '\n' {
		s = s[:n-1]
	}
	for i := 0; i < len(s); i++ {
		if s[i] == ':' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
