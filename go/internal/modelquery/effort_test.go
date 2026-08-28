package modelquery

// effort_test.go — discovering each CLI's reasoning-effort ladder.
//
// Why this exists: the catalog refresh enumerated MODELS and nothing else, so
// when codex 0.147.0 grew two rungs above xhigh ("Max" and "Ultra", behind a
// "More reasoning..." submenu) nothing noticed. That is not cosmetic —
// realizeScalar silently DROPS an effort value absent from a manifest's values
// table, so an unmapped rung launches with no effort flag at all: the CLI's
// built-in preset, quietly weaker than intended and indistinguishable from
// success. Discovering the ladder is what turns that into a visible diff.
//
// The help text below is captured VERBATIM from the live CLIs, because the two
// formats differ in exactly the ways a naive parser gets wrong: claude wraps
// its enum onto a continuation line and separates with ", "; agy keeps it
// inline and separates with "|".

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Captured from `claude --help` on Claude Code v2.1.248. The enum is on the
// CONTINUATION line, not the flag line — a line-local parser finds nothing.
const realClaudeHelp = `  --debug                               Enable debug mode
  --effort <level>                      Effort level for the current session
                                        (low, medium, high, xhigh, max)
  --environment <environment_id>        Create a new cloud session that runs on
                                        the given self-hosted environment
                                        (ccpool_...).
`

// Captured from `agy --help` on Antigravity CLI 1.1.22. Inline, pipe-separated.
const realAgyHelp = `  --effort                        Reasoning effort for the current CLI session (low|medium|high)
  --model                         Model for the current CLI session
  models          List available models
`

func TestHelpEffortLister_ParsesBothRealHelpFormats(t *testing.T) {
	for _, tc := range []struct {
		cli  string
		help string
		want []string
	}{
		// The continuation-line case. If this returns nothing, the parser is
		// only looking at the --effort line itself.
		{"claude", realClaudeHelp, []string{"low", "medium", "high", "xhigh", "max"}},
		// The inline pipe-separated case.
		{"agy", realAgyHelp, []string{"low", "medium", "high"}},
	} {
		l := HelpEffortLister{Run: func(_ context.Context, name string, args []string, _ string) (string, error) {
			if len(args) != 1 || args[0] != "--help" {
				t.Fatalf("%s: want `--help`, got %v", name, args)
			}
			return tc.help, nil
		}}
		got, err := l.ListEfforts(context.Background(), tc.cli)
		if err != nil {
			t.Fatalf("%s: ListEfforts: %v", tc.cli, err)
		}
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: efforts = %v, want %v", tc.cli, got, tc.want)
		}
	}
}

// The enum must come from the --effort flag, never from some other flag's
// parenthetical. claude's --sandbox and codex's -s both carry "(possible
// values: ...)" lists that would poison a scan-any-parenthesis parser.
func TestHelpEffortLister_IgnoresOtherFlagsParentheticals(t *testing.T) {
	help := `  --sandbox <mode>    Sandbox policy
                      (read-only, workspace-write, danger-full-access)
  --effort <level>    Effort level
                      (low, high)
  --verbose <n>       Noise level (1, 2, 3)
`
	l := HelpEffortLister{Run: func(context.Context, string, []string, string) (string, error) {
		return help, nil
	}}
	got, err := l.ListEfforts(context.Background(), "claude")
	if err != nil {
		t.Fatalf("ListEfforts: %v", err)
	}
	if strings.Join(got, ",") != "low,high" {
		t.Fatalf("efforts = %v, want [low high] — a neighbouring flag's parenthetical leaked in", got)
	}
}

// A CLI with no --effort flag is not an error: ollama and any future dial-less
// CLI must yield an empty ladder and no failure, so the refresh records
// "this CLI has no dial" rather than "discovery broke".
func TestHelpEffortLister_NoEffortFlagIsEmptyNotError(t *testing.T) {
	l := HelpEffortLister{Run: func(context.Context, string, []string, string) (string, error) {
		return "  --model <m>   Model to use\n  --verbose     Chatty\n", nil
	}}
	got, err := l.ListEfforts(context.Background(), "ollama")
	if err != nil {
		t.Fatalf("a CLI without an effort dial must not error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("efforts = %v, want empty", got)
	}
}

func TestHelpEffortLister_PropagatesRunFailure(t *testing.T) {
	l := HelpEffortLister{Run: func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("command not found")
	}}
	if _, err := l.ListEfforts(context.Background(), "claude"); err == nil {
		t.Fatal("a failed --help must return an error, not an empty ladder — those mean different things")
	}
}

// codex does NOT publish its ladder in --help (verified live on 0.147.0: the
// rungs live only in the /model picker's "More reasoning..." submenu). Its
// ladder therefore comes from the manifest it already declares, and the verify
// step is what proves those values are real. Pinned so nobody "fixes"
// discovery for codex by scraping help text that has never contained it.
func TestDefaultEffortListers_CodexIsNotHelpDiscoverable(t *testing.T) {
	reg := DefaultEffortListers()
	for _, cli := range []string{"claude", "agy"} {
		if _, ok := reg[cli]; !ok {
			t.Errorf("%s publishes its effort enum in --help and must be registered", cli)
		}
	}
	if _, ok := reg["codex"]; ok {
		t.Error("codex must NOT use help discovery — its --help omits reasoning effort entirely, so a parse would silently yield an empty ladder and read as 'codex has no dial'")
	}
}

// --- wiring: the discovered ladder must reach the catalog ---

type fakeEfforts struct {
	rungs []string
	err   error
}

func (f fakeEfforts) ListEfforts(context.Context, string) ([]string, error) {
	return f.rungs, f.err
}

// A ladder discovered for a CLI must be RECORDED, or discovery is a no-op that
// looks like a feature. This is the wiring proof: without it, HelpEffortLister
// could be perfect and the catalog would still never learn a new rung existed.
func TestRefresh_RecordsDiscoveredEffortLadder(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"codex", "claude"},
		Lister:     fakeLister{ids: map[string][]string{"codex": {"gpt-5.4-mini", "gpt-5.4", "gpt-5.5"}, "claude": {"haiku", "sonnet", "opus"}}},
		Classifier: fakeClassifier{},
		Now:        fixedNow,
		EffortListers: map[string]EffortLister{
			"claude": fakeEfforts{rungs: []string{"low", "medium", "high", "xhigh", "max"}},
		},
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := cat.CLIs["claude"].Efforts; strings.Join(got, ",") != "low,medium,high,xhigh,max" {
		t.Errorf("claude efforts = %v, want the discovered ladder", got)
	}
	// codex has no registered lister (its ladder is not help-discoverable), so
	// it must record NOTHING rather than an empty-looking "no dial" claim that
	// a reader would mistake for a discovered fact.
	if got := cat.CLIs["codex"].Efforts; len(got) != 0 {
		t.Errorf("codex efforts = %v, want none recorded (not help-discoverable)", got)
	}
}

// A discovery failure must not sink the refresh: models are the primary
// payload, the ladder is an enrichment. Losing the whole catalog because
// `--help` hiccuped would be a worse failure than the one being prevented.
func TestRefresh_EffortDiscoveryFailureDoesNotSinkTheRefresh(t *testing.T) {
	deps := RefreshDeps{
		CLIs:       []string{"claude"},
		Lister:     fakeLister{ids: map[string][]string{"claude": {"haiku", "sonnet", "opus"}}},
		Classifier: fakeClassifier{},
		Now:        fixedNow,
		EffortListers: map[string]EffortLister{
			"claude": fakeEfforts{err: errors.New("help exploded")},
		},
	}
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("a failed effort discovery must not fail the refresh: %v", err)
	}
	if _, ok := cat.Lookup("claude", "deep"); !ok {
		t.Error("models must still be cataloged when only effort discovery failed")
	}
	if got := cat.CLIs["claude"].Efforts; len(got) != 0 {
		t.Errorf("efforts = %v, want none recorded on a discovery failure", got)
	}
}

// HIGH, found by adversarial review and REPRODUCED: the flag scan matched on
// prefix, so a neighbouring `--effort-budget` (or any --effort* flag) listed
// BEFORE the real one captured the scan, terminated on the next line, and
// returned an empty ladder — a false negative indistinguishable from "this CLI
// has no dial", which is precisely the bug class this feature exists to close.
// No shipping CLI trips it today; pinned so onboarding the next one cannot.
func TestHelpEffortLister_EffortPrefixedNeighbourDoesNotCaptureTheScan(t *testing.T) {
	help := `  --effort-budget <n>   Token budget for reasoning
                        (default: 4096)
  --effort <level>      Effort level for the current session
                        (low, medium, high, xhigh, max)
`
	l := HelpEffortLister{Run: func(context.Context, string, []string, string) (string, error) {
		return help, nil
	}}
	got, err := l.ListEfforts(context.Background(), "claude")
	if err != nil {
		t.Fatalf("ListEfforts: %v", err)
	}
	if strings.Join(got, ",") != "low,medium,high,xhigh,max" {
		t.Fatalf("efforts = %v, want the real --effort ladder — a --effort-PREFIXED neighbour captured the scan", got)
	}
}

// MEDIUM, same review: the block terminator treated ANY dash-leading line as
// the next flag, so a continuation line that merely starts with a dash (a
// bullet, or prose like "-1 disables it") truncated the block and produced an
// empty ladder. Only something shaped like a flag should end the block.
func TestHelpEffortLister_DashLeadingProseDoesNotTruncateTheBlock(t *testing.T) {
	help := `  --effort <level>   Effort level
                     -1 disables it entirely
                     (low, medium, high)
  --model <m>        Model to use
`
	l := HelpEffortLister{Run: func(context.Context, string, []string, string) (string, error) {
		return help, nil
	}}
	got, err := l.ListEfforts(context.Background(), "claude")
	if err != nil {
		t.Fatalf("ListEfforts: %v", err)
	}
	if strings.Join(got, ",") != "low,medium,high" {
		t.Fatalf("efforts = %v, want [low medium high] — dash-leading prose truncated the block", got)
	}
}
