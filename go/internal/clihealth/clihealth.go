// Package clihealth is the durable memory for transient CLI-family outages
// (cycle-283 forensics): the bridge classifies a quota wall on every dispatch
// (pattern_name=rate_limit → exit 85) but nothing remembered it, so every
// phase re-burned the walled CLI's 5-15min boot before falling back. This
// package stores expiring per-FAMILY bench records in .evolve/cli-health.json;
// llmroute demotes benched families, the loop canaries expired ones, and
// preflight reports active ones. Stdlib-only leaf — importable by llmroute,
// runner, cmd, and looppreflight without cycles.
//
// The store is deliberately tiny and last-writer-wins: concurrent writers
// (runner mid-cycle, loop between cycles) write the same idempotent fact
// ("family X is walled until T"), so a lost update is harmless. Writes are
// temp+rename atomic; a corrupt or missing file degrades to empty, never
// fatal — bench state is advice, losing it only costs one re-discovery.
package clihealth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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

// Benchable reports whether an escalation pattern name marks a classified
// transient outage worth benching the whole CLI family for. Deliberately a
// tiny closed set (single home — runner bench-writer and loop canary both
// consult it): re-dispatching a walled family is guaranteed waste, while most
// escalations (trust prompts, auth rechecks) are situational. auth_recheck is
// the documented next candidate (operator call).
func Benchable(pattern string) bool {
	return pattern == "rate_limit"
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
		fmt.Fprintf(os.Stderr, "[clihealth] WARN read %s: %v (treating as empty)\n", s.path, err)
		return map[string]Entry{}, nil
	}
	var f fileSchema
	if err := json.Unmarshal(b, &f); err != nil {
		fmt.Fprintf(os.Stderr, "[clihealth] WARN corrupt %s: %v (treating as empty)\n", s.path, err)
		return map[string]Entry{}, nil
	}
	if f.Benches == nil {
		return map[string]Entry{}, nil
	}
	return f.Benches, nil
}

// Bench upserts e (keyed by e.Family) via read-modify-write + temp+rename.
func (s *Store) Bench(e Entry) error {
	benches, _ := s.Load()
	benches[e.Family] = e
	return s.write(benches)
}

// Clear removes family's entry; clearing an absent family is a no-op.
func (s *Store) Clear(family string) error {
	benches, _ := s.Load()
	if _, ok := benches[family]; !ok {
		return nil
	}
	delete(benches, family)
	return s.write(benches)
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
