package bridge

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

type runCall struct {
	name  string
	args  []string
	stdin string
}

type fakeRun struct {
	out   map[string]string
	err   error
	calls atomic.Int32
	last  runCall
}

func (f *fakeRun) run(_ context.Context, name string, args []string, stdin string) (string, error) {
	f.calls.Add(1)
	f.last = runCall{name: name, args: args, stdin: stdin}
	return f.out[name], f.err
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func manifestSet(manifests map[string]Manifest) DriverSet {
	names := make([]string, 0, len(manifests))
	for name := range manifests {
		names = append(names, name)
	}
	return NewDriverSet(names, func(name string) (Manifest, error) {
		m, ok := manifests[name]
		if !ok {
			return Manifest{}, errors.New("no manifest " + name)
		}
		return m, nil
	})
}

func modelsManifest(binary, id string) Manifest {
	return Manifest{Binary: binary, Parsers: map[string]string{ParserModels: id}}
}

func TestModelLister_AgyFixtureGivesTheDisplayNamesThroughTheManifestCommand(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"agy": fixture(t, "agy_models.txt")}}
	got, err := NewModelLister(DefaultDriverSet(), f.run).List(context.Background(), "agy")
	want := []string{"Gemini 3.7 Flash (High)", "Gemini 3.7 Flash (Medium)", "Gemini 3.7 Flash (Low)", "Gemini 3.1 Pro (High)", "Gemini 3.1 Pro (Low)", "Claude Sonnet 4.6 (Thinking)", "GPT-OSS 120B (Medium)"}
	if err != nil || !slices.Equal(got, want) {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
	if !reflect.DeepEqual(f.last, runCall{name: "agy", args: []string{"models"}}) {
		t.Errorf("run = %+v; want agy models with no stdin", f.last)
	}
}

func TestModelLister_OllamaFixtureGivesTheModelIDs(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"ollama": fixture(t, "ollama_list.txt")}}
	got, err := NewModelLister(DefaultDriverSet(), f.run).List(context.Background(), "ollama")
	if err != nil || !slices.Equal(got, []string{"gemma4:latest", "phi4:latest"}) {
		t.Fatalf("got %q, %v", got, err)
	}
	if !reflect.DeepEqual(f.last, runCall{name: "ollama", args: []string{"list"}}) {
		t.Errorf("run = %+v; want ollama list with no stdin", f.last)
	}
}

func TestModelLister_AnOllamaWithNoModelIsAnEmptyListNotAnError(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"ollama": "NAME ID SIZE MODIFIED\n"}}
	got, err := NewModelLister(DefaultDriverSet(), f.run).List(context.Background(), "ollama")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %q, %v; want an empty list", got, err)
	}
}

func TestModelLister_AgyNoiseAndHeaderRowsNeverBecomeModels(t *testing.T) {
	t.Parallel()
	out := "ID\tDISPLAY NAME\nMODEL\tNAME\nFetching available models...\n\n\ngemini-3.7-flash-low\tGemini 3.7 Flash (Low)\nx\t \n"
	f := &fakeRun{out: map[string]string{"agy": out}}
	got, err := NewModelLister(DefaultDriverSet(), f.run).List(context.Background(), "agy")
	if err != nil || !slices.Equal(got, []string{"Gemini 3.7 Flash (Low)"}) {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestModelLister_AgyWithNoModelRowIsAnErrorThatNamesTheCommand(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"agy": "no model rows"}}
	_, err := NewModelLister(DefaultDriverSet(), f.run).List(context.Background(), "agy")
	var listing *modelListingError
	if !errors.As(err, &listing) || listing.Command != "agy models" || !errors.Is(err, errNoModelRows) {
		t.Fatalf("err = %v; want a listing error for agy models that wraps errNoModelRows", err)
	}
}

func TestModelLister_AFailureCarriesTheFirst200CharactersOfOutput(t *testing.T) {
	t.Parallel()
	runErr := errors.New("exit status 1")
	cases := map[string]string{strings.Repeat("é", 250): strings.Repeat("é", modelquery.OutputClip) + "…", "could not connect": "could not connect"}
	for out, want := range cases {
		f := &fakeRun{out: map[string]string{"ollama": out}, err: runErr}
		got, err := NewModelLister(DefaultDriverSet(), f.run).List(context.Background(), "ollama")
		var listing *modelListingError
		if got != nil || !errors.As(err, &listing) || listing.Output != want || listing.Command != "ollama list" || !errors.Is(err, runErr) {
			t.Errorf("got %v, err %v; want nil and the output clipped to %d runes", got, err, modelquery.OutputClip)
		}
	}
}

func TestModelLister_ACallerEditingItsListingLeavesTheSharedListingIntact(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"agy": fixture(t, "agy_models.txt")}}
	l := NewModelLister(DefaultDriverSet(), f.run)
	first, err := l.List(context.Background(), "agy")
	if err != nil {
		t.Fatal(err)
	}
	first[0] = "edited"
	second, err := l.List(context.Background(), "agy-claude")
	if err != nil || second[0] == "edited" {
		t.Fatalf("second = %q, %v; the two families share one backing array", second, err)
	}
}

func TestModelLister_ARefusedFamilyStartsNoProcess(t *testing.T) {
	t.Parallel()
	loadFails := NewDriverSet([]string{"broken-tmux"}, func(string) (Manifest, error) { return Manifest{}, errors.New("bad manifest") })
	cases := map[string]DriverSet{"claude": DefaultDriverSet(), "nothere": DefaultDriverSet(), "broken": loadFails}
	for cli, set := range cases {
		f := &fakeRun{}
		if _, err := NewModelLister(set, f.run).List(context.Background(), cli); err == nil || f.calls.Load() != 0 {
			t.Errorf("%s: err=%v calls=%d; want an error and no process", cli, err, f.calls.Load())
		}
	}
}

func TestModelLister_AgyAndAgyClaudeShareOneListing(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"agy": fixture(t, "agy_models.txt")}}
	l := NewModelLister(DefaultDriverSet(), f.run)
	for _, cli := range []string{"agy", "agy-claude"} {
		if got, err := l.List(context.Background(), cli); err != nil || len(got) != 7 {
			t.Fatalf("%s: got %q, %v", cli, got, err)
		}
	}
	if f.calls.Load() != 1 {
		t.Errorf("agy models ran %d times, want once", f.calls.Load())
	}
}

func TestModelLister_AFailedListingFailsEveryFamilyThatSharesIt(t *testing.T) {
	t.Parallel()
	f := &fakeRun{out: map[string]string{"agy": "no model rows"}}
	l := NewModelLister(DefaultDriverSet(), f.run)
	for _, cli := range []string{"agy", "agy-claude"} {
		if _, err := l.List(context.Background(), cli); err == nil {
			t.Errorf("%s listed models from an empty listing", cli)
		}
	}
	if f.calls.Load() != 1 {
		t.Errorf("agy models ran %d times, want once", f.calls.Load())
	}
}

func TestModelLister_ADifferentBinaryAndArgvKeyGetsItsOwnListing(t *testing.T) {
	t.Parallel()
	set := manifestSet(map[string]Manifest{
		"agy-tmux":  modelsManifest("agy", "agy-models-v1"),
		"agyb-tmux": modelsManifest("agyb", "agy-models-v1"),
		"oll-tmux":  modelsManifest("agy", "ollama-list-v1"),
		"agy2-tmux": modelsManifest("agy", "agy-models-v1"),
	})
	f := &fakeRun{out: map[string]string{"agy": "x\tM (Low)\nNAME\nm1\n", "agyb": "x\tOther (High)\n"}}
	l := NewModelLister(set, f.run)
	for _, cli := range []string{"agy2", "agyb", "oll"} {
		if _, err := l.List(context.Background(), cli); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := l.List(context.Background(), "oll"); err != nil {
		t.Fatal(err)
	}
	if f.calls.Load() != 3 {
		t.Errorf("runs = %d, want 3: one per distinct (binary, argv), one cached", f.calls.Load())
	}
}

func TestModelLister_FamiliesComeFromTheManifestsNotFromAList(t *testing.T) {
	t.Parallel()
	real := NewModelLister(DefaultDriverSet(), nil).Families()
	if !slices.Equal(real, []string{"agy", "agy-claude", "ollama"}) {
		t.Fatalf("real families = %v", real)
	}
	set := manifestSet(map[string]Manifest{
		"agy-tmux":    modelsManifest("agy", "agy-models-v1"),
		"newcli-tmux": modelsManifest("newcli", "ollama-list-v1"),
		"claude-tmux": {Binary: "claude"},
		"bad-tmux":    modelsManifest("bad", "no-such-id"),
	})
	got := NewModelLister(set, nil).Families()
	if !slices.Equal(got, []string{"agy", "newcli"}) {
		t.Fatalf("families = %v; want the two that declare a known models id, a third family included", got)
	}
}

func TestModelLister_AThirdFamilyThatDeclaresAFormatRoutesToTheAdapter(t *testing.T) {
	t.Parallel()
	set := manifestSet(map[string]Manifest{"newcli-tmux": modelsManifest("newcli", "ollama-list-v1")})
	f := &fakeRun{out: map[string]string{"newcli": "NAME\nm1:latest x\n"}}
	got, err := NewModelLister(set, f.run).List(context.Background(), "newcli")
	if err != nil || !slices.Equal(got, []string{"m1:latest"}) || f.last.name != "newcli" || !slices.Equal(f.last.args, []string{"list"}) {
		t.Fatalf("got %q, %v, run %+v", got, err, f.last)
	}
}

func TestModelListingError_TextNamesTheCommandTheCauseAndTheOutput(t *testing.T) {
	t.Parallel()
	cause := errors.New("exit status 1")
	withOutput := &modelListingError{Command: "ollama list", Output: "no server", Err: cause}
	parseOnly := &modelListingError{Command: "agy models", Err: cause}
	wantWith := "ollama list: exit status 1: no server"
	wantBare := "agy models: exit status 1"
	if got := withOutput.Error(); got != wantWith {
		t.Errorf("with output: %q, want %q", got, wantWith)
	}
	if got := parseOnly.Error(); got != wantBare {
		t.Errorf("without output: %q, want %q", got, wantBare)
	}
}
