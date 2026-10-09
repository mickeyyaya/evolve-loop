package derived

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func repoRootForTest(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func TestCatalog_SkillProjectionsAreDerivedOutputs(t *testing.T) {
	cases := []struct {
		path, entry, region string
	}{
		{"commands/build.md", "skill-projections", ""},
		{".codex-plugin/plugin.json", "skill-projections", ""},
		{".agents/plugins/marketplace.json", "skill-projections", ""},
		{"skills/build/SKILL.md", "skill-projections", "GENERATED:phase-facts"},
		{"docs/architecture/control-flags.md", "flag-index", "GENERATED:flag-index"},
		{"docs/architecture/signal-codes.md", "signal-codes", "GENERATED:signal-codes"},
	}
	for _, c := range cases {
		e, o, ok := OutputOf(c.path)
		if !ok || e.Name != c.entry || o.Region != c.region {
			t.Errorf("OutputOf(%q) = (%q, region %q, %v), want (%q, region %q, true)", c.path, e.Name, o.Region, ok, c.entry, c.region)
		}
	}
	for _, p := range []string{"commands/sub/build.md", "skills/build/references/a.md", "skills/SKILL.md", "README.md", "go/internal/skillcheck/commands.go", ""} {
		if e, _, ok := OutputOf(p); ok {
			t.Errorf("OutputOf(%q) = %q, want no entry", p, e.Name)
		}
	}
}

func TestCatalog_EveryOutputCarriesItsGeneratedMarker(t *testing.T) {
	root := repoRootForTest(t)
	if got := names(Catalog()); !reflect.DeepEqual(got, []string{"flag-index", "signal-codes", "skill-projections"}) {
		t.Fatalf("Catalog entries = %v, want [flag-index signal-codes skill-projections]", got)
	}
	for _, e := range Catalog() {
		for _, o := range e.Outputs {
			matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(o.Pattern)))
			if err != nil || len(matches) == 0 {
				t.Fatalf("%s output %q matches no file in the repository (err %v)", e.Name, o.Pattern, err)
			}
			if len(markersOf(o)) == 0 && !strings.HasSuffix(o.Pattern, ".json") {
				t.Errorf("%s output %q declares no marker; only a JSON output can carry none", e.Name, o.Pattern)
			}
			carriers := 0
			for _, m := range matches {
				if carriesAll(t, m, markersOf(o)) {
					carriers++
				} else if o.Region == "" || !strings.Contains(o.Pattern, "*") {
					t.Errorf("%s: %s does not carry the markers %q", e.Name, m, markersOf(o))
				}
			}
			if carriers == 0 {
				t.Errorf("%s output %q: no file carries the markers %q", e.Name, o.Pattern, markersOf(o))
			}
		}
	}
}

func carriesAll(t *testing.T, path string, markers []string) bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, m := range markers {
		if !strings.Contains(string(raw), m) {
			return false
		}
	}
	return true
}

func TestCatalog_TheRouterRecipeRegionIsNotAnEntry(t *testing.T) {
	const router = "agents/evolve-router.md"
	if e, _, ok := OutputOf(router); ok {
		t.Fatalf("OutputOf(%q) = %q; the goal-recipes region has no generator, so it is not an entry", router, e.Name)
	}
	for _, e := range Catalog() {
		for _, o := range e.Outputs {
			if o.Region == "GENERATED:goal-recipes" {
				t.Fatalf("entry %s holds the goal-recipes region", e.Name)
			}
		}
	}
	merged := "# Router\n<!-- GENERATED:goal-recipes BEGIN -->\n<<<<<<< HEAD\na\n=======\nb\n>>>>>>> lane\n<!-- GENERATED:goal-recipes END -->\n"
	if IsDerivedConflict(router, []byte(merged)) {
		t.Fatal("a conflict inside the router recipe region is genuine, got derived")
	}
}

const flagsDoc = "docs/architecture/control-flags.md"

func flagDocWith(before, inside, after string) []byte {
	return []byte("# Flags\n" + before +
		"<!-- GENERATED:flag-index BEGIN — do not edit by hand; run `evolve flags generate` -->\n" + inside +
		"<!-- GENERATED:flag-index END -->\n" + after)
}

const block = "<<<<<<< HEAD\nours\n=======\ntheirs\n>>>>>>> lane\n"

func TestRegionConflict_ABlockInsideTheRegionIsDerived(t *testing.T) {
	cases := map[string][]byte{
		"one block":                       flagDocWith("intro\n", "row\n"+block+"row\n", "tail\n"),
		"two blocks":                      flagDocWith("", block+"row\n"+block, ""),
		"an end marker before the region": flagDocWith("<!-- GENERATED:flag-index END -->\n", block, ""),
		"a diff3 block":                   flagDocWith("", "<<<<<<< HEAD\nours\n||||||| base\nold\n=======\ntheirs\n>>>>>>> lane\n", ""),
		"a whole command":                 []byte(commandStubMarker + "build/SKILL.md -->\n" + block),
	}
	for name, merged := range cases {
		path := flagsDoc
		if name == "a whole command" {
			path = "commands/build.md"
		}
		if !IsDerivedConflict(path, merged) {
			t.Errorf("%s: IsDerivedConflict(%q) = false, want true", name, path)
		}
	}
}

func TestRegionConflict_ABlockOutsideTheRegionIsGenuine(t *testing.T) {
	straddle := []byte("# Flags\n<!-- GENERATED:flag-index BEGIN -->\nrow\n<<<<<<< HEAD\n<!-- GENERATED:flag-index END -->\n=======\nx\n>>>>>>> lane\n")
	cases := map[string][]byte{
		"before the region":     flagDocWith(block, "row\n", ""),
		"after the region":      flagDocWith("", "row\n", block),
		"one in and one out":    flagDocWith("", block, block),
		"across the end marker": straddle,
		"no region at all":      []byte("# Flags\n" + block),
		"no conflict block":     flagDocWith("", "row\n", ""),
		"an unclosed block":     flagDocWith("", block+"<<<<<<< HEAD\nours\n", ""),
	}
	for name, merged := range cases {
		if IsDerivedConflict(flagsDoc, merged) {
			t.Errorf("%s: IsDerivedConflict = true, want genuine", name)
		}
	}
	if IsDerivedConflict("go/internal/core/x.go", []byte(block)) {
		t.Error("a path outside the catalog is genuine, got derived")
	}
}

func names(es []Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Name)
	}
	return out
}

func TestFires_CrossStalenessNeedsBothSides(t *testing.T) {
	registry := "go/internal/flagregistry/registry_table.go"
	unrelated := "go/internal/router/router.go"
	cases := []struct {
		name                   string
		lane, peer, conflicted []string
		want                   []string
	}{
		{"input on one side only", []string{registry}, []string{unrelated}, nil, nil},
		{"input against output", []string{registry}, []string{flagsDoc}, nil, []string{"flag-index"}},
		{"output against input", []string{flagsDoc}, []string{registry}, nil, []string{"flag-index"}},
		{"input on both sides", []string{registry}, []string{"go/internal/flagregistry/other.go"}, nil, []string{"flag-index"}},
		{"outputs on both sides", []string{flagsDoc}, []string{flagsDoc}, nil, nil},
		{"a conflict on an output", []string{unrelated}, nil, []string{"commands/build.md"}, []string{"skill-projections"}},
		{"a go input set on one side", []string{"go/internal/core/signal.go"}, []string{"docs/architecture/signal-codes.md"}, nil, []string{"signal-codes"}},
	}
	dirs := GoInputDirs{CodeRegistrars: {"go/internal/core"}}
	for _, c := range cases {
		if got := names(Fires(c.lane, c.peer, c.conflicted, dirs)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Fires = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestCatalog_EveryInputExists(t *testing.T) {
	root := repoRootForTest(t)
	for _, e := range Catalog() {
		if len(e.Generate) == 0 || len(e.Check) == 0 {
			t.Errorf("%s has an empty generate or check command", e.Name)
		}
		for _, in := range e.Inputs {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(in))); err != nil {
				t.Errorf("%s input %q does not exist: %v", e.Name, in, err)
			}
		}
	}
}

func TestLookup_AnUnknownNameIsNoEntry(t *testing.T) {
	if e, ok := Lookup("goal-recipes"); ok {
		t.Fatalf("Lookup(goal-recipes) = %q, want no entry", e.Name)
	}
}

func TestPathspecs_ListEveryOutputPattern(t *testing.T) {
	got := mustLookup(t, "skill-projections").Pathspecs()
	want := []string{"commands/*.md", ".codex-plugin/plugin.json", ".agents/plugins/marketplace.json", "skills/*/SKILL.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Pathspecs = %v, want %v", got, want)
	}
}

func TestStale_AnInputChangeMakesItsEntryStale(t *testing.T) {
	dirs := GoInputDirs{SkillcheckClosure: {"go/internal/phasespec"}, CodeRegistrars: {"go/internal/core"}}
	cases := []struct {
		name    string
		changed []string
		want    []string
	}{
		{"a registry file", []string{"go/internal/flagregistry/registry_table.go"}, []string{"flag-index"}},
		{"an exact input file", []string{".claude-plugin/plugin.json"}, []string{"skill-projections"}},
		{"a package in the skillcheck closure", []string{"go/internal/phasespec/catalog.go"}, []string{"skill-projections"}},
		{"a code registrar", []string{"go/internal/core/signal.go"}, []string{"signal-codes"}},
		{"a file that only shares a prefix", []string{".claude-plugin/plugin.json.bak", "go/internal/phasespecx/a.go"}, nil},
		{"an output alone", []string{flagsDoc}, nil},
		{"nothing", nil, nil},
	}
	for _, c := range cases {
		if got := names(Stale(c.changed, dirs)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Stale = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestMarkerConflict_BothSidesMustCarryTheMarker(t *testing.T) {
	marker := commandStubMarker + "build/SKILL.md -->\n"
	cases := []struct {
		name   string
		merged string
		want   bool
	}{
		{"a generated stub on both sides", marker + block, true},
		{"a setext heading above the marker", "Title\n=======\n" + marker + block, true},
		{"the marker inside both sides of the block", "<<<<<<< HEAD\n" + marker + "ours\n=======\n" + marker + "theirs\n>>>>>>> lane\n", true},
		{"a hand-written command", "# foo\n" + block, false},
		{"the marker on our side only", "<<<<<<< HEAD\n" + marker + "ours\n=======\ntheirs\n>>>>>>> lane\n", false},
		{"the marker on their side only", "<<<<<<< HEAD\nours\n||||||| base\n" + marker + "=======\n" + marker + "theirs\n>>>>>>> lane\n", false},
		{"a deleted file", "", false},
	}
	for _, c := range cases {
		if got := IsDerivedConflict("commands/foo.md", []byte(c.merged)); got != c.want {
			t.Errorf("%s: IsDerivedConflict = %v, want %v", c.name, got, c.want)
		}
	}
	if !IsDerivedConflict(".codex-plugin/plugin.json", []byte(block)) {
		t.Error("a JSON output carries no marker; its conflict stays derived and the leftover check guards it")
	}
}

func markersOf(o Output) []string {
	if o.Region != "" {
		return []string{o.begin(), o.end()}
	}
	if o.Marker != "" {
		return []string{o.Marker}
	}
	return nil
}
