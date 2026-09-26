// Package ledger implements core.Ledger as an append-only JSONL file hash-chained over each line's raw bytes.
// See docs/architecture/packages/internal-adapters-ledger.md.
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

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// ZeroSeed is the prev_hash of a genesis entry (64 ASCII '0's).
const ZeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

// FileLedger writes/reads <evolveDir>/ledger.jsonl + ledger.tip.
type FileLedger struct {
	ledgerPath string
	tipPath    string
	lockPath   string
	anchorPath string
	mu         sync.Mutex
	// onAppend observes every entry through Append, the chokepoint all writers reach; nil is unobserved.
	onAppend func(e core.LedgerEntry, err error)
}

// Option configures a FileLedger at construction (functional options).
type Option func(*FileLedger)

// ledgerHooks are test seams for marshal and I/O error branches a healthy filesystem never reaches.
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

// New constructs a FileLedger rooted at evolveDir, applying opts in order.
func New(evolveDir string, opts ...Option) *FileLedger {
	l := &FileLedger{
		ledgerPath: filepath.Join(evolveDir, "ledger.jsonl"),
		tipPath:    filepath.Join(evolveDir, "ledger.tip"),
		lockPath:   filepath.Join(evolveDir, "ledger.lock"),
		anchorPath: filepath.Join(evolveDir, "ledger-anchor.json"),
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Append chains e onto the tip (entry_seq, prev_hash), appends it to ledger.jsonl and replaces ledger.tip.
func (l *FileLedger) Append(_ context.Context, e core.LedgerEntry) error {
	err := l.appendChained(func(seq int, prevHash string) any {
		e.EntrySeq = seq
		e.PrevHash = prevHash
		return e
	})
	if l.onAppend != nil {
		l.onAppend(e, err) // e carries the chained seq the fill stamped on success
	}
	return err
}

// appendChained is the tip-read→append→tip-write critical section every chained line goes through.
// A line written outside it breaks the next entry, which chains from the tip while walkChain reads the file.
func (l *FileLedger) appendChained(fill func(seq int, prevHash string) any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	// mu cannot serialize two evolve processes; without the flock their tip reads and appends interleave.
	release, err := flock.Lock(l.lockPath)
	if err != nil {
		return fmt.Errorf("ledger: %w", err)
	}
	defer release()

	prevSeq, prevHash, err := l.readTip()
	if err != nil {
		return err
	}
	seq := 0
	if prevHash == "" {
		prevHash = ZeroSeed
	} else {
		seq = prevSeq + 1
	}

	return l.appendLineAndReplaceTip(fill(seq, prevHash), seq)
}

// appendLineAndReplaceTip is the one write half of both chain writers, so they cannot diverge.
// Callers hold mu and the flock and pass an entry whose seq and prev_hash are already derived.
func (l *FileLedger) appendLineAndReplaceTip(entry any, seq int) error {
	line, err := hooks.marshal(entry)
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
	tip := fmt.Sprintf("%d:%s", seq, newHash)
	// tmp+rename so a concurrent reader never sees a truncated tip.
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

// LifecycleRecord is one inbox-lifecycle event to append as a chained entry without inboxmover importing core.
type LifecycleRecord struct {
	TS      string
	Action  string
	TaskID  string
	Message string
	GitHead string
	Cycle   int
}

// AppendLifecycle appends one inbox-lifecycle record through the chained Append path.
func (l *FileLedger) AppendLifecycle(ctx context.Context, r LifecycleRecord) error {
	return l.Append(ctx, core.LedgerEntry{
		TS:      r.TS,
		Role:    "orchestrator",
		Kind:    "inbox-lifecycle",
		Action:  r.Action,
		TaskID:  r.TaskID,
		Message: r.Message,
		GitHEAD: r.GitHead,
		Cycle:   r.Cycle,
	})
}

// appendChainedFromTail is the repair-path appendChained: it chains from the physical last line, not the tip,
// because walkChain checks physical predecessors. Its full-file read keeps it off the hot append path.
func (l *FileLedger) appendChainedFromTail(fill func(seq int, prevHash string) any) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	release, err := flock.Lock(l.lockPath)
	if err != nil {
		return fmt.Errorf("ledger: %w", err)
	}
	defer release()

	raw, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		return fmt.Errorf("ledger read: %w", err)
	}
	lines := splitLines(raw)
	if len(lines) == 0 {
		return fmt.Errorf("ledger: appendChainedFromTail on an empty chain")
	}
	prevHash := sha256Hex(lines[len(lines)-1])
	prevSeq, _, err := l.readTip()
	if err != nil {
		return err
	}
	return l.appendLineAndReplaceTip(fill(prevSeq+1, prevHash), prevSeq+1)
}

// Verify walks the live file's hash chain and tip, returning core.ErrLedgerChainBroken on any inconsistency.
func (l *FileLedger) Verify(ctx context.Context) error {
	_, err := l.VerifyScope(ctx)
	return err
}

// VerifyScope is Verify plus the scope that same walk validated; zero means every line from genesis.
func (l *FileLedger) VerifyScope(_ context.Context) (VerifiedScope, error) {
	raw, err := os.ReadFile(l.ledgerPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return VerifiedScope{}, nil
		}
		return VerifiedScope{}, fmt.Errorf("ledger read: %w", err)
	}
	lines := splitLines(raw)
	if len(lines) == 0 {
		return VerifiedScope{}, nil
	}

	anchorSHA, anchorSeq := effectiveAnchorSHA(lines, l.loadAnchorSHA())
	lastSeq, lastSha, sawV837, err := walkChain(lines, anchorSHA)
	if err != nil {
		return VerifiedScope{}, err
	}
	scope := VerifiedScope{AnchorLineSHA: anchorSHA, AnchorSeq: anchorSeq}
	// A ledger with no chained (v8.37+) line has no tip to check.
	if !sawV837 {
		return scope, nil
	}
	if err := l.checkTip(lastSeq, lastSha); err != nil {
		return VerifiedScope{}, err
	}
	return scope, nil
}

// walkChain is the one strict chain walk behind Verify and VerifyDeep. It accepts two seams real
// history carries: a re-genesis (seq 0, zero prev) and a fork sibling (the previous line's parent).
func walkChain(lines [][]byte, anchorLineSHA string) (lastSeq int, lastSha string, sawV837 bool, err error) {
	seenPrev := map[string]struct{}{}
	prevLinePrev := "" // previous line's prev_hash (fork-sibling signature)
	// Lines before an epoch anchor are preserved but not validated, by operator sign-off.
	// See ADR-0048.
	inEpoch := anchorLineSHA == ""
	for i, line := range lines {
		if !inEpoch {
			// lastSha is set before the match so the first post-anchor line chains from the anchor line.
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
		// A composition verdict whose diffs no longer re-derive its patch_id is tampered, like a hash break.
		if e.Kind == CompositionVerdictKind {
			if cerr := verifyCompositionLine(i, line); cerr != nil {
				return 0, "", false, cerr
			}
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
				// The chained region starts with a seq-0 zero-seeded entry or one chained from the last
				// unchained (pre-v8.37) line; a zero seed with a nonzero seq stays a break.
				return 0, "", false, fmt.Errorf("%w: line %d chained-genesis prev_hash mismatch (have %s want zero seed or %s)", core.ErrLedgerChainBroken, i, e.PrevHash, lastSha)
			}
			// Seams and siblings repeat a prev_hash by nature; any other duplicate is a non-adjacent fork.
			// Sibling runs are unbounded: without control of ledger.tip an attacker gains nothing from one.
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
		lastSha = sha256Hex(line)
	}
	if !inEpoch {
		// No line carries the anchor's SHA, so its content is absent or altered: fail rather than relax.
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

// decodeLedgerLine parses one line and reports whether prev_hash is present as a key, even with value "".
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
