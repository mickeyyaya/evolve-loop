package inboxcurator

import (
	"errors"
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

func probeFS() fstest.MapFS {
	return fstest.MapFS{
		"go/internal/a/a.go":          {Data: []byte("package a\n\nimport \"strings\"\n\nfunc Exported() {}\n\nfunc (r R) Method() {}\n\ntype R struct{}\n\nconst (\n\tOne = 1\n\tTwo = 2\n)\n\nvar Three = 3\n")},
		"go/internal/a/a_test.go":     {Data: []byte("package a\n\nimport \"testing\"\n\nfunc TestAlpha(t *testing.T) {}\n\nfunc OnlyInATest() {}\n")},
		"go/internal/a/testdata/x.go": {Data: []byte("package a\n\nfunc Fixture() {}\n")},
		"go/internal/a/notes.txt":     {Data: []byte("package a\n\nfunc NotGo() {}\n")},
		"go/internal/b/b.go":          {Data: []byte("package b\n\nfunc Other() {}\n")},
		".evolve/policy.json":         {Data: []byte(`{"inbox":{"curator":{"stage":"enforce","consume_min_share":0.5}},"flag":true}`)},
		"docs/x.md":                   {Data: []byte("the old phrase lives here")},
	}
}

type brokenReadFS struct {
	fstest.MapFS
	broken string
}

func (b brokenReadFS) Open(name string) (fs.File, error) {
	if name == b.broken {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return b.MapFS.Open(name)
}

func (b brokenReadFS) ReadFile(name string) ([]byte, error) {
	if name == b.broken {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return b.MapFS.ReadFile(name)
}

func probeTree() RepoTree { return RepoTree{FS: probeFS()} }

func probe(kind inboxbatch.ProbeKind, target, value string) inboxbatch.Probe {
	return inboxbatch.Probe{Kind: kind, Target: target, Value: value}
}

func checkAll(t *testing.T, tree RepoTree, cases map[inboxbatch.Probe]bool) {
	t.Helper()
	for p, want := range cases {
		if got := tree.Check(p); got.Pass != want || got.Probe != p {
			t.Errorf("Check(%+v) = %+v, want pass %v", p, got, want)
		}
	}
}

func TestRepoTree_TestExistsPassesForATestOfTheTree(t *testing.T) {
	checkAll(t, probeTree(), map[inboxbatch.Probe]bool{
		probe(inboxbatch.ProbeTestExists, "TestAlpha", ""):   true,
		probe(inboxbatch.ProbeTestExists, "TestMissing", ""): false,
	})
	injected := RepoTree{FS: probeFS(), HasTest: func(name string) bool { return name == "TestInjected" }}
	checkAll(t, injected, map[inboxbatch.Probe]bool{
		probe(inboxbatch.ProbeTestExists, "TestInjected", ""): true,
		probe(inboxbatch.ProbeTestExists, "TestAlpha", ""):    false,
	})
}

func TestRepoTree_TestExistsFailsWhenTheTestsCannotBeRead(t *testing.T) {
	got := RepoTree{FS: brokenReadFS{probeFS(), "go/internal/a/a_test.go"}}.Check(probe(inboxbatch.ProbeTestExists, "TestAlpha", ""))
	if got.Pass || !strings.Contains(got.Detail, "a_test.go") {
		t.Errorf("Check = %+v, want a fail that names the unreadable file", got)
	}
}

func TestRepoTree_SymbolExistsFindsATopLevelNameOfThePackage(t *testing.T) {
	checkAll(t, probeTree(), map[inboxbatch.Probe]bool{
		probe(inboxbatch.ProbeSymbolExists, "a.Exported", ""):    true,
		probe(inboxbatch.ProbeSymbolExists, "a.R", ""):           true,
		probe(inboxbatch.ProbeSymbolExists, "a.Two", ""):         true,
		probe(inboxbatch.ProbeSymbolExists, "a.Three", ""):       true,
		probe(inboxbatch.ProbeSymbolExists, "a.Method", ""):      false,
		probe(inboxbatch.ProbeSymbolExists, "a.OnlyInATest", ""): false,
		probe(inboxbatch.ProbeSymbolExists, "a.Fixture", ""):     false,
		probe(inboxbatch.ProbeSymbolExists, "a.NotGo", ""):       false,
		probe(inboxbatch.ProbeSymbolExists, "a.Other", ""):       false,
		probe(inboxbatch.ProbeSymbolExists, "b.Exported", ""):    false,
	})
}

func TestRepoTree_SymbolExistsRefusesAPackageNameOfTwoDirectoriesUnlessTheDirectoryIsNamed(t *testing.T) {
	fsys := probeFS()
	fsys["go/cmd/a/main.go"] = &fstest.MapFile{Data: []byte("package a\n\nfunc Elsewhere() {}\n")}
	tree := RepoTree{FS: fsys}
	if got := tree.Check(probe(inboxbatch.ProbeSymbolExists, "a.Elsewhere", "")); got.Pass || !strings.Contains(got.Detail, "2 directories") {
		t.Errorf("an ambiguous package = %+v, want a fail that names the two directories", got)
	}
	checkAll(t, tree, map[inboxbatch.Probe]bool{
		probe(inboxbatch.ProbeSymbolExists, "cmd/a.Elsewhere", ""):             true,
		probe(inboxbatch.ProbeSymbolExists, "internal/a.Elsewhere", ""):        false,
		probe(inboxbatch.ProbeSymbolExists, "internal/a.Exported", ""):         true,
		probe(inboxbatch.ProbeSymbolExists, "internal/a/testdata.Fixture", ""): false,
		probe(inboxbatch.ProbeSymbolExists, "internal/gone.Exported", ""):      false,
	})
}

func TestRepoTree_SymbolExistsFailsWithTheReasonOnATreeItCannotRead(t *testing.T) {
	noGo := RepoTree{FS: fstest.MapFS{"docs/x.md": {Data: []byte("x")}}}
	if got := noGo.Check(probe(inboxbatch.ProbeSymbolExists, "a.Exported", "")); got.Pass || got.Detail == "" {
		t.Errorf("a tree with no go/ = %+v, want a fail with a detail", got)
	}
	broken := probeFS()
	broken["go/internal/a/broken.go"] = &fstest.MapFile{Data: []byte("package a\n\nfunc (")}
	if got := (RepoTree{FS: broken}).Check(probe(inboxbatch.ProbeSymbolExists, "a.Exported", "")); got.Pass || !strings.Contains(got.Detail, "broken.go") {
		t.Errorf("a file that does not parse = %+v, want a fail that names it", got)
	}
	if got := (RepoTree{FS: brokenReadFS{probeFS(), "go/internal/b/b.go"}}).Check(probe(inboxbatch.ProbeSymbolExists, "b.Other", "")); got.Pass || !strings.Contains(got.Detail, "b.go") {
		t.Errorf("an unreadable file = %+v, want a fail that names it", got)
	}
}

func TestRepoTree_TextAbsentPassesWhenTheFileLacksTheText(t *testing.T) {
	checkAll(t, probeTree(), map[inboxbatch.Probe]bool{
		probe(inboxbatch.ProbeTextAbsent, "docs/x.md", "new phrase"): true,
		probe(inboxbatch.ProbeTextAbsent, "docs/x.md", "old phrase"): false,
		probe(inboxbatch.ProbeTextAbsent, "docs/gone.md", "phrase"):  false,
	})
	if got := (RepoTree{FS: brokenReadFS{probeFS(), "docs/x.md"}}).Check(probe(inboxbatch.ProbeTextAbsent, "docs/x.md", "new")); got.Pass || !strings.Contains(got.Detail, "permission") {
		t.Errorf("an unreadable file = %+v, want a fail with the read error", got)
	}
}

func TestRepoTree_PolicyValueMatchesTheValueAtADottedKey(t *testing.T) {
	checkAll(t, probeTree(), map[inboxbatch.Probe]bool{
		probe(inboxbatch.ProbePolicyValue, "inbox.curator.stage", "enforce"):                               true,
		probe(inboxbatch.ProbePolicyValue, "inbox.curator.stage", "shadow"):                                false,
		probe(inboxbatch.ProbePolicyValue, "inbox.curator.consume_min_share", "0.5"):                       true,
		probe(inboxbatch.ProbePolicyValue, "flag", "true"):                                                 true,
		probe(inboxbatch.ProbePolicyValue, "inbox.missing", "x"):                                           false,
		probe(inboxbatch.ProbePolicyValue, "inbox.curator.stage.deeper", "enforce"):                        false,
		probe(inboxbatch.ProbePolicyValue, "inbox.curator", `{"stage":"enforce"}`):                         false,
		probe(inboxbatch.ProbePolicyValue, "inbox", `{"curator":{"stage":"enforce"}}`):                     false,
		probe(inboxbatch.ProbePolicyValue, "inbox.curator", `{"stage":"enforce","consume_min_share":0.5}`): true,
	})
	noPolicy := RepoTree{FS: fstest.MapFS{"docs/x.md": {Data: []byte("x")}}}
	if got := noPolicy.Check(probe(inboxbatch.ProbePolicyValue, "flag", "true")); got.Pass || got.Detail == "" {
		t.Errorf("no policy file = %+v, want a fail with a detail", got)
	}
	torn := RepoTree{FS: fstest.MapFS{".evolve/policy.json": {Data: []byte(`{"flag":`)}}}
	if got := torn.Check(probe(inboxbatch.ProbePolicyValue, "flag", "true")); got.Pass || got.Detail == "" {
		t.Errorf("a torn policy file = %+v, want a fail with a detail", got)
	}
}

func TestRepoTree_AnUnknownKindNeverPasses(t *testing.T) {
	if got := probeTree().Check(probe("file_exists", "docs/x.md", "")); got.Pass || !strings.Contains(got.Detail, "file_exists") {
		t.Errorf("Check = %+v, want a fail that names the kind", got)
	}
}

func TestRepoTree_ExistsSeesAFileOrADirectoryAndJudgesNoOddPath(t *testing.T) {
	tree := probeTree()
	for p, want := range map[string]bool{
		"go/internal/a/a.go": true, "go/internal/a/": true,
		"go/gone.go": false, "../outside": true, "": true,
	} {
		if got := tree.Exists(p); got != want {
			t.Errorf("Exists(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestCheckProbes_RefusesAMalformedProbe(t *testing.T) {
	good := []inboxbatch.Probe{
		probe(inboxbatch.ProbeTestExists, "TestAlpha", ""),
		probe(inboxbatch.ProbeSymbolExists, "inboxcurator.Admit", ""),
		probe(inboxbatch.ProbeSymbolExists, "internal/inboxcurator.Admit", ""),
		probe(inboxbatch.ProbeTextAbsent, "docs/x.md", "old phrase"),
		probe(inboxbatch.ProbePolicyValue, "inbox.curator.stage", "enforce"),
	}
	for _, kind := range inboxbatch.ProbeKinds() {
		if _, known := probeShapes[kind]; !known {
			t.Errorf("kind %q has no shape", kind)
		}
	}
	if err := CheckProbes(good); err != nil {
		t.Fatalf("CheckProbes(good) = %v, want nil", err)
	}
	for name, tc := range map[string]struct {
		p   inboxbatch.Probe
		why string
	}{
		"an unknown kind":           {probe("file_exists", "docs/x.md", ""), `"file_exists"`},
		"a test that is not a test": {probe(inboxbatch.ProbeTestExists, "Alpha", ""), `"Alpha"`},
		"a value on a name probe":   {probe(inboxbatch.ProbeTestExists, "TestAlpha", "x"), "takes no value"},
		"a bare symbol":             {probe(inboxbatch.ProbeSymbolExists, "Admit", ""), `"Admit"`},
		"a path that leaves":        {probe(inboxbatch.ProbeTextAbsent, "../x.md", "old"), `"../x.md"`},
		"an absolute path":          {probe(inboxbatch.ProbeTextAbsent, "/etc/x", "old"), `"/etc/x"`},
		"no text":                   {probe(inboxbatch.ProbeTextAbsent, "docs/x.md", ""), "needs a value"},
		"a bad key":                 {probe(inboxbatch.ProbePolicyValue, "inbox..stage", "enforce"), `"inbox..stage"`},
		"no policy value":           {probe(inboxbatch.ProbePolicyValue, "inbox.curator.stage", ""), "needs a value"},
	} {
		err := CheckProbes(append(slices.Clone(good), tc.p))
		if !errors.Is(err, ErrIntake) || !strings.Contains(err.Error(), tc.why) || !strings.Contains(err.Error(), "probe 6") {
			t.Errorf("%s: err = %v, want ErrIntake naming probe 6 and %q", name, err, tc.why)
		}
	}
}

func TestCheckIntake_RefusesAMalformedProbe(t *testing.T) {
	it := goodItem()
	it.Probes = []inboxbatch.Probe{probe("file_exists", "docs/x.md", "")}
	if err := CheckIntake(it, []string{"feature"}); !errors.Is(err, ErrIntake) || !strings.Contains(err.Error(), "file_exists") {
		t.Errorf("CheckIntake = %v, want ErrIntake naming the kind", err)
	}
}

func TestProbesPass_EveryProbeMustPassAndNoProbesIsNotDone(t *testing.T) {
	tree := probeTree()
	pass := probe(inboxbatch.ProbeTestExists, "TestAlpha", "")
	fail := probe(inboxbatch.ProbeTestExists, "TestMissing", "")
	if results, all := ProbesPass([]inboxbatch.Probe{pass, pass}, tree); !all || len(results) != 2 {
		t.Errorf("two passing probes = %+v, %v; want all pass", results, all)
	}
	if results, all := ProbesPass([]inboxbatch.Probe{pass, fail}, tree); all || len(results) != 2 || results[1].Pass {
		t.Errorf("one failing probe = %+v, %v; want not all", results, all)
	}
	if _, all := ProbesPass(nil, tree); all {
		t.Error("an item with no probes is not done")
	}
}

func TestProbeResult_StringNamesTheKindTheTargetAndTheVerdict(t *testing.T) {
	for r, want := range map[ProbeResult]string{
		{Probe: probe(inboxbatch.ProbeTestExists, "TestAlpha", ""), Pass: true}:                                      "test_exists TestAlpha: pass",
		{Probe: probe(inboxbatch.ProbeTextAbsent, "docs/x.md", "old"), Detail: "the text is present"}:                `text_absent docs/x.md "old": fail (the text is present)`,
		{Probe: probe(inboxbatch.ProbePolicyValue, "inbox.curator.stage", "enforce"), Detail: "the value is shadow"}: `policy_value inbox.curator.stage "enforce": fail (the value is shadow)`,
	} {
		if got := r.String(); got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

func probeItem(probes ...inboxbatch.Probe) inboxbatch.Item {
	it := goodItem()
	it.Files = []string{"go/internal/a/a.go", "go/internal/a/gone.go"}
	it.Probes = probes
	return it
}

func TestAdmit_RefusesAnItemWhoseProbesAlreadyPass(t *testing.T) {
	rules := Rules{Kinds: []string{"feature"}, Tree: probeTree()}
	done := probeItem(probe(inboxbatch.ProbeTestExists, "TestAlpha", ""), probe(inboxbatch.ProbeTextAbsent, "docs/x.md", "new phrase"))

	_, err := Admit(done, nil, rules, Override{Reason: "an override does not file done work"})

	if !errors.Is(err, ErrIntake) || !errors.Is(err, ErrAlreadyDone) || !strings.Contains(err.Error(), "test_exists TestAlpha: pass") {
		t.Fatalf("Admit = %v, want ErrAlreadyDone naming each probe", err)
	}
	open := probeItem(probe(inboxbatch.ProbeTestExists, "TestAlpha", ""), probe(inboxbatch.ProbeTestExists, "TestMissing", ""))
	if adm, err := Admit(open, nil, rules, Override{}); err != nil || adm.Record != nil {
		t.Errorf("Admit of an item with a failing probe = %+v, %v; want admitted", adm, err)
	}
}

func TestAdmit_RefusesAProbeItemWhoseDeclaredPathsAreAllGone(t *testing.T) {
	rules := Rules{Kinds: []string{"feature"}, Tree: probeTree()}
	it := probeItem(probe(inboxbatch.ProbeTestExists, "TestMissing", ""))
	it.Files = []string{"go/internal/new/new.go"}

	_, err := Admit(it, nil, rules, Override{})

	if !errors.Is(err, ErrIntake) || !strings.Contains(err.Error(), "go/internal/new/new.go") {
		t.Fatalf("Admit = %v, want ErrIntake naming the gone path", err)
	}
	it.Probes = nil
	if _, err := Admit(it, nil, rules, Override{}); err != nil {
		t.Errorf("Admit of a probe-less item on new paths = %v, want admitted", err)
	}
}

func TestSweep_EveryDeclaredPathGoneIsAQuarantine(t *testing.T) {
	dead := probeItem(probe(inboxbatch.ProbeTestExists, "TestAlpha", ""))
	dead.ID, dead.Path, dead.Files = "dead", "dead.json", []string{"go/gone/a.go", "go/gone/"}
	live := probeItem(probe(inboxbatch.ProbeTestExists, "TestMissing", ""))
	probeless := goodItem()
	probeless.Files = []string{"go/gone/b.go"}

	got := SweepProbes([]inboxbatch.Item{dead, live, probeless}, probeTree())

	if len(got) != 1 {
		t.Fatalf("SweepProbes = %+v, want one quarantine", got)
	}
	q := got[0]
	if q.Kind != ProposeQuarantine || q.ID != "dead" || q.Path != "dead.json" || q.Trigger != TriggerSweep || q.Deterministic {
		t.Errorf("proposal = %+v, want a judgment quarantine of dead", q)
	}
	if !slices.Contains(q.Evidence, "paths gone: go/gone/a.go, go/gone/") {
		t.Errorf("evidence = %v, want the gone paths", q.Evidence)
	}
}

func TestSweepProbes_EveryProbePassingIsADeterministicConsume(t *testing.T) {
	done := probeItem(probe(inboxbatch.ProbeTestExists, "TestAlpha", ""), probe(inboxbatch.ProbeSymbolExists, "a.Exported", ""))
	done.Path = "done.json"

	got := SweepProbes([]inboxbatch.Item{done}, probeTree())

	if len(got) != 1 || got[0].Kind != ProposeConsume || !got[0].Deterministic || got[0].Trigger != TriggerSweep || got[0].Path != "done.json" {
		t.Fatalf("SweepProbes = %+v, want one deterministic consume", got)
	}
	want := []string{"probe test_exists TestAlpha: pass", "probe symbol_exists a.Exported: pass"}
	if !slices.Equal(got[0].Evidence, want) {
		t.Errorf("evidence = %v, want %v", got[0].Evidence, want)
	}
}

func TestCurate_TheBackstopRunsTheProbeSweep(t *testing.T) {
	done := probeItem(probe(inboxbatch.ProbeTestExists, "TestAlpha", ""))
	done.Acceptance = []string{"TestAlpha passes"}
	in := Inputs{Open: []inboxbatch.Item{done}, HasTest: func(string) bool { return true }, MinShare: 0.5, Tree: probeTree()}

	if got := Curate(in); len(got) != 0 {
		t.Errorf("Curate with no change and no backstop = %+v, want none", got)
	}
	in.Backstop = true
	got := Curate(in)
	if len(got) != 1 || got[0].Kind != ProposeConsume || !got[0].Deterministic {
		t.Errorf("Curate at the backstop = %+v, want the deterministic probe consume over the judgment sweep", got)
	}
}
