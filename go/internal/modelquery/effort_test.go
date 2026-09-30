package modelquery

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const realClaudeHelp = `  --debug                               Enable debug mode
  --effort <level>                      Effort level for the current session
                                        (low, medium, high, xhigh, max)
  --environment <environment_id>        Create a new cloud session that runs on
                                        the given self-hosted environment
                                        (ccpool_...).
`

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
		{"claude", realClaudeHelp, []string{"low", "medium", "high", "xhigh", "max"}},
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

type fakeEfforts struct {
	rungs []string
	err   error
}

func (f fakeEfforts) ListEfforts(context.Context, string) ([]string, error) {
	return f.rungs, f.err
}

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
	if got := cat.CLIs["codex"].Efforts; len(got) != 0 {
		t.Errorf("codex efforts = %v, want none recorded (not help-discoverable)", got)
	}
}

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
