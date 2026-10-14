package testkey

import (
	"slices"
	"strings"
	"testing"
)

func TestKey_ChangesWhenAFileOfADependencyChanges(t *testing.T) {
	before := mustCompute(t, baseInput())

	after := mustCompute(t, withIDs(baseInput(), map[string]string{srcFile("b"): "id-b-edited"}))

	assertChanged(t, before, after)
	if before.File == after.File {
		t.Fatalf("file key stayed %s for a changed dependency file", before.File)
	}
}

func TestKey_StaysWhenAnUnrelatedPackageChanges(t *testing.T) {
	before := mustCompute(t, baseInput())

	after := mustCompute(t, withIDs(baseInput(), map[string]string{srcFile("c"): "id-c-edited"}))

	assertSame(t, before, after)
}

func TestKey_ChangesWhenADataRootChanges(t *testing.T) {
	root := ".evolve/profiles"
	in := withRoot(baseInput(), "a", root, "root-1")
	in.Log = ParseLog("# test log\nopen /repo/.evolve/profiles/x.json\n")
	before := mustCompute(t, in)

	changed := in
	changed.RootIDs = map[string]string{root: "root-2"}
	after := mustCompute(t, changed)

	assertChanged(t, before, after)
	if before.Whole {
		t.Fatal("a catalogued data root keyed on the whole tree, want a root key")
	}
}

func TestKey_ChangesWhenATagOrFlagChanges(t *testing.T) {
	before := mustCompute(t, baseInput())
	tagged := baseInput()
	tagged.Toolchain.Tags = []string{"e2e"}
	flagged := baseInput()
	flagged.Toolchain.Flags = []string{"-count=1", "-race"}
	versioned := baseInput()
	versioned.Toolchain.GoVersion = "go1.24.0"

	for name, in := range map[string]Input{"tag": tagged, "flag": flagged, "version": versioned} {
		if mustCompute(t, in).Full == before.Full {
			t.Errorf("a changed %s left the key at %s", name, before.Full)
		}
	}
}

func TestKey_ATreeReadingTestKeysOnTheWholeTree(t *testing.T) {
	in := baseInput()
	in.TreeReading = true
	before := mustCompute(t, in)

	unrelated := withIDs(in, map[string]string{srcFile("c"): "id-c-edited"})
	unrelated.WholeTreeID = "tree-2"
	after := mustCompute(t, unrelated)

	if !before.Whole {
		t.Fatal("a tree-reading test did not key on the whole tree")
	}
	assertChanged(t, before, after)
	same := mustCompute(t, in)
	assertSame(t, before, same)
}

func TestKey_AnUnknownReadKeysOnTheWholeTree(t *testing.T) {
	for name, log := range map[string]Log{"empty log": ParseLog(""), "odd line": ParseLog("# test log\nteleport /x\n")} {
		in := baseInput()
		in.Log = log
		before := mustCompute(t, in)
		moved := in
		moved.WholeTreeID = "tree-2"

		after := mustCompute(t, moved)

		if !before.Whole {
			t.Errorf("%s: an unknown read did not key on the whole tree", name)
		}
		assertChanged(t, before, after)
	}
	unresolved := baseInput()
	unresolved.UnresolvedRoots = []string{importPath("b")}
	if !mustCompute(t, unresolved).Whole {
		t.Error("an unresolved data root of a dependency did not key on the whole tree")
	}
}

func TestKey_ATestdataChangeChangesTheKey(t *testing.T) {
	before := mustCompute(t, baseInput())
	in := baseInput()
	in.TestdataIDs = map[string]string{"go/internal/a": "td-a-edited", "go/internal/b": "td-go/internal/b", "go/internal/c": "td-go/internal/c", "go/internal/h": "td-go/internal/h"}

	assertChanged(t, before, mustCompute(t, in))
}

func TestKey_ChangesWhenAProductionDependencyReadsAnOutsideRoot(t *testing.T) {
	root := ".evolve/config"
	in := withRoot(baseInput(), "b", root, "root-1")
	before := mustCompute(t, in)
	changed := in
	changed.RootIDs = map[string]string{root: "root-2"}

	after := mustCompute(t, changed)

	assertChanged(t, before, after)
	other := withRoot(baseInput(), "c", root, "root-1")
	otherChanged := other
	otherChanged.RootIDs = map[string]string{root: "root-2"}
	assertSame(t, mustCompute(t, other), mustCompute(t, otherChanged))
}

func TestKey_ChangesWhenAReadEnvVarChanges(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\ngetenv EVOLVE_MODE\nopen /repo/go/internal/a/testdata/x.json\n")
	set := withEnv(in, map[string]string{"EVOLVE_MODE": "fast"})
	before := mustCompute(t, set)

	after := mustCompute(t, withEnv(in, map[string]string{"EVOLVE_MODE": "slow"}))
	unset := mustCompute(t, in)
	empty := mustCompute(t, withEnv(in, map[string]string{"EVOLVE_MODE": ""}))

	assertChanged(t, before, after)
	if before.File != after.File {
		t.Fatalf("file key moved from %s to %s on an env change, want the two-step key to hold", before.File, after.File)
	}
	if unset.Full == empty.Full {
		t.Fatal("an unset variable and an empty variable gave one key")
	}
	if len(before.Env) != 1 || before.Env[0] != (EnvValue{Name: "EVOLVE_MODE", Value: "fast", Set: true}) {
		t.Fatalf("recorded env = %+v, want EVOLVE_MODE=fast", before.Env)
	}
}

func TestKey_AnOutsideReadInTheTestLogKeysOnTheWholeTree(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\nopen /etc/hosts\n")
	before := mustCompute(t, in)
	moved := in
	moved.WholeTreeID = "tree-2"

	after := mustCompute(t, moved)

	if !before.Whole {
		t.Fatal("a read outside the repository did not key on the whole tree")
	}
	assertChanged(t, before, after)
}

func TestKey_AReadInTheTempDirIsNotAnOutsideRead(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\nopen /tmp/run-1/work/out.json\nstat /tmp/run-1\n")

	k := mustCompute(t, in)

	if k.Whole {
		t.Fatal("a read under the run's temp dir keyed on the whole tree")
	}
	other := in
	other.Log = ParseLog("# test log\nopen /tmp/run-2/work/out.json\n")
	if !mustCompute(t, other).Whole {
		t.Fatal("a read under another temp dir did not key on the whole tree")
	}
}

func TestKey_AChangeOfADeclaredChildVarChangesTheKey(t *testing.T) {
	in := baseInput()
	in.ChildVars = []string{"EVOLVE_CHILD"}
	before := mustCompute(t, withEnv(in, map[string]string{"EVOLVE_CHILD": "1"}))

	after := mustCompute(t, withEnv(in, map[string]string{"EVOLVE_CHILD": "2"}))
	undeclared := mustCompute(t, withEnv(baseInput(), map[string]string{"EVOLVE_CHILD": "2"}))
	plain := mustCompute(t, baseInput())

	assertChanged(t, before, after)
	assertSame(t, plain, undeclared)
}

func TestKey_TestImportsJoinTheClosure(t *testing.T) {
	in := baseInput()
	in.TestImports = []string{importPath("h")}
	before := mustCompute(t, in)

	viaHelper := mustCompute(t, withIDs(in, map[string]string{srcFile("h"): "id-h-edited"}))
	viaHelperDep := mustCompute(t, withIDs(in, map[string]string{srcFile("c"): "id-c-edited"}))
	withoutImport := mustCompute(t, withIDs(baseInput(), map[string]string{srcFile("h"): "id-h-edited"}))

	assertChanged(t, before, viaHelper)
	assertChanged(t, before, viaHelperDep)
	assertSame(t, mustCompute(t, baseInput()), withoutImport)
}

func TestKey_ARelativeOpenResolvesAgainstTheWorkDirAndChdir(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\nopen testdata/x.json\nopen ../b/b.go\nchdir /repo/go\nopen go.mod\n")
	if mustCompute(t, in).Whole {
		t.Fatal("covered relative opens keyed on the whole tree")
	}
	in.Log = ParseLog("# test log\nopen ../../../.evolve/state.json\n")
	if !mustCompute(t, in).Whole {
		t.Fatal("a relative open of the repository's .evolve did not key on the whole tree")
	}
	in.Log = ParseLog("# test log\nchdir /elsewhere\n")
	if !mustCompute(t, in).Whole {
		t.Fatal("a chdir outside the repository did not key on the whole tree")
	}
}

func TestKey_AnOpenInTheModuleThatTheKeyDoesNotHoldKeysOnTheWholeTree(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\nopen /repo/go/Makefile\n")
	if !mustCompute(t, in).Whole {
		t.Fatal("an open of a module file outside the key did not key on the whole tree")
	}
	in.Log = ParseLog("# test log\nstat /repo\nstat /repo/go\nstat /repo/go/internal/a\nchdir /repo/go\n")
	if mustCompute(t, in).Whole {
		t.Fatal("a probe of an ancestor directory keyed on the whole tree")
	}
}

func TestKey_RefusesInputsItCannotKey(t *testing.T) {
	missingPkg := baseInput()
	missingPkg.Package = importPath("nope")
	missingFile := baseInput()
	delete(missingFile.IDs, srcFile("b"))
	missingGoMod := baseInput()
	delete(missingGoMod.IDs, "go/go.mod")
	missingRoot := withRoot(baseInput(), "a", ".evolve/p", "r")
	missingRoot.RootIDs = map[string]string{}
	missingTree := baseInput()
	missingTree.TreeReading = true
	missingTree.WholeTreeID = ""
	danglingDep := baseInput()
	danglingDep.Module.Packages = append(danglingDep.Module.Packages, pkg("d", "gone"))
	danglingDep.Package = importPath("d")
	noTestdata := baseInput()
	delete(noTestdata.TestdataIDs, "go/internal/b")
	emptyTestdata := baseInput()
	emptyTestdata.TestdataIDs = map[string]string{"go/internal/a": "", "go/internal/b": "x"}
	noBuildEnv := baseInput()
	noBuildEnv.Toolchain.BuildEnv = nil
	partialBuildEnv := baseInput()
	partialBuildEnv.Toolchain.BuildEnv = fixtureBuildEnv()
	delete(partialBuildEnv.Toolchain.BuildEnv, "CGO_ENABLED")
	missingTestImport := baseInput()
	missingTestImport.TestImports = []string{importPath("nope")}

	cases := map[string]struct {
		in   Input
		want string
	}{
		"unknown package":     {missingPkg, "nope"},
		"missing file id":     {missingFile, srcFile("b")},
		"missing go.mod id":   {missingGoMod, "go/go.mod"},
		"missing root id":     {missingRoot, ".evolve/p"},
		"missing tree id":     {missingTree, "whole tree"},
		"unknown test import": {missingTestImport, "nope"},
		"dangling dependency": {danglingDep, "gone"},
		"no testdata id":      {noTestdata, "go/internal/b"},
		"empty testdata id":   {emptyTestdata, "go/internal/a"},
		"no build env":        {noBuildEnv, "build environment"},
		"partial build env":   {partialBuildEnv, "CGO_ENABLED"},
	}
	for name, tc := range cases {
		_, err := Compute(tc.in)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %v, want it to name %q", name, err, tc.want)
		}
	}
}

func TestParseLog_ReadsEachRecordKind(t *testing.T) {
	log := ParseLog("# test log\ngetenv A\nopen /x\nstat /y\nchdir /z\n\nopen /w\n")

	if log.Unknown {
		t.Fatal("a well-formed log was marked unknown")
	}
	if len(log.Getenv) != 1 || log.Getenv[0] != "A" {
		t.Fatalf("Getenv = %v, want [A]", log.Getenv)
	}
	want := []Op{{"open", "/x"}, {"stat", "/y"}, {"chdir", "/z"}, {"open", "/w"}}
	if len(log.Ops) != len(want) {
		t.Fatalf("Ops = %v, want %v", log.Ops, want)
	}
	for i := range want {
		if log.Ops[i] != want[i] {
			t.Fatalf("Ops[%d] = %v, want %v", i, log.Ops[i], want[i])
		}
	}
	for name, text := range map[string]string{"no header": "open /x\n", "unknown kind": "# test log\nexec /bin/sh\n", "no operand": "# test log\nopen\n"} {
		if !ParseLog(text).Unknown {
			t.Errorf("%s: log was not marked unknown", name)
		}
	}
}

func TestKey_TheFileKeyHoldsTheNamesOfTheVariablesThatTheLogReads(t *testing.T) {
	reads := baseInput()
	reads.Log = ParseLog("# test log\ngetenv EVOLVE_MODE\n")
	silent := baseInput()

	if mustCompute(t, reads).File == mustCompute(t, silent).File {
		t.Fatal("a log that reads a variable gave the file key of a log that reads none")
	}
}

func TestKey_ChangesWhenAnyBuildEnvValueChanges(t *testing.T) {
	before := mustCompute(t, baseInput())

	for _, name := range BuildEnvNames() {
		after := mustCompute(t, withBuildEnv(baseInput(), name, "other"))
		if after.Full == before.Full || after.File == before.File {
			t.Errorf("a changed %s left the key unchanged", name)
		}
	}
	extra := withBuildEnv(baseInput(), "GOMODCACHE", "/elsewhere")
	assertSame(t, before, mustCompute(t, extra))
}

func TestKey_BuildEnvNamesCoverTheVariablesThatReachTheBuild(t *testing.T) {
	want := []string{"GOOS", "GOARCH", "GOFLAGS", "GOEXPERIMENT", "CGO_ENABLED", "CC", "CXX", "CGO_CFLAGS", "CGO_CXXFLAGS", "CGO_LDFLAGS", "GOAMD64", "GOARM", "GO386", "GOARM64", "GOWASM"}
	got := BuildEnvNames()
	for _, name := range want {
		if !slices.Contains(got, name) {
			t.Errorf("BuildEnvNames lacks %s", name)
		}
	}
	got[0] = "MUTATED"
	if BuildEnvNames()[0] == "MUTATED" {
		t.Error("BuildEnvNames hands out its own slice")
	}
}

func TestKey_AnOpenOfADirectoryTheKeyDoesNotFullyHoldKeysOnTheWholeTree(t *testing.T) {
	for _, dir := range []string{"/repo/go/internal/a", "/repo/go/internal", "/repo/go", "/repo"} {
		in := baseInput()
		in.Log = ParseLog("# test log\nopen " + dir + "\n")
		if !mustCompute(t, in).Whole {
			t.Errorf("open of the ancestor %s did not key on the whole tree", dir)
		}
	}
	in := baseInput()
	in.Log = ParseLog("# test log\nopen /repo/go/internal/a/testdata\nopen /repo/go/internal/a/testdata/deep/x.json\n")
	if mustCompute(t, in).Whole {
		t.Fatal("open of a fully keyed testdata tree keyed on the whole tree")
	}
}

func TestKey_AShuffleNeedsAFixedSeed(t *testing.T) {
	refused := [][]string{{"-shuffle=on"}, {"-test.shuffle=on"}, {"-shuffle", "on"}, {"-shuffle"}, {"-shuffle=abc"}, {"--shuffle=on"}, {"-count=1", "-shuffle", "x"}}
	for _, flags := range refused {
		in := baseInput()
		in.Toolchain.Flags = flags
		if _, err := Compute(in); err == nil || !strings.Contains(err.Error(), "shuffle") {
			t.Errorf("flags %v: error = %v, want a refusal that names shuffle", flags, err)
		}
	}
	allowed := [][]string{{"-shuffle=off"}, {"-shuffle=42"}, {"-test.shuffle=7"}, {"-shuffle", "42"}, {"-shuffle", "off"}, {"-count=1", "-run", "TestShuffle"}}
	for _, flags := range allowed {
		in := baseInput()
		in.Toolchain.Flags = flags
		if _, err := Compute(in); err != nil {
			t.Errorf("flags %v: unexpected error %v", flags, err)
		}
	}
}

func TestKey_ASymlinkInACoveredPathKeysOnTheWholeTree(t *testing.T) {
	covered := []string{srcFile("b"), "go/internal/a/testdata", "go/internal/a/testdata/link.json", "go/internal/b", "go/internal"}
	for _, link := range covered {
		in := baseInput()
		in.Symlinks = []string{link}
		if !mustCompute(t, in).Whole {
			t.Errorf("a symlink at %s did not key on the whole tree", link)
		}
	}
	root := withRoot(baseInput(), "a", ".evolve/profiles", "r")
	root.Symlinks = []string{".evolve/profiles/x"}
	if !mustCompute(t, root).Whole {
		t.Error("a symlink under a data root did not key on the whole tree")
	}
	unrelated := baseInput()
	unrelated.Symlinks = []string{"docs/link.md", srcFile("c")}
	if mustCompute(t, unrelated).Whole {
		t.Error("a symlink outside the key keyed on the whole tree")
	}
}

func TestKey_APostRunKeyIsConsistentWithItsOwnLogOnly(t *testing.T) {
	logB := ParseLog("# test log\ngetenv W\nopen /repo/go/internal/a/testdata/x.json\n")
	logC := ParseLog("# test log\nopen /repo/go/internal/a/testdata/x.json\n")
	own := baseInput()
	own.Log = logB
	stale := baseInput()
	stale.Log = logC

	if !mustCompute(t, own).Consistent(logB) {
		t.Fatal("a key rejected the log that it was computed from")
	}
	if mustCompute(t, stale).Consistent(logB) {
		t.Fatal("a key computed from a log without W accepted a log that reads W")
	}
	outside := baseInput()
	outside.Log = ParseLog("# test log\nopen /etc/hosts\n")
	if mustCompute(t, stale).Consistent(outside.Log) {
		t.Fatal("a key accepted a log with an outside read that it was not computed from")
	}
	if mustCompute(t, stale).Consistent(ParseLog("")) {
		t.Fatal("a key accepted an unknown log")
	}
	if !mustCompute(t, outside).Consistent(outside.Log) {
		t.Fatal("a whole-tree key rejected its own log")
	}
}

func TestKey_ConsistentRefusesAKeyThatHidesWhatItsLogRead(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\ngetenv W\n")
	forgedWhole := mustCompute(t, in)

	noEnv := forgedWhole
	noEnv.Env = nil
	if noEnv.Consistent(in.Log) {
		t.Error("a key without the env entry of a name in its log was consistent")
	}
	unknown := baseInput()
	unknown.Log = ParseLog("")
	forged := mustCompute(t, unknown)
	forged.Whole = false
	if forged.Consistent(unknown.Log) {
		t.Error("an unknown log with a closure key was consistent")
	}
}

func TestKey_ValidRecomputesTheFullKeyFromItsParts(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\ngetenv W\n")
	in.Env = map[string]string{"W": "1"}
	k := mustCompute(t, in)

	if !k.Valid() {
		t.Fatal("a computed key was not valid")
	}
	edited := k
	edited.Full = strings.Repeat("0", 64)
	if edited.Valid() {
		t.Error("a key with another Full was valid")
	}
	changedEnv := k
	changedEnv.Env = []EnvValue{{Name: "W", Value: "2", Set: true}}
	if changedEnv.Valid() {
		t.Error("a key with edited env values was valid")
	}
}

func TestKey_ReadsRecordTheContextAndTheOpensOfTheRun(t *testing.T) {
	in := baseInput()
	in.Log = ParseLog("# test log\nopen /repo/go/internal/a/testdata/x.json\nopen /tmp/run-1/out\nstat /repo\n")

	var reads Reads = mustCompute(t, in).Reads
	var ctx Context = reads.Context

	if ctx.RepoRoot != repoRoot || ctx.WorkDir != "/repo/go/internal/a" || len(ctx.TempDirs) != 1 {
		t.Fatalf("context = %+v, want the context of the input", ctx)
	}
	want := []string{"open go/internal/a/testdata/x.json", "stat ."}
	if !slices.Equal(reads.Opens, want) {
		t.Fatalf("Opens = %v, want %v (the temp read is left out)", reads.Opens, want)
	}
}
