package looppreflight

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

const geminiFooter = "? for shortcuts      Gemini 3.1 Pro · high"

func TestRun_BridgeBoot_AModelMismatchWarnsWithItsUsageLineAndKeepsTheBatch(t *testing.T) {
	opts, _ := agyBootOptions(t, func(context.Context, string, bool) BootOutcome {
		return BootOutcome{RC: bridge.ExitModelMismatch, Scrollback: geminiFooter}
	})
	opts.UsageEvidence = func(string) string {
		return "quota exhausted, verified by a usage query: CLAUDE MODELS week window 100% used"
	}

	r, err := Run(opts)
	if err != nil {
		t.Fatal(err)
	}

	c := findCheck(t, r, "bridge-boot")
	if r.Halted() || c.Level != LevelWarn {
		t.Fatalf("level %s, halted %v: a REPL that booted another model family is the walk's failover, never a batch halt", c.Level, r.Halted())
	}
	for _, want := range []string{"ExitModelMismatch", "usage: quota exhausted", "Gemini 3.1 Pro · high"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail misses %q:\n%s", want, c.Detail)
		}
	}
}

func bootFailureOptions(t *testing.T, rc int, besideMismatch bool) Options {
	t.Helper()
	opts, _ := agyBootOptions(t, func(_ context.Context, driver string, _ bool) BootOutcome {
		if besideMismatch && driver == "agy-tmux" {
			return BootOutcome{RC: bridge.ExitModelMismatch, Scrollback: geminiFooter}
		}
		return BootOutcome{RC: rc, Scrollback: "claude"}
	})
	if besideMismatch {
		opts.ProfileLister = func() ([]string, error) { return []string{"builder", "auditor"}, nil }
		opts.ProfileGetter = func(name string) (profiles.Profile, error) {
			if name == "auditor" {
				return profiles.Profile{Name: name, CLI: "claude-tmux"}, nil
			}
			return profiles.Profile{Name: name, CLI: "agy-tmux"}, nil
		}
	}
	return opts
}

func TestRun_BridgeBoot_EveryBootFailureHaltsAloneAndBesideAMismatch(t *testing.T) {
	failures := map[string]int{
		"REPL boot timeout":    bridge.ExitREPLBootTimeout,
		"artifact timeout":     bridge.ExitArtifactTimeout,
		"missing binary":       bridge.ExitMissingBinary,
		"bad flags":            bridge.ExitBadFlags,
		"workspace setup (-1)": exitWorkspaceSetupFailed,
	}
	for name, rc := range failures {
		for _, besideMismatch := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s beside a mismatch=%v", name, besideMismatch), func(t *testing.T) {
				r, err := Run(bootFailureOptions(t, rc, besideMismatch))
				if err != nil {
					t.Fatal(err)
				}

				c := findCheck(t, r, "bridge-boot")
				if !r.Halted() || c.Level != LevelHalt || !strings.Contains(c.Detail, fmt.Sprintf("rc=%d ", rc)) {
					t.Fatalf("level %s, halted %v: rc=%d is a boot failure that halts the batch; only 87 is the walk's failover\n%s", c.Level, r.Halted(), rc, c.Detail)
				}
				if besideMismatch && !strings.Contains(c.Detail, "ExitModelMismatch") {
					t.Fatalf("the mismatch beside the failure is not reported:\n%s", c.Detail)
				}
			})
		}
	}
}
