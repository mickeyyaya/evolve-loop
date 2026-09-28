package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func TestShip_Decide_GitIsGitInAnyCase(t *testing.T) {
	for _, cmd := range []string{"GIT push origin main", "Git commit -m x", "/usr/bin/GIT push origin main"} {
		if dec := shipDecide(cmd); dec.Allow {
			t.Errorf("%q allowed: a case-insensitive filesystem runs it as git", cmd)
		}
	}
}

func TestShip_Decide_OnlyAPlainEvolveShipIsNative(t *testing.T) {
	for _, cmd := range []string{
		"env -S'git push origin HEAD:main' evolve ship",
		"env --split-string='git commit -m' evolve ship",
		"command git push origin main",
	} {
		if dec := shipDecide(cmd); dec.Allow {
			t.Errorf("%q allowed: a wrapper option can run git before evolve ship", cmd)
		}
	}
	for _, cmd := range []string{`FOO=1 evolve ship --class manual "m"`, `go/bin/evolve ship --class manual "m"`} {
		if dec := shipDecide(cmd); !dec.Allow {
			t.Errorf("%q denied: it is the native ship: %s", cmd, dec.Reason)
		}
	}
}

func shipDecide(cmd string) core.GuardDecision {
	in := core.GuardInput{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
	return NewShip(false).Decide(context.Background(), in)
}
