package modelquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

// EffortLister enumerates a CLI's reasoning-effort rungs (its "ladder").
//
// This exists because the catalog refresh enumerated MODELS only. When codex
// 0.147.0 grew two rungs above xhigh, nothing observed it — and an unmapped
// rung is not a no-op: realizeScalar silently drops an effort value absent
// from a manifest's values table, so the phase launches with no effort flag
// at all. Discovering the ladder turns that silence into a diff.
type EffortLister interface {
	// ListEfforts returns the rungs in the order the CLI presents them.
	// An empty result with a nil error means "this CLI has no effort dial",
	// which is a real answer and must stay distinguishable from a failure.
	ListEfforts(ctx context.Context, cli string) ([]string, error)
}

// HelpEffortLister reads a CLI's ladder from `<cli> --help`.
//
// Suitable only for CLIs that actually publish the enum there. claude and agy
// do; codex does NOT — its rungs live behind the /model picker's "More
// reasoning..." submenu and appear nowhere in help output, so pointing this
// lister at codex would yield an empty ladder that reads as "no dial". See
// DefaultEffortListers.
type HelpEffortLister struct {
	// Run defaults to the package's exec runner when nil.
	Run Runner
}

// effortToken is the shape of a rung word: a lowercase bare word. It keeps
// prose and placeholders out of the ladder when a parenthetical happens to
// contain separators.
var effortToken = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// effortFlag matches the --effort flag as a WHOLE TOKEN. A prefix match would
// also hit a neighbouring `--effort-budget`, capture the scan on the wrong
// flag, and yield an empty ladder — a false negative indistinguishable from
// "this CLI has no dial", which is the exact bug class this file closes.
var effortFlag = regexp.MustCompile(`^--effort(?:[\s=]|$)`)

// flagStart matches something shaped like a flag: a dash run followed by a
// LETTER. It ends a flag's help block. Terminating on any dash-leading line
// instead would let a bullet ("- low: quick") or prose ("-1 disables it")
// truncate the block before its enum was reached.
var flagStart = regexp.MustCompile(`^--?[A-Za-z]`)

// ListEfforts parses the enum out of the CLI's own help text.
//
// C1 exception, as with the model listers: `--help` is metadata-only. It
// reaches no model and is passed no prompt or stdin.
func (l HelpEffortLister) ListEfforts(ctx context.Context, cli string) ([]string, error) {
	run := l.Run
	if run == nil {
		run = defaultRunner
	}
	out, err := run(ctx, cli, []string{"--help"}, "")
	if err != nil {
		// A help failure and a dial-less CLI are different facts. Only the
		// former is an error; conflating them would record "no dial" for a
		// CLI we simply failed to ask.
		return nil, fmt.Errorf("%s --help: %w", cli, err)
	}
	return parseEffortEnum(out), nil
}

// parseEffortEnum extracts the effort rungs from help text.
//
// Two real formats must both work, and they differ in exactly the ways a naive
// parser gets wrong (both captured live):
//
//	claude:  --effort <level>   Effort level for the current session
//	                            (low, medium, high, xhigh, max)      <- WRAPPED
//	agy:     --effort           Reasoning effort ... (low|medium|high)  <- inline
//
// So the flag's block is accumulated across continuation lines, and stops at
// the next flag — which is what keeps a NEIGHBOURING flag's parenthetical
// (claude's --sandbox "(read-only, workspace-write, ...)") from being read as
// an effort ladder.
func parseEffortEnum(help string) []string {
	lines := strings.Split(help, "\n")
	start := -1
	for i, ln := range lines {
		if effortFlag.MatchString(strings.TrimSpace(ln)) {
			start = i
			break
		}
	}
	if start < 0 {
		return nil // no dial — a real answer, not a failure
	}
	block := []string{lines[start]}
	for _, ln := range lines[start+1:] {
		t := strings.TrimSpace(ln)
		if t == "" || flagStart.MatchString(t) {
			break // next flag (or a blank) ends this flag's block
		}
		block = append(block, t)
	}
	return firstEnumIn(strings.Join(block, " "))
}

// firstEnumIn returns the tokens of the first parenthetical that reads as an
// enum: at least two separator-delimited bare lowercase words. Requiring two
// keeps "(recommended)" and "(ccpool_...)" out, and requiring the token shape
// keeps prose out.
func firstEnumIn(s string) []string {
	for {
		open := strings.Index(s, "(")
		if open < 0 {
			return nil
		}
		closeIdx := strings.Index(s[open:], ")")
		if closeIdx < 0 {
			return nil
		}
		inner := s[open+1 : open+closeIdx]
		// enumTokens itself enforces the "at least two tokens" rule, so a
		// non-empty result is already an enum — restating the count here
		// would put the same invariant in two places.
		if toks := enumTokens(inner); len(toks) > 0 {
			return toks
		}
		s = s[open+closeIdx+1:]
	}
}

// enumTokens splits on either separator the CLIs use — ", " (claude) or "|"
// (agy) — and returns the tokens only if EVERY one is a bare lowercase word.
// All-or-nothing on purpose: a partial match means the parenthetical is prose
// that happens to contain a comma, and half an effort ladder is worse than
// none because it would silently narrow the discovered range.
func enumTokens(inner string) []string {
	fields := strings.FieldsFunc(inner, func(r rune) bool { return r == ',' || r == '|' })
	if len(fields) < 2 {
		return nil
	}
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		t := strings.TrimSpace(f)
		if !effortToken.MatchString(t) {
			return nil
		}
		out = append(out, t)
	}
	return out
}

// DefaultEffortListers is the single registry of which CLIs can have their
// ladder discovered from help text.
//
// codex is deliberately ABSENT. Its --help documents no reasoning effort at
// all (verified live on 0.147.0), so a help parse returns an empty ladder —
// indistinguishable from "codex has no dial", which is exactly the false
// negative this whole feature exists to remove. codex's ladder comes from the
// manifest it already declares, and the verify step is what proves those
// values are real.
func DefaultEffortListers() map[string]EffortLister {
	return map[string]EffortLister{
		"claude": HelpEffortLister{},
		"agy":    HelpEffortLister{},
	}
}
