package bridge

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/llmroute"
)

func TestResolve_EveryInteractiveManifestResolvesFromItsFamily(t *testing.T) {
	t.Parallel()
	set := DefaultDriverSet()
	want := map[string]string{
		"claude":     "claude-tmux",
		"codex":      "codex-tmux",
		"agy":        "agy-tmux",
		"agy-claude": "agy-claude-tmux",
		"ollama":     "ollama-tmux",
	}
	for family, driver := range want {
		got, err := Resolve(set, family)
		if err != nil {
			t.Errorf("Resolve(%q) err=%v, want the %s driver", family, err, driver)
			continue
		}
		if got.Driver != driver || got.Manifest.CLI != driver {
			t.Errorf("Resolve(%q) = driver %q manifest %q, want %q for both", family, got.Driver, got.Manifest.CLI, driver)
		}
	}
	for _, name := range ManifestNames() {
		m, err := LoadManifest(name)
		if err != nil || !m.IsTmux() {
			continue
		}
		if !slices.Contains(mapValues(want), name) {
			t.Errorf("the tmux manifest %s has no family in this test: add it", name)
		}
	}
}

func TestResolve_AnUnknownFamilyIsATypedErrorAndNoGuessedDriver(t *testing.T) {
	t.Parallel()
	got, err := Resolve(DefaultDriverSet(), "gemini")

	var unknown *UnknownFamilyError
	if !errors.As(err, &unknown) || unknown.Family != "gemini" {
		t.Fatalf("err=%v, want an *UnknownFamilyError for gemini", err)
	}
	if !strings.Contains(err.Error(), `"gemini"`) {
		t.Errorf("err=%q, want the family named", err)
	}
	if got.Driver != "" {
		t.Errorf("driver=%q, want none: an unknown family gets no guessed name", got.Driver)
	}
}

func TestResolve_AHeadlessManifestNameIsNotAFamily(t *testing.T) {
	t.Parallel()
	set := NewDriverSet([]string{"claude-p", "claude-tmux"}, LoadManifest)

	_, err := Resolve(set, "claude-p")

	var unknown *UnknownFamilyError
	if !errors.As(err, &unknown) {
		t.Fatalf("err=%v, want claude-p unknown: only an interactive driver names a family", err)
	}
}

func TestResolve_AManifestThatFailsToLoadKeepsTheDriverAndWrapsTheCause(t *testing.T) {
	t.Parallel()
	cause := errors.New("bad manifest")
	set := NewDriverSet([]string{"fake-tmux"}, func(string) (Manifest, error) { return Manifest{}, cause })

	got, err := Resolve(set, "fake")

	if !errors.Is(err, cause) {
		t.Fatalf("err=%v, want the load error wrapped", err)
	}
	if got.Driver != "fake-tmux" {
		t.Errorf("driver=%q, want fake-tmux", got.Driver)
	}
}

func TestResolve_TheSetLoadsTheManifestThroughItsInjectedLoader(t *testing.T) {
	t.Parallel()
	var asked string
	var set DriverSet = NewDriverSet([]string{"fake-tmux"}, func(cli string) (Manifest, error) {
		asked = cli
		return Manifest{CLI: cli, Binary: "fake", ModelFamily: "local"}, nil
	})

	var got Resolved
	got, err := Resolve(set, "fake")

	if err != nil || asked != "fake-tmux" || got.Manifest.ModelFamily != "local" {
		t.Fatalf("Resolve(fake) = %+v err=%v asked=%q, want the fake manifest", got, err, asked)
	}
}

func TestResolve_AgreesWithTheRoutingTableForEveryFamily(t *testing.T) {
	t.Parallel()
	set := DefaultDriverSet()
	for _, family := range set.Families() {
		got, err := Resolve(set, family)
		if err != nil {
			t.Fatalf("Resolve(%q): %v", family, err)
		}
		if route := llmroute.DefaultDriverForFamily(family); route != got.Driver {
			t.Errorf("family %q: llmroute says %q, Resolve says %q: the two tables differ", family, route, got.Driver)
		}
	}
	if len(set.Families()) < 5 {
		t.Errorf("families=%v, want at least the five interactive drivers", set.Families())
	}
}

func mapValues(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func TestWithLoader_KeepsDriversAndSwapsTheLoader(t *testing.T) {
	t.Parallel()
	want := Manifest{ModelFamily: "swapped"}
	set := DefaultDriverSet().WithLoader(func(string) (Manifest, error) { return want, nil })

	got, err := Resolve(set, "claude")

	if err != nil || got.Manifest.ModelFamily != "swapped" || got.Driver != "claude-tmux" {
		t.Errorf("got %+v, %v; want the claude driver with the injected manifest", got, err)
	}
}

func TestDefaultDriverSet_IsOneSharedSet(t *testing.T) {
	t.Parallel()
	a, b := DefaultDriverSet(), DefaultDriverSet()
	if len(a.Families()) == 0 || len(a.Families()) != len(b.Families()) {
		t.Errorf("default set families differ: %v vs %v", a.Families(), b.Families())
	}
}
