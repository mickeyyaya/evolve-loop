// Package evalqualitycheck detects tautological / no-op evals before
// Builder commits to executing them. Per the skill docs (phase 2,
// discover): Level-0 commands (always-pass tautologies) trigger HALT
// because they let Builder claim PASS without doing any real work.
//
// The eval file is a Markdown doc under .evolve/evals/<slug>.md with
// frontmatter listing commands and expected outputs. The check parses
// the commands and flags:
//
//   - Level 0 (HALT): always-true tautologies like `:` `true` `[ true ]`
//     `exit 0`, empty commands, single-character no-ops.
//   - Level 1 (WARN): commands that test only string presence in their
//     own arguments (e.g., grep against an inlined constant), or echo-
//     only commands.
//   - Level 2 (PASS): commands that actually inspect the workspace
//     (run a build, touch a file, parse a config).
//
// Exit code policy (matches the bash script):
//
//   - 0 — PASS
//   - 1 — WARN (advisory; Builder proceeds with caution)
//   - 2 — HALT (Scout must rewrite the eval)
//
// v12.1 Phase 2A port. CLI: `evolve eval quality-check <eval.md>`.
package evalqualitycheck

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Level classifies an individual command's rigor.
type Level int

const (
	LevelPass Level = 0 // genuinely inspects the workspace
	LevelWarn Level = 1 // weak signal; advisory
	LevelHalt Level = 2 // no-op tautology; rewrite required
)

// Result is the overall verdict + per-command breakdown.
type Result struct {
	Path     string           // input file path
	Overall  Level            // worst classification across commands
	Commands []ClassifiedLine // one entry per parsed command line
}

// ClassifiedLine is a single command and its classification.
type ClassifiedLine struct {
	Line   string
	Level  Level
	Reason string
}

// Options configures Check. Path is required.
type Options struct {
	Path string
}

// Check reads the eval file at opts.Path and classifies every command
// it finds inside fenced ```bash blocks. Returns a Result with the
// worst-of classifications and per-command details. File-not-found is
// surfaced as an error (the caller decides whether that's HALT).
func Check(opts Options) (Result, error) {
	if opts.Path == "" {
		return Result{}, fmt.Errorf("evalqualitycheck: Path required")
	}
	raw, err := os.ReadFile(opts.Path)
	if err != nil {
		return Result{}, fmt.Errorf("evalqualitycheck: open %s: %w", opts.Path, err)
	}

	res := Result{Path: opts.Path, Overall: LevelPass}
	scoreCapGraded := scoreCapRE.Match(raw)

	cmds, err := scanBashCommands(strings.NewReader(string(raw)))
	if err != nil {
		return Result{}, fmt.Errorf("evalqualitycheck: read %s: %w", opts.Path, err)
	}
	for _, cmd := range cmds {
		cl := classify(cmd)
		res.Commands = append(res.Commands, cl)
		if cl.Level > res.Overall {
			res.Overall = cl.Level
		}
	}
	if len(cmds) == 0 {
		if scoreCapGraded {
			// score_cap/evidence-graded evals are consumed by the ACS suite,
			// not by this bash-rigor check — zero bash commands there is the
			// designed shape, not vacuity. Note it, don't cry wolf (the
			// alert-fatigue half of the adversarial-review MEDIUM finding).
			res.Commands = append(res.Commands, ClassifiedLine{
				Line: "(score_cap-graded eval)", Level: LevelPass,
				Reason: "no bash graders by design; scoring is consumed by the ACS suite",
			})
			return res, nil
		}
		// Zero parsed commands means this gate verified NOTHING. Returning
		// PASS here is the vacuity that silently defeated the gate for every
		// bullet-format eval until 2026-08-09 (ADR-0084 invariant 2) — a
		// format the scanner cannot read and an eval with no graders must
		// both surface, not slide through.
		res.Overall = LevelWarn
		res.Commands = append(res.Commands, ClassifiedLine{
			Line: "(no commands parsed)", Level: LevelWarn,
			Reason: "zero parsed commands — the gate verified nothing (no ```bash fence, no `[code]` grader bullet, no score_cap block)",
		})
	}
	return res, nil
}

// codeBulletRE matches the scout template's grader bullet form
// (agents/evolve-scout-reference.md, eval-format-template anchor):
//
//   - `[code]` `<command>`
//
// The command is the second backtick span (greedy: a command may itself
// contain backticks). [model]/[human] bullets are not bash and are not
// matched. 281 of the 625 live evals use this form — reading only ```bash
// fences left them all unscanned (the vacuous-gate class, ADR-0084).
var codeBulletRE = regexp.MustCompile("^-\\s*`\\[code\\]`\\s*`(.+)`\\s*$")

// scoreCapRE detects the score_cap eval form (line-anchored key), whose
// grading is consumed by the ACS suite rather than this bash-rigor scanner —
// zero bash commands there is designed, not vacuous.
var scoreCapRE = regexp.MustCompile(`(?m)^\s*score_cap\s*:`)

// scanBashCommands returns the non-blank, non-comment command lines found in
// r, in order — from ```bash fenced blocks AND from the template's
// `[code]` grader bullets. Shared by Check (single-file rigor) and
// CheckDiversity (suite-level diversity) so both parse evals identically.
func scanBashCommands(r io.Reader) ([]string, error) {
	var cmds []string
	inFence := false // inside ANY fenced block
	inBash := false  // inside a bash-tagged fenced block specifically
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				inFence, inBash = false, false
			} else {
				inFence = true
				inBash = strings.Contains(trimmed, "bash")
			}
			continue
		}
		if !inFence {
			// Grader bullets count ONLY at top level: a `[code]`-styled
			// bullet inside a text/markdown fence is illustration (or a
			// decoy planted to fake rigor — the adversarial-review BLOCK
			// finding), never a real command.
			if m := codeBulletRE.FindStringSubmatch(trimmed); m != nil {
				cmds = append(cmds, strings.TrimSpace(m[1]))
			}
			continue
		}
		if !inBash {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		cmds = append(cmds, trimmed)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return cmds, nil
}

// Patterns considered Level-0 (HALT): always-pass tautologies.
var (
	tautologyExactRE = regexp.MustCompile(`^(:|true|exit\s+0|/bin/true)\s*$`)
	tautologyBrackRE = regexp.MustCompile(`^\[\s+(true|1\s+-eq\s+1|"a"\s+=\s+"a")\s+\]\s*$`)
	echoOnlyRE       = regexp.MustCompile(`^echo\b`)
	// grepLiteralRE matches `grep <flags?> "<literal>" "<literal>"` —
	// two quoted args side-by-side, indicating grep against a string
	// inlined into the eval rather than a workspace file. RE2 has no
	// backreferences, so we accept any two adjacent quoted args.
	grepLiteralRE = regexp.MustCompile(`^grep\s+(-[a-zA-Z]+\s+)*["'][^"']+["']\s+["'][^"']+["']\s*$`)
	// commitPresenceRE matches `git log`/`git rev-list` invocations whose
	// arguments contain a revision range (`a..b` / `a...b` / trailing-open
	// `a..`), anywhere in the line (the RE is unanchored, so it also fires
	// inside $(...) and after && — the [^|&;]* guard only stops a range in
	// a LATER segment being attributed to this invocation). Such predicates
	// assert commit PRESENCE, which is structurally false after
	// worktree-normalize soft-resets builder commits to base before audit
	// (killed cycles 236/237). Boundaries require rev-name characters so
	// pathspecs like `dir/../file` don't fire. Deliberate gaps: left-open
	// `..HEAD` and `^main HEAD` exclusion syntax are not matched (rare in
	// eval predicates); `git diff a..b` is NOT matched — content parity is
	// the sanctioned pattern.
	commitPresenceRE = regexp.MustCompile(`\bgit\s+(log|rev-list)\b[^|&;]*[a-zA-Z0-9_~^@]\.{2,3}([a-zA-Z0-9_~^@]|\s|$)`)
)

// classify maps one command line to a Level + reason.
func classify(cmd string) ClassifiedLine {
	switch {
	case tautologyExactRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelHalt, Reason: "always-pass tautology"}
	case tautologyBrackRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelHalt, Reason: "trivial bracket test"}
	case commitPresenceRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelHalt, Reason: "commit-presence assertion (structurally false after worktree-normalize); assert content parity instead, e.g. `git diff <ref> --quiet`"}
	case echoOnlyRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelWarn, Reason: "echo-only (no workspace inspection)"}
	case grepLiteralRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelWarn, Reason: "grep against inlined literal"}
	}
	return ClassifiedLine{Line: cmd, Level: LevelPass, Reason: "non-trivial command"}
}
