package recurrence

// Chronicle S2: deterministic token-capped recent-outcomes digest
// (chronicle-s2-digest-writer). WriteDigest renders the caller-assembled
// recent cycle history into <workspacePath>/recent-outcomes.md so downstream
// prompts can cite what actually happened lately. The function is pure with
// respect to inputs: the CALLER assembles DigestInput (dossiers, failed
// approaches, recurrence index) — WriteDigest only renders and writes.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/failureadapter"
)

// DigestInput is the caller-assembled history WriteDigest renders.
type DigestInput struct {
	Dossiers         []dossier.Dossier
	FailedApproaches []failureadapter.Entry
	Index            *Ledger
}

// DigestConfig bounds the rendered digest: TokenBudget caps the whole file
// (local len/4 estimator), Cycles caps the dossier window.
type DigestConfig struct {
	TokenBudget int
	Cycles      int
}

// digestFileName is the single artifact WriteDigest owns.
const digestFileName = "recent-outcomes.md"

// digestTopPatterns caps the non-generic PatternStats section (compiled
// constant per the chronicle plan, not policy-tunable).
const digestTopPatterns = 5

// estimateDigestTokens is the LOCAL chars/4 token estimator (same heuristic as
// deliverable.charsPerToken — deliberately not imported: deliverable would be
// an import cycle from this core package).
func estimateDigestTokens(s string) int { return len(s) / 4 }

// sanitizeLine collapses newlines, carriage returns, and tabs to spaces (the
// triage sanitizePromptValue idiom). Digest values are LLM-authored and are
// re-injected into later prompts, so an embedded "\n- directive" is a prompt
// injection channel — every rendered value passes through here.
func sanitizeLine(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, s)
}

// WriteDigest renders in to <workspacePath>/recent-outcomes.md, newest cycle
// first, one line per cycle, truncated to cfg.TokenBudget by dropping the
// oldest content first. Empty history writes no file (a headers-only artifact
// would waste prompt tokens every cycle and signal false coverage).
func WriteDigest(workspacePath string, in DigestInput, cfg DigestConfig) error {
	lines := digestLines(in, cfg)
	if len(lines) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("# Recent Outcomes\n")
	for _, line := range lines {
		next := line + "\n"
		if cfg.TokenBudget > 0 && estimateDigestTokens(b.String()+next) > cfg.TokenBudget {
			break
		}
		b.WriteString(next)
	}
	path := filepath.Join(workspacePath, digestFileName)
	tmp := path + fmt.Sprintf(".tmp.%d", os.Getpid())
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write digest tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename digest into place: %w", err)
	}
	return nil
}

// digestLines renders the body lines in priority order: cycle outcomes newest
// first, then failed approaches, then pattern stats. Truncation later drops
// from the END, so priority order doubles as retention order.
func digestLines(in DigestInput, cfg DigestConfig) []string {
	var lines []string

	ds := append([]dossier.Dossier(nil), in.Dossiers...)
	sort.Slice(ds, func(i, j int) bool { return ds[i].Cycle > ds[j].Cycle })
	if cfg.Cycles > 0 && len(ds) > cfg.Cycles {
		ds = ds[:cfg.Cycles]
	}
	for _, d := range ds {
		lines = append(lines, cycleLine(d))
	}

	if fa := failedApproachLines(in.FailedApproaches); len(fa) > 0 {
		lines = append(lines, "## Failed Approaches")
		lines = append(lines, fa...)
	}
	if ps := patternLines(in.Index); len(ps) > 0 {
		lines = append(lines, "## Pattern Stats")
		lines = append(lines, ps...)
	}
	return lines
}

// cycleLine renders one dossier as a single sanitized bullet:
// cycle, goal, verdict, one-line why, carryover ids.
func cycleLine(d dossier.Dossier) string {
	parts := []string{fmt.Sprintf("cycle %d %s — %s", d.Cycle, sanitizeLine(d.FinalVerdict), sanitizeLine(d.Goal))}
	if len(d.Defects) > 0 {
		parts = append(parts, "why: "+sanitizeLine(d.Defects[0].Summary))
	}
	if len(d.Lessons) > 0 {
		ps := make([]string, 0, len(d.Lessons))
		for _, l := range d.Lessons {
			ps = append(ps, sanitizeLine(l.Pattern))
		}
		parts = append(parts, "lessons: "+strings.Join(ps, "; "))
	}
	if len(d.Carryover) > 0 {
		ids := make([]string, 0, len(d.Carryover))
		for _, c := range d.Carryover {
			ids = append(ids, sanitizeLine(c.ID))
		}
		parts = append(parts, "carryover: "+strings.Join(ids, ", "))
	}
	return "- " + strings.Join(parts, " — ")
}

func failedApproachLines(entries []failureadapter.Entry) []string {
	var lines []string
	for _, e := range entries {
		line := fmt.Sprintf("- cycle %d %s: %s", e.Cycle, sanitizeLine(e.Verdict),
			sanitizeLine(string(e.Classification)))
		if len(e.Defects) > 0 {
			line += " — " + sanitizeLine(strings.Join(e.Defects, "; "))
		}
		lines = append(lines, line)
	}
	return lines
}

// patternLines renders the top-K non-generic recurrence patterns one per line
// and rolls ALL generic (classification-vocabulary) patterns up into ONE
// aggregate line — e.g. "158 floor lessons: operator-reset x96, loop-fatal x62"
// — so vocabulary noise can never crowd out actionable patterns.
func patternLines(idx *Ledger) []string {
	if idx == nil {
		return nil
	}
	var generic, specific []*Entry
	for _, e := range idx.Entries {
		if e.Generic {
			generic = append(generic, e)
		} else {
			specific = append(specific, e)
		}
	}
	byCountDesc := func(s []*Entry) {
		sort.Slice(s, func(i, j int) bool {
			if s[i].Count != s[j].Count {
				return s[i].Count > s[j].Count
			}
			return s[i].Pattern < s[j].Pattern
		})
	}
	byCountDesc(specific)
	byCountDesc(generic)

	var lines []string
	if len(specific) > digestTopPatterns {
		specific = specific[:digestTopPatterns]
	}
	for _, e := range specific {
		lines = append(lines, fmt.Sprintf("- %s x%d", sanitizeLine(e.Pattern), e.Count))
	}
	if len(generic) > 0 {
		total := 0
		rollup := make([]string, 0, len(generic))
		for _, e := range generic {
			total += e.Count
			rollup = append(rollup, fmt.Sprintf("%s x%d", sanitizeLine(e.Pattern), e.Count))
		}
		lines = append(lines, fmt.Sprintf("- %d floor lessons: %s", total, strings.Join(rollup, ", ")))
	}
	return lines
}
