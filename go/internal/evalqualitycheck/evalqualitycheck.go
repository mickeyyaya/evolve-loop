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
	f, err := os.Open(opts.Path)
	if err != nil {
		return Result{}, fmt.Errorf("evalqualitycheck: open %s: %w", opts.Path, err)
	}
	defer f.Close()

	res := Result{Path: opts.Path, Overall: LevelPass}

	cmds, err := scanBashCommands(f)
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
	return res, nil
}

// scanBashCommands returns the non-blank, non-comment command lines found
// inside ```bash fenced blocks of r, in order. Shared by Check (single-file
// rigor) and CheckDiversity (suite-level diversity) so both parse evals
// identically.
func scanBashCommands(r io.Reader) ([]string, error) {
	var cmds []string
	inBash := false
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		trimmed := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(trimmed, "```") {
			// Opening: ```bash sets inBash=true. Closing (```) or any
			// non-bash fence sets inBash=false because "bash" is absent.
			inBash = strings.Contains(trimmed, "bash") && !inBash
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
)

// classify maps one command line to a Level + reason.
func classify(cmd string) ClassifiedLine {
	switch {
	case tautologyExactRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelHalt, Reason: "always-pass tautology"}
	case tautologyBrackRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelHalt, Reason: "trivial bracket test"}
	case echoOnlyRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelWarn, Reason: "echo-only (no workspace inspection)"}
	case grepLiteralRE.MatchString(cmd):
		return ClassifiedLine{Line: cmd, Level: LevelWarn, Reason: "grep against inlined literal"}
	}
	return ClassifiedLine{Line: cmd, Level: LevelPass, Reason: "non-trivial command"}
}
