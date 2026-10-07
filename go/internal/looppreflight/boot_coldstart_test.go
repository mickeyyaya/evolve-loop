package looppreflight

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func agyBootOptions(t *testing.T, tester func(ctx context.Context, driver string, sandbox bool) BootOutcome) (Options, *bytes.Buffer) {
	t.Helper()
	var stderr bytes.Buffer
	opts := goodPipelineOptions(t)
	opts.SkipBoot = false
	opts.Stderr = &stderr
	opts.BootBudget = time.Hour
	opts.ProfileGetter = func(name string) (profiles.Profile, error) {
		return profiles.Profile{Name: name, CLI: "agy-tmux"}, nil
	}
	opts.BootTester = tester
	return opts, &stderr
}

func TestRun_BridgeBoot_AnAgyColdStartThatBootsOnTheRetryPasses(t *testing.T) {
	var budgets []time.Duration
	opts, stderr := agyBootOptions(t, func(ctx context.Context, driver string, _ bool) BootOutcome {
		deadline, _ := ctx.Deadline()
		budgets = append(budgets, time.Until(deadline))
		if len(budgets) == 1 {
			return BootOutcome{RC: bridge.ExitREPLBootTimeout, Scrollback: "Antigravity CLI\n  signing in…"}
		}
		return BootOutcome{RC: bridge.ExitOK, Scrollback: ""}
	})

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if c := findCheck(t, r, "bridge-boot"); c.Level != LevelPass {
		t.Fatalf("an agy whose first boot is slow and whose retry boots must pass; got %s (%s)", c.Level, c.Detail)
	}
	if len(budgets) != 2 {
		t.Fatalf("%d agy boots; want the cold first boot retried once", len(budgets))
	}
	if budgets[1] < 59*time.Minute {
		t.Errorf("the retry ran on %v of a 1h budget; each attempt gets the whole boot budget", budgets[1])
	}
	if !strings.Contains(stderr.String(), "[agy-tmux] cold start: boot attempt 1 of 2") {
		t.Errorf("the preflight log does not name the cold start:\n%s", stderr)
	}
}

func TestRun_BridgeBoot_AnAgyThatNeverBootsHaltsAfterBothAttemptsWithThePane(t *testing.T) {
	launches := 0
	opts, stderr := agyBootOptions(t, func(context.Context, string, bool) BootOutcome {
		launches++
		return BootOutcome{RC: bridge.ExitREPLBootTimeout, Scrollback: "Antigravity CLI\n  signing in…"}
	})

	r, err := Run(opts)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	c := findCheck(t, r, "bridge-boot")
	if c.Level != LevelHalt || launches != 2 {
		t.Fatalf("level=%s after %d boot(s); a second timeout must still halt", c.Level, launches)
	}
	for _, want := range []string{`"agy-tmux"`, "signing in", "on all 2 boot attempts"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("the halt detail lacks %q:\n%s", want, c.Detail)
		}
	}
	if !strings.Contains(stderr.String(), "FAIL: the REPL never drew its prompt on any of 2 boot attempts") {
		t.Errorf("the second timeout is not logged loudly:\n%s", stderr)
	}
}

func TestRun_BridgeBoot_AFailedBootCarriesTheUsageVerdictAndAPassAsksNothing(t *testing.T) {
	for verdict, summary := range map[string]string{
		"healthy":     "quota ruled out by a usage query: GEMINI MODELS week window 8.3% used",
		"exhausted":   "quota exhausted, verified by a usage query: CLAUDE AND GPT MODELS 5h window 100% used",
		"unavailable": "the usage query itself failed (boot timed out), which points at an auth, install or network cause",
	} {
		var asked []string
		opts, _ := agyBootOptions(t, func(context.Context, string, bool) BootOutcome {
			return BootOutcome{RC: bridge.ExitREPLBootTimeout, Scrollback: "Antigravity CLI\n  signing in…"}
		})
		opts.UsageEvidence = func(driver string) string { asked = append(asked, driver); return summary }

		r, err := Run(opts)
		if err != nil {
			t.Fatal(err)
		}

		c := findCheck(t, r, "bridge-boot")
		if c.Level != LevelHalt || !strings.Contains(c.Detail, "usage: "+summary) || len(asked) != 1 || asked[0] != "agy-tmux" {
			t.Errorf("%s: level %s, asked %v, detail:\n%s", verdict, c.Level, asked, c.Detail)
		}
	}
	asked := 0
	opts, _ := agyBootOptions(t, func(context.Context, string, bool) BootOutcome { return BootOutcome{RC: bridge.ExitOK, Scrollback: ""} })
	opts.UsageEvidence = func(string) string { asked++; return "" }
	if _, err := Run(opts); err != nil || asked != 0 {
		t.Errorf("a booted driver asked for usage %d time(s) (err %v)", asked, err)
	}
}

func TestRun_BridgeBoot_ABootTimeoutCarryingAWallIsNotRetriedAndNamesTheWall(t *testing.T) {
	launches := 0
	opts, stderr := agyBootOptions(t, func(context.Context, string, bool) BootOutcome {
		launches++
		return BootOutcome{RC: bridge.ExitREPLBootTimeout, Wall: "auth_recheck", Scrollback: "Please log in"}
	})

	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}

	c := findCheck(t, r, "bridge-boot")
	if c.Level != LevelHalt || launches != 1 {
		t.Fatalf("level %s after %d boot(s); a login wall at boot is no cold start, so it is not retried", c.Level, launches)
	}
	if !strings.Contains(c.Detail, "auth_recheck") || strings.Contains(c.Detail, "boot attempts") || strings.Contains(stderr.String(), "cold start") {
		t.Errorf("the halt must name the wall and claim no retries:\n%s\n%s", c.Detail, stderr)
	}
}
