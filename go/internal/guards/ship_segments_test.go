package guards

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func decideShip(t *testing.T, cmd string) core.GuardDecision {
	t.Helper()
	return NewShip(false).Decide(context.Background(), core.GuardInput{
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": cmd},
	})
}

func TestShip_ANativeShipElsewhereInTheCommandAllowsNothingElse(t *testing.T) {
	for _, cmd := range []string{
		"git push origin main; echo evolve ship",
		"echo evolve ship && git push origin main",
		"git commit -m x || evolve ship --class manual msg",
		"evolve ship --class manual msg\ngit push origin main",
		"git push origin main & evolve ship --class manual msg",
	} {
		if dec := decideShip(t, cmd); dec.Allow {
			t.Errorf("%q allowed: the ship verb runs as its own command, not through evolve ship", cmd)
		}
	}
}

func TestShip_ACommandSubstitutionInsideANativeShipStillRuns(t *testing.T) {
	for _, cmd := range []string{
		`evolve ship --class manual "$(git push origin main)"`,
		"evolve ship --class manual \"`git push origin main`\"",
		`evolve ship --class manual $(git commit -m x)`,
	} {
		if dec := decideShip(t, cmd); dec.Allow {
			t.Errorf("%q allowed: the shell runs the substitution before evolve ship starts", cmd)
		}
	}
}

func TestShip_OnlyAnUnquotedHeredocOpenerHidesTheLinesAfterIt(t *testing.T) {
	for _, cmd := range []string{
		"echo '<<EOF'\ngit push origin main\nEOF",
		"echo \"<<EOF\"\ngit push origin main\nEOF",
		"cat <<< EOF\ngit push origin main\nEOF",
		"echo hi # <<EOF\ngit push origin main\nEOF",
		"echo $((x<<y))\ngit push origin main\ny",
		"((x<<y))\ngit push origin main\ny",
	} {
		if dec := decideShip(t, cmd); dec.Allow {
			t.Errorf("%q allowed: no heredoc opens there, so the shell runs the git push line", cmd)
		}
	}
}

func TestShip_GitGlobalOptionsDoNotHideTheSubcommand(t *testing.T) {
	for _, cmd := range []string{
		"git -C . push origin main",
		"git -c user.name=x commit -m y",
		"git --git-dir=.git --work-tree . push",
		"env GIT_TRACE=1 git -C /repo push",
	} {
		if dec := decideShip(t, cmd); dec.Allow {
			t.Errorf("%q allowed: git's global options come before the subcommand, which is still %s", cmd, "a ship verb")
		}
	}
	if dec := decideShip(t, "git -C . status"); !dec.Allow {
		t.Errorf("git -C . status denied: %s", dec.Reason)
	}
}

// bash closes each of these heredocs at its marker line and then runs the git push after it.
func TestShip_AHeredocMarkerIsTheWholeShellWord(t *testing.T) {
	for _, cmd := range []string{
		"cat <<EOF-MARKER\ndata\nEOF-MARKER\ngit push origin main",
		"cat <<'END.'\ndata\nEND.\ngit push origin main",
		"cat <<1X\ndata\n1X\ngit push origin main",
		"cat <<\"a\\b\"\ndata\na\\b\ngit push origin main",
		"cat <<EO\\\nF\ndata\nEOF\ngit push origin main",
		"cat <<EOF\r\ndata\r\nEOF\r\ngit push origin main",
	} {
		if dec := decideShip(t, cmd); dec.Allow {
			t.Errorf("%q allowed: its heredoc closes before the git push line, which bash runs", cmd)
		}
	}
	if dec := decideShip(t, "cat <<'EOF-MSG'\ngit push origin main\nEOF-MSG\necho done"); !dec.Allow {
		t.Errorf("a ship verb inside an EOF-MSG heredoc body is data, not a command; got DENY: %s", dec.Reason)
	}
}

func TestShip_ShipShIsNoLongerAShipPath(t *testing.T) {
	cmd := `bash legacy/scripts/lifecycle/ship.sh "msg" && git push origin main`
	if dec := decideShip(t, cmd); dec.Allow {
		t.Errorf("%q allowed: ship.sh is gone, and a script at that path is whatever an agent wrote", cmd)
	}
}

func TestShip_ANativeShipMayMentionShipVerbs(t *testing.T) {
	for _, cmd := range []string{
		`EVOLVE_SHIP_AUTO_CONFIRM=1 ./go/bin/evolve ship --class manual "fix: git push races" && echo done`,
		`evolve ship --class manual 'git commit -m and $(git push) stay literal here'`,
		"evolve ship --class manual \"$(cat <<'EOF'\nfix: \"quoted\" git push; git commit\nEOF\n)\"",
		`evolve ship --class manual "$(cat msg.txt)"`,
	} {
		if dec := decideShip(t, cmd); !dec.Allow {
			t.Errorf("%q denied: %s", cmd, dec.Reason)
		}
	}
}
