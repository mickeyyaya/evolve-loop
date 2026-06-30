// Package clihealth is the durable memory for transient CLI-family outages
// (cycle-283 forensics): the bridge classifies a quota wall on every dispatch
// (pattern_name=rate_limit → exit 85) but nothing remembered it, so every
// phase re-burned the walled CLI's 5-15min boot before falling back. This
// package stores expiring per-FAMILY bench records in .evolve/cli-health.json;
// llmroute demotes benched families, the loop canaries expired ones, and
// preflight reports active ones. Stdlib-only leaf — importable by llmroute,
// runner, cmd, and looppreflight without cycles.
//
// The "walled until T" fact is idempotent, but the per-family Strikes counter is
// ACCUMULATIVE — concurrent fleet cycles walling the same family must each see the
// prior strike. An unlocked read-modify-write loses increments and under-escalates
// the cooldown, so the full RMW (BenchWall/Bench/Clear) runs under the "<file>.lock"
// sidecar flock (the project-wide convention; EVOLVE_FLEET=1 skips the global cycle
// lock that previously serialized these writers). Writes stay temp+rename atomic; a
// corrupt or missing file degrades to empty, never fatal.
package clihealth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
)

// Entry is one benched CLI family.
type Entry struct {
	Family       string    `json:"family"` // CLI family ("codex"), never a driver name
	Reason       string    `json:"reason"` // classifier pattern, e.g. "rate_limit"
	BenchedAt    time.Time `json:"benched_at"`
	BenchedUntil time.Time `json:"benched_until"`
	Evidence     string    `json:"evidence,omitempty"` // truncated wall line from pane_tail
	Strikes      int       `json:"strikes"`            // consecutive re-benches; doubles cooldown
}

// DefaultCooldown is the bench duration when no reset hint parses; doubled
// per strike via CooldownForStrikes, capped at MaxCooldown.
const (
	DefaultCooldown = 30 * time.Minute
	MaxCooldown     = 4 * time.Hour
)

// BootTimeoutPattern is the classifier pattern name for a driver-scoped REPL
// boot failure (ExitREPLBootTimeout, exit 80). Used as the Reason field in
// boot-timeout bench entries so the pattern vocabulary stays in a single home.
const BootTimeoutPattern = "repl_boot_timeout"

// DefaultBootBenchThreshold is how many consecutive RecordBootStrike calls for
// the same driver (exit-80 events) promote it to an active bench. Below
// threshold the strikes are tracked but the driver remains retryable.
const DefaultBootBenchThreshold = 2

// IsBootTimeoutExitCode reports whether exitCode is the REPL boot-timeout class
// (exit 80 = ExitREPLBootTimeout in bridge/exitcodes.go). Kept as an integer
// literal to avoid an import cycle (bridge imports clihealth, not vice versa).
func IsBootTimeoutExitCode(exitCode int) bool { return exitCode == 80 }

// Benchable reports whether an escalation pattern name marks a classified
// transient outage worth benching for. Deliberately a tiny closed set (single
// home — runner bench-writer and loop canary both consult it): re-dispatching a
// walled resource is guaranteed waste, while most escalations (trust prompts,
// auth rechecks) are situational. auth_recheck is the documented next candidate.
func Benchable(pattern string) bool {
	return pattern == "rate_limit" || pattern == BootTimeoutPattern
}

// CooldownForStrikes returns DefaultCooldown doubled per re-bench beyond the
// first, capped at MaxCooldown. Strikes <=1 → DefaultCooldown.
func CooldownForStrikes(strikes int) time.Duration {
	d := DefaultCooldown
	for s := 1; s < strikes && d < MaxCooldown; s++ {
		d *= 2
	}
	if d > MaxCooldown {
		return MaxCooldown
	}
	return d
}

// NewBenchEntry composes the bench record for family after a classified wall:
// strikes continue from prev (zero-value prev → first strike), benched_until
// honors the pane's own reset hint when one parses (else the strike-scaled
// cooldown), and the wall's banner line (the reset-hint line via evidenceLine,
// or the first line as fallback) is kept as evidence. Single home — the
// runner's bench-writer and the loop's canary compose entries HERE so the
// strike/cooldown/evidence logic can never drift between them.
func NewBenchEntry(prev Entry, family, pattern, paneText string, now time.Time) Entry {
	strikes := prev.Strikes + 1
	until, parsed := ParseResetHint(paneText, now)
	if !parsed {
		until = now.Add(CooldownForStrikes(strikes))
	}
	return Entry{
		Family: family, Reason: pattern, BenchedAt: now, BenchedUntil: until,
		Evidence: truncateRunes(evidenceLine(paneText), 160), Strikes: strikes,
	}
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

// truncateRunes truncates on RUNE boundaries — wall text carries multi-byte
// glyphs (the cycle-283 codex wall opens with '■') and a byte slice could
// split one, corrupting the stored evidence.
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// fileSchema is the on-disk shape of .evolve/cli-health.json.
type fileSchema struct {
	SchemaVersion int              `json:"schema_version"`
	Benches       map[string]Entry `json:"benches"`
}

// Store reads and writes the bench file. now is injectable for tests; nil
// means time.Now.
type Store struct {
	path string
	now  func() time.Time
}

// NewStore returns a Store rooted at <projectRoot>/.evolve/cli-health.json.
func NewStore(projectRoot string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{path: filepath.Join(projectRoot, ".evolve", "cli-health.json"), now: now}
}

// Load returns all bench entries. Missing or corrupt file degrades to an
// empty map with a stderr WARN — bench state must never break a dispatch.
func (s *Store) Load() (map[string]Entry, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]Entry{}, nil
		}
		log.Default().Warnf("[clihealth] WARN read %s: %v (treating as empty)\n", s.path, err)
		return map[string]Entry{}, nil
	}
	var f fileSchema
	if err := json.Unmarshal(b, &f); err != nil {
		log.Default().Warnf("[clihealth] WARN corrupt %s: %v (treating as empty)\n", s.path, err)
		return map[string]Entry{}, nil
	}
	if f.Benches == nil {
		return map[string]Entry{}, nil
	}
	return f.Benches, nil
}

// BenchWall atomically records a classified wall for family: under the sidecar
// lock it reads the CURRENT entry, accumulates strikes via NewBenchEntry, and
// writes — so concurrent fleet cycles walling the same family never lose a strike
// increment (which would under-escalate the cooldown). It is the single home for
// the read-prev→compose→write sequence both the runner bench-writer and the loop
// canary need. Returns the composed entry.
func (s *Store) BenchWall(family, pattern, paneText string) (Entry, error) {
	var entry Entry
	err := s.withLock(func() error {
		benches, _ := s.Load()
		entry = NewBenchEntry(benches[family], family, pattern, paneText, s.now())
		benches[family] = entry
		return s.write(benches)
	})
	return entry, err
}

// Bench upserts e (keyed by e.Family) via a locked read-modify-write + temp+rename.
func (s *Store) Bench(e Entry) error {
	return s.withLock(func() error {
		benches, _ := s.Load()
		benches[e.Family] = e
		return s.write(benches)
	})
}

// Clear removes family's entry under the sidecar lock; clearing an absent family
// is a no-op.
func (s *Store) Clear(family string) error {
	return s.withLock(func() error {
		benches, _ := s.Load()
		if _, ok := benches[family]; !ok {
			return nil
		}
		delete(benches, family)
		return s.write(benches)
	})
}

// RecordBootStrike atomically records one boot-timeout strike for the named
// driver (keyed by driver name, not CLI family — a tmux-REPL boot failure is
// driver-specific, not family-wide). Returns benched=true when consecutive
// strikes reach DefaultBootBenchThreshold; below the threshold the driver's
// strike count is persisted but it remains retryable (BenchedUntil is not in
// the future, so Active() excludes it). Single-strike transient retries are
// preserved. Reason is always BootTimeoutPattern (single home).
func (s *Store) RecordBootStrike(driver string) (benched bool, err error) {
	err = s.withLock(func() error {
		benches, _ := s.Load()
		prev := benches[driver]
		strikes := prev.Strikes + 1
		now := s.now()
		var until time.Time
		if strikes >= DefaultBootBenchThreshold {
			until = now.Add(CooldownForStrikes(strikes))
			benched = true
		} else {
			// Below threshold: record the strike count but expire immediately so
			// Active() excludes this entry (the driver stays retryable).
			until = now
		}
		benches[driver] = Entry{
			Family:       driver,
			Reason:       BootTimeoutPattern,
			BenchedAt:    now,
			BenchedUntil: until,
			Strikes:      strikes,
		}
		return s.write(benches)
	})
	return benched, err
}

// withLock serializes a read-modify-write on the bench file. The "<file>.lock"
// sidecar lives beside the data file, so the dir must exist before we can create
// the lock — MkdirAll first (the data file itself is rename-replaced, so locking
// a sidecar, not the inode, is the project convention).
func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("clihealth: mkdir: %w", err)
	}
	return flock.WithPathLock(s.path, fn)
}

// Active returns entries still within their bench window (now < BenchedUntil).
func (s *Store) Active() map[string]Entry {
	return s.filter(func(e Entry) bool { return s.now().Before(e.BenchedUntil) })
}

// Expired returns entries past their bench window — canary candidates.
func (s *Store) Expired() map[string]Entry {
	return s.filter(func(e Entry) bool { return !s.now().Before(e.BenchedUntil) })
}

func (s *Store) filter(keep func(Entry) bool) map[string]Entry {
	benches, _ := s.Load()
	out := map[string]Entry{}
	for fam, e := range benches {
		if keep(e) {
			out[fam] = e
		}
	}
	return out
}

func (s *Store) write(benches map[string]Entry) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("clihealth: mkdir: %w", err)
	}
	b, err := json.MarshalIndent(fileSchema{SchemaVersion: 1, Benches: benches}, "", "  ")
	if err != nil {
		return fmt.Errorf("clihealth: marshal: %w", err)
	}
	tmp := fmt.Sprintf("%s.tmp.%d", s.path, os.Getpid())
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("clihealth: write temp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("clihealth: rename: %w", err)
	}
	return nil
}
