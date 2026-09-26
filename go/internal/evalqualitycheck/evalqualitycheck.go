// Package evalqualitycheck runs deterministic authoring-time checks on evals and ACS predicates:
// single-eval rigor (Check), suite diversity (CheckDiversity) and flaky shapes (LintFlakyPredicates).
// See docs/architecture/packages/internal-evalqualitycheck.md.
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

// Levels in rising severity; Check reports the worst one.
const (
	LevelPass Level = 0 // genuinely inspects the workspace
	LevelWarn Level = 1 // weak signal; advisory
	LevelHalt Level = 2 // no-op tautology; rewrite required
)

// Result is the overall verdict + per-command breakdown.
type Result struct {
	Path     string
	Overall  Level            // worst classification across commands
	Commands []ClassifiedLine // one per parsed command, or one diagnostic entry when none parse
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

// Check classifies every ```bash-fence and `[code]`-bullet command in the eval at opts.Path.
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
			// The ACS suite grades score_cap evals, so zero bash commands is their designed shape.
			res.Commands = append(res.Commands, ClassifiedLine{
				Line: "(score_cap-graded eval)", Level: LevelPass,
				Reason: "no bash graders by design; scoring is consumed by the ACS suite",
			})
			return res, nil
		}
		// Zero parsed commands means the gate verified nothing, so it must never read as PASS.
		res.Overall = LevelWarn
		res.Commands = append(res.Commands, ClassifiedLine{
			Line: "(no commands parsed)", Level: LevelWarn,
			Reason: "zero parsed commands — the gate verified nothing (no ```bash fence, no `[code]` grader bullet, no score_cap block)",
		})
	}
	return res, nil
}

// codeBulletRE matches the scout template's `[code]` grader bullet; the greedy
// capture lets the command itself contain backticks.
var codeBulletRE = regexp.MustCompile("^-\\s*`\\[code\\]`\\s*`(.+)`\\s*$")

var scoreCapRE = regexp.MustCompile(`(?m)^\s*score_cap\s*:`)

// scanBashCommands is the one eval parser Check and CheckDiversity share, so both read evals identically.
func scanBashCommands(r io.Reader) ([]string, error) {
	var cmds []string
	inFence := false
	inBash := false
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
			// Grader bullets count only at top level: a fenced one is illustration or a planted decoy.
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

var (
	tautologyExactRE = regexp.MustCompile(`^(:|true|exit\s+0|/bin/true)\s*$`)
	tautologyBrackRE = regexp.MustCompile(`^\[\s+(true|1\s+-eq\s+1|"a"\s+=\s+"a")\s+\]\s*$`)
	echoOnlyRE       = regexp.MustCompile(`^echo\b`)
	// RE2 has no backreferences, so any two adjacent quoted args count as a grep against an inlined literal.
	grepLiteralRE = regexp.MustCompile(`^grep\s+(-[a-zA-Z]+\s+)*["'][^"']+["']\s+["'][^"']+["']\s*$`)
	// A git log/rev-list range asserts commit presence, which is structurally false once
	// worktree-normalize soft-resets builder commits to base before audit.
	commitPresenceRE = regexp.MustCompile(`\bgit\s+(log|rev-list)\b[^|&;]*[a-zA-Z0-9_~^@]\.{2,3}([a-zA-Z0-9_~^@]|\s|$)`)
)

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
