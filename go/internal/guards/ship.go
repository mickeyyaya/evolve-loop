package guards

import (
	"context"
	"path"
	"regexp"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Ship denies ship-class Bash commands (git commit, git push, gh release) that bypass `evolve ship`.
type Ship struct {
	bypass bool
}

// NewShip returns a Ship guard; bypass allows every call.
func NewShip(bypass bool) *Ship { return &Ship{bypass: bypass} }

// Name reports "ship".
func (s *Ship) Name() string { return "ship" }

var (
	shipVerbRe        = regexp.MustCompile(`\b(git[ \t]+commit|git[ \t]+push|gh[ \t]+release[ \t]+(create|edit))\b`)
	shellAssignmentRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*=`)
)

// gitOptionsWithValue are git's global options that take the next word as their value.
var gitOptionsWithValue = map[string]bool{
	"-C": true, "-c": true, "--git-dir": true, "--work-tree": true,
	"--namespace": true, "--super-prefix": true, "--config-env": true,
}

// Decide denies a command line in which any simple command is ship-class without itself being `evolve ship`.
// Each command is judged on its own, so an `evolve ship` elsewhere on the line allows nothing else.
func (s *Ship) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if s.bypass || in.ToolName != "Bash" {
		return core.GuardDecision{Allow: true}
	}
	for _, c := range splitShellCommands(cmdString(in)) {
		if isShipClass(c) && !isNativeShip(c.words) {
			return core.GuardDecision{
				Allow:  false,
				Reason: "ship-class command must invoke the native 'evolve ship' CLI; pass --bypass to the guard for emergencies",
			}
		}
	}
	return core.GuardDecision{Allow: true}
}

// isShipClass reports whether a command commits, pushes or publishes a release, whether spelled in its
// text or reached through git's global options, as in `git -C dir push`.
func isShipClass(c shellCommand) bool {
	if shipVerbRe.MatchString(c.text) {
		return true
	}
	for i, w := range c.words {
		if path.Base(w) != "git" {
			continue
		}
		if sub := gitSubcommand(c.words[i+1:]); sub == "commit" || sub == "push" {
			return true
		}
	}
	return false
}

func gitSubcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		if !strings.HasPrefix(args[i], "-") {
			return args[i]
		}
		if gitOptionsWithValue[args[i]] {
			i++
		}
	}
	return ""
}

// isNativeShip reports whether the words run `evolve ship`, after any leading VAR=value assignments.
func isNativeShip(words []string) bool {
	for len(words) > 0 && shellAssignmentRe.MatchString(words[0]) {
		words = words[1:]
	}
	return len(words) >= 2 && path.Base(words[0]) == "evolve" && words[1] == "ship"
}
