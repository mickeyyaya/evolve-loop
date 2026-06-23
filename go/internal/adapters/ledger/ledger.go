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

	"github.com/mickeyyaya/evolveloop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolveloop/go/internal/core"
)

// ZeroSeed is the prev_hash value used by the very first ledger entry.
// Matches the bash convention (64 ASCII '0's).
const ZeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// FileLedger writes/reads <evolveDir>/ledger.jsonl + ledger.tip.
type FileLedger struct {
	ledgerPath string
	tipPath    string
	lockPath   string
	anchorPath string
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
		lockPath:   filepath.Join(evolveDir, "ledger.lock"),
		anchorPath: filepath.Join(evolveDir, "ledger-anchor.json"),
	}
}

// Append serializes e (with prev_hash + entry_seq filled in by the
// ledger), appends it to ledger.jsonl, and updates ledger.tip.
// Safe under concurrent goroutines (mutex) AND concurrent processes
// (CA.1: blocking flock on ledger.lock around the whole
// tip-read→append→tip-write critical section — two `evolve` processes
// otherwise interleave and break the hash chain).
func (l *FileLedger) Append(_ context.Context, e core.LedgerEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := flock.Lock(l.lockPath)
	if err != nil {
		return fmt.Errorf("ledger: %w", err)
	}
	defer release()

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
	// Atomic tip replace (tmp+rename): a concurrent reader must never see a
	// truncated tip — the RED stress run surfaced exactly that (`tip
	// malformed: ""` from a mid-WriteFile read).
	tmp := fmt.Sprintf("%s.tmp.%d", l.tipPath, os.Getpid())
	if err := hooks.writeF(tmp, []byte(tip), 0o644); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("tip write: %w", err)
	}
	if err := os.Rename(tmp, l.tipPath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("tip rename: %w", err)
	}
	return nil
}

// Verify walks every line, recomputes prev_hash, checks first entry's
// zero-init, checks tip equals SHA256 of the last line, and flags any
// duplicate prev_hash anomalies. Returns core.ErrLedgerChainBroken on
// any inconsistency.
//
// Soft-start boundary (port of verify-ledger-chain.sh): pre-v8.37
// entries have no prev_hash field at all. They are not retro-validated
// but their SHA is still computed so the first v8.37+ entry can chain
// from the last pre-v8.37 line. If the entire file is pre-v8.37 the
// tip file is optional.
func (l *FileLedger) Verify(_ context.Context) error {
	raw, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("ledger read: %w", err)
	}
	lines := splitLines(raw)
	if len(lines) == 0 {
		return nil
	}

	lastSeq, lastSha, sawV837, err := walkChain(lines, l.loadAnchorSHA())
	if err != nil {
		return err
	}
	// If no v8.37 entries exist, tip file is optional.
	if !sawV837 {
		return nil
	}
	return l.checkTip(lastSeq, lastSha)
}

// walkChain is THE chain walk, shared by Verify (live file only) and
// VerifyDeep (decompressed segments + live tail, L3.3) so the two can
// never diverge on what "intact" means.
// The walk is strict, with two carve-outs the PRODUCTION ledger's history
// requires (both made plain `evolve ledger verify` red on the real file,
// unnoticed, until the L3.3 acceptance run surfaced them):
//
//   - RE-GENESIS seam: an Append against a missing/lost tip re-seeds the
//     chain (entry_seq==0 + zero prev_hash). One exists (line 15,
//     2026-05-07 — the day v8.37 chain hashing landed). Accepted: a seam
//     is visible, every later line still hashes over its bytes, and the
//     tip + L3.3 segment anchors bind the end state. A zero prev with a
//     NONZERO seq stays a break.
//   - FORK SIBLING: pre-CA.1 concurrent Appends raced the tip and wrote
//     sibling entries sharing one parent (e.g. lines 263/264 with equal
//     seqs; line 273 with a +1 seq — the racy seq is unreliable, the hash
//     linkage is the trustworthy part). Accepted exactly when the entry's
//     prev equals the PREVIOUS line's prev (shared parent); the chain
//     resumes from the last sibling. The CA.1 flock prevents new ones.
func walkChain(lines [][]byte, anchorLineSHA string) (lastSeq int, lastSha string, sawV837 bool, err error) {
	seenPrev := map[string]struct{}{}
	prevLinePrev := "" // previous line's prev_hash (fork-sibling signature)
	// ADR-0048 ledger epoch-anchor (ledger-1740): when an operator has recorded a
	// trusted genesis line, lines BEFORE it are NOT chain-validated — the
	// historical damage is real, preserved (never deleted), and accepted by
	// explicit operator sign-off. inEpoch starts true when no anchor is set, so
	// the no-anchor path is byte-identical to the pre-anchor behavior; strict
	// validation always resumes for every line AFTER the anchor.
	inEpoch := anchorLineSHA == ""
	for i, line := range lines {
		if !inEpoch {
			// Pre-epoch: skip all validation; only locate the anchor genesis by
			// its bound SHA. lastSha is set FIRST (computed once) so the first
			// post-anchor line chains from the anchor line's SHA.
			lastSha = sha256Hex(line)
			if lastSha == anchorLineSHA {
				inEpoch = true
				sawV837 = true // strict from here forward
				if _, e, derr := decodeLedgerLine(line); derr == nil {
					lastSeq = e.EntrySeq
				}
			}
			continue
		}
		hasPrev, e, err := decodeLedgerLine(line)
		if err != nil {
			return 0, "", false, fmt.Errorf("%w: line %d unmarshal: %v", core.ErrLedgerChainBroken, i, err)
		}
		if hasPrev {
			isReGenesis := e.PrevHash == ZeroSeed && e.EntrySeq == 0
			// A sibling never shares a ZERO parent: a zero-prev line is
			// either a true (re-)genesis (seq 0, handled above) or forged.
			isForkSibling := prevLinePrev != "" && prevLinePrev != ZeroSeed && e.PrevHash == prevLinePrev
			if sawV837 {
				if e.PrevHash != lastSha && !isReGenesis && !isForkSibling {
					return 0, "", false, fmt.Errorf("%w: line %d prev_hash mismatch (have %s want %s)", core.ErrLedgerChainBroken, i, e.PrevHash, lastSha)
				}
			} else if !isReGenesis && e.PrevHash != lastSha {
				// Genesis of the chained region: either a seq-0 zero-seeded
				// entry or one chained from the last unchained (pre-v8.37)
				// line — both occur in real histories. A ZeroSeed with a
				// NONZERO seq stays a break (the soft-boundary pin). The
				// unchained prelude was never tamper-protected either way;
				// strictness begins here.
				return 0, "", false, fmt.Errorf("%w: line %d chained-genesis prev_hash mismatch (have %s want zero seed or %s)", core.ErrLedgerChainBroken, i, e.PrevHash, lastSha)
			}
			// Seams and fork siblings necessarily repeat a prev_hash —
			// exempt them; any OTHER duplicate is a same-parent fork with
			// the wrong signature (non-adjacent) and stays a break. Sibling
			// runs are deliberately UNBOUNDED: each sibling keeps
			// prevLinePrev equal to the shared parent, so a third/fourth
			// racer is accepted by the same adjacency signature — that is
			// what a wider pre-CA.1 race produced, and an attacker gains
			// nothing from it without controlling ledger.tip. The set add
			// below is a no-op for siblings (parent hash already present).
			if _, dup := seenPrev[e.PrevHash]; dup && sawV837 && !isReGenesis && !isForkSibling {
				return 0, "", false, fmt.Errorf("%w: line %d duplicate prev_hash (concurrent fan-out anomaly)", core.ErrLedgerChainBroken, i)
			}
			seenPrev[e.PrevHash] = struct{}{}
			lastSeq = e.EntrySeq
			sawV837 = true
			prevLinePrev = e.PrevHash
		} else {
			prevLinePrev = ""
		}
		// Always compute the line SHA for the next iteration's chain check.
		lastSha = sha256Hex(line)
	}
	if !inEpoch {
		// An anchor was set but no line matched its bound SHA — the anchored
		// content is absent or was altered. Fail loudly rather than silently
		// validate the whole (damaged) chain or silently relax it.
		return 0, "", false, fmt.Errorf("%w: epoch anchor line not found (sha %s) — anchored content absent or altered", core.ErrLedgerChainBroken, anchorLineSHA)
	}
	return lastSeq, lastSha, sawV837, nil
}

// checkTip verifies ledger.tip equals "<seq>:<sha>" of the last line.
func (l *FileLedger) checkTip(lastSeq int, lastSha string) error {
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

// decodeLedgerLine parses one JSONL line and returns whether prev_hash
// was present as a JSON key (distinct from being present with value "").
func decodeLedgerLine(line []byte) (hasPrevHash bool, e core.LedgerEntry, err error) {
	if err = json.Unmarshal(line, &e); err != nil {
		return false, e, err
	}
	var raw map[string]json.RawMessage
	if err = json.Unmarshal(line, &raw); err != nil {
		return false, e, err
	}
	_, hasPrevHash = raw["prev_hash"]
	return hasPrevHash, e, nil
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
