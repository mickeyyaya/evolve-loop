// Package clihealth remembers which CLI families and drivers are walled, and for how long.
package clihealth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/log"
)

type Entry struct {
	Family         string    `json:"family"`
	Scope          string    `json:"scope,omitempty"`
	Model          string    `json:"model,omitempty"`
	Reason         string    `json:"reason"`
	BenchedAt      time.Time `json:"benched_at"`
	BenchedUntil   time.Time `json:"benched_until"`
	Evidence       string    `json:"evidence,omitempty"`
	Strikes        int       `json:"strikes"`
	OperatorAction string    `json:"operator_action,omitempty"`
}

const (
	DefaultCooldown = 30 * time.Minute
	MaxCooldown     = 4 * time.Hour
)

const BootTimeoutPattern = "repl_boot_timeout"

const DefaultBootBenchThreshold = 2

const replBootTimeoutExitCode = 80

func IsBootTimeoutExitCode(exitCode int) bool { return exitCode == replBootTimeoutExitCode }

func Benchable(pattern string) bool {
	return pattern == "rate_limit" || pattern == ExhaustedPattern || pattern == QuotaExhaustedPattern || pattern == BootTimeoutPattern || pattern == CredentialPattern
}

const QuotaExhaustedPattern = "quota_exhausted"

const UsageProbePattern = "usage_probe"

func QuotaPattern(pattern string) bool {
	switch pattern {
	case "rate_limit", ExhaustedPattern, QuotaExhaustedPattern, UsageProbePattern:
		return true
	}
	return false
}

const CredentialPattern = "auth_recheck"

func OperatorAction(family, pattern string) string {
	if pattern != CredentialPattern {
		return ""
	}
	return fmt.Sprintf("operator: the %s CLI needs to be logged in again; the bench clears itself once a probe succeeds", family)
}

const ExhaustedPattern = "exhausted"

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

func NewBenchEntry(prev Entry, family, pattern, paneText string, now time.Time) Entry {
	strikes := prev.Strikes + 1
	until, parsed := time.Time{}, false
	switch pattern {
	case CredentialPattern:
	case ExhaustedPattern:
		until, parsed = parseHints(clockedHints, paneText, now)
	default:
		until, parsed = ParseResetHint(paneText, now)
	}
	if !parsed {
		until = now.Add(CooldownForStrikes(strikes))
	}
	return Entry{
		Family: family, Reason: pattern, BenchedAt: now, BenchedUntil: until,
		Evidence: truncateRunes(evidenceLine(paneText), 160), Strikes: strikes,
		OperatorAction: OperatorAction(family, pattern),
	}
}

func (e Entry) Key() string {
	if e.Model != "" {
		return e.Family + "\x00" + e.Model
	}
	if e.Scope == "" {
		return e.Family
	}
	return e.Family + "@" + e.Scope
}

func ModelGroup(model string) string {
	model = strings.TrimSpace(model)
	if i := strings.LastIndex(model, " ("); i > 0 && strings.HasSuffix(model, ")") {
		return strings.TrimSpace(model[:i])
	}
	return model
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

type fileSchema struct {
	SchemaVersion int              `json:"schema_version"`
	Benches       map[string]Entry `json:"benches"`
}

type Store struct {
	path string
	now  func() time.Time
}

func NewStore(projectRoot string, now func() time.Time) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{path: filepath.Join(projectRoot, ".evolve", "cli-health.json"), now: now}
}

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

func (s *Store) BenchWall(family, pattern, paneText string) (Entry, error) {
	return s.BenchScopedWall(family, "", pattern, paneText)
}

func (s *Store) BenchScopedWall(family, scope, pattern, paneText string) (Entry, error) {
	key := Entry{Family: family, Scope: scope}.Key()
	return s.benchComposed(key, func(prev Entry, now time.Time) Entry {
		entry := NewBenchEntry(prev, family, pattern, paneText, now)
		entry.Scope = scope
		return entry
	})
}

type Wall struct {
	Pattern  string
	Evidence string
	Reset    time.Time
}

func (s *Store) BenchWallUntil(family string, w Wall) (Entry, error) {
	return s.benchComposed(family, func(prev Entry, now time.Time) Entry {
		return wallEntry(prev, family, w, now)
	})
}

func (s *Store) BenchModelUntil(family, model string, w Wall) (Entry, error) {
	key := Entry{Family: family, Model: model}.Key()
	return s.benchComposed(key, func(prev Entry, now time.Time) Entry {
		entry := wallEntry(prev, family, w, now)
		entry.Model = model
		return entry
	})
}

func wallEntry(prev Entry, family string, w Wall, now time.Time) Entry {
	entry := NewBenchEntry(prev, family, w.Pattern, w.Evidence, now)
	if w.Reset.After(now) {
		entry.BenchedUntil = capHint(w.Reset.Add(resetMargin), now)
	}
	return entry
}

func (s *Store) benchComposed(key string, compose func(prev Entry, now time.Time) Entry) (Entry, error) {
	var entry Entry
	err := s.withLock(func() error {
		benches, _ := s.Load()
		entry = compose(benches[key], s.now())
		benches[key] = entry
		return s.write(benches)
	})
	return entry, err
}

func (s *Store) Bench(e Entry) error {
	return s.withLock(func() error {
		benches, _ := s.Load()
		benches[e.Key()] = e
		return s.write(benches)
	})
}

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

func (s *Store) ClearBootStrike(driver string) error {
	return s.withLock(func() error {
		benches, _ := s.Load()
		e, ok := benches[driver]
		if !ok || e.Reason != BootTimeoutPattern {
			return nil
		}
		delete(benches, driver)
		return s.write(benches)
	})
}

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

func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("clihealth: mkdir: %w", err)
	}
	return flock.WithPathLock(s.path, fn)
}

func (s *Store) Active() map[string]Entry {
	return s.filter(func(e Entry) bool {
		return e.Model == "" && (e.Reason == CredentialPattern || s.now().Before(e.BenchedUntil))
	})
}

func (s *Store) ActiveModels() map[string]Entry {
	return s.filter(func(e Entry) bool { return e.Model != "" && s.now().Before(e.BenchedUntil) })
}

func (s *Store) Expired() map[string]Entry {
	return s.filter(func(e Entry) bool { return e.Model == "" && !s.now().Before(e.BenchedUntil) })
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
