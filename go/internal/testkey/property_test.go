package testkey

import (
	"maps"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"pgregory.net/rapid"
)

func drawNames(rt *rapid.T, label string) []string {
	return rapid.SliceOfNDistinct(rapid.StringMatching(`[a-z]{1,6}`), 0, 5, rapid.ID[string]).Draw(rt, label)
}

func shuffle[T any](rt *rapid.T, label string, xs []T) []T {
	out := slices.Clone(xs)
	for i := len(out) - 1; i > 0; i-- {
		j := rapid.IntRange(0, i).Draw(rt, label)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func reorderModule(rt *rapid.T, m overlap.Module) overlap.Module {
	pkgs := make([]overlap.Package, 0, len(m.Packages))
	for _, p := range shuffle(rt, "packages", m.Packages) {
		p.Files = shuffle(rt, "files", p.Files)
		p.Deps = shuffle(rt, "deps", p.Deps)
		pkgs = append(pkgs, p)
	}
	return overlap.Module{Path: m.Path, Packages: pkgs}
}

func TestKey_DoesNotDependOnTheOrderOfItsSetInputs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		in := baseInput()
		in.Toolchain.Tags = drawNames(rt, "tags")
		in.ChildVars = drawNames(rt, "child vars")
		in.TestImports = []string{importPath("h"), importPath("c")}
		in.Env = map[string]string{"x": "1", "y": "2"}
		in.Log = Log{Getenv: []string{"x", "y", "z"}, Ops: []Op{{"open", "/repo/go/internal/a/testdata/x"}}}
		in.UnresolvedRoots = []string{"zzz", "yyy"}
		want := mustCompute(t, in)

		shuffled := in
		shuffled.Toolchain.Tags = shuffle(rt, "tags", in.Toolchain.Tags)
		shuffled.ChildVars = shuffle(rt, "child vars", in.ChildVars)
		shuffled.TestImports = shuffle(rt, "test imports", in.TestImports)
		shuffled.Log.Getenv = shuffle(rt, "getenv", in.Log.Getenv)
		shuffled.UnresolvedRoots = shuffle(rt, "unresolved", in.UnresolvedRoots)
		shuffled.Module = reorderModule(rt, in.Module)

		got := mustCompute(t, shuffled)

		if got.Full != want.Full || got.File != want.File {
			rt.Fatalf("key moved from %+v to %+v on a reorder", want, got)
		}
	})
}

func TestKey_AnyChangeOfAKeyedInputChangesTheKey(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		in := baseInput()
		in.ChildVars = []string{"CHILD"}
		in.Env = map[string]string{"CHILD": "v"}
		in = withRoot(in, "a", ".evolve/p", "root-1")
		in.Log = ParseLog("# test log\ngetenv READ\nopen /repo/.evolve/p/x\n")
		in.Env["READ"] = "r"
		want := mustCompute(t, in)
		fresh := "z" + rapid.StringMatching(`[A-Za-z0-9]{1,8}`).Draw(rt, "value")

		mutate := rapid.SampledFrom([]string{"file", "testdata", "root", "tag", "flag", "goversion", "buildenv", "child", "read", "gomod"}).Draw(rt, "mutation")
		changed := in
		changed.Env = map[string]string{"CHILD": "v", "READ": "r"}
		changed.Toolchain.Tags = slices.Clone(in.Toolchain.Tags)
		changed.Toolchain.Flags = slices.Clone(in.Toolchain.Flags)
		switch mutate {
		case "file":
			changed = withIDs(changed, map[string]string{srcFile("b"): "id-b-" + fresh})
		case "gomod":
			changed = withIDs(changed, map[string]string{"go/go.mod": "mod-" + fresh})
		case "testdata":
			changed.TestdataIDs = maps.Clone(in.TestdataIDs)
			changed.TestdataIDs["go/internal/a"] = "td-" + fresh
		case "root":
			changed.RootIDs = map[string]string{".evolve/p": "root-" + fresh}
		case "tag":
			changed.Toolchain.Tags = append(changed.Toolchain.Tags, fresh)
		case "flag":
			changed.Toolchain.Flags = append(changed.Toolchain.Flags, "-"+fresh)
		case "goversion":
			changed.Toolchain.GoVersion = "go" + fresh
		case "buildenv":
			changed = withBuildEnv(changed, "GOFLAGS", fresh)
		case "child":
			changed.Env["CHILD"] = fresh
		case "read":
			changed.Env["READ"] = fresh
		}

		got := mustCompute(t, changed)

		if got.Full == want.Full {
			rt.Fatalf("mutation %q left the key at %s", mutate, got.Full)
		}
	})
}

func TestKey_ListBoundariesAreUnambiguous(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		a := rapid.StringMatching(`[a-z]{1,4}`).Draw(rt, "a")
		b := rapid.StringMatching(`[a-z]{1,4}`).Draw(rt, "b")
		split := baseInput()
		split.Toolchain.Tags = []string{a, b}
		joined := baseInput()
		joined.Toolchain.Tags = []string{a + b}

		if mustCompute(t, split).Full == mustCompute(t, joined).Full {
			rt.Fatalf("tags [%s %s] and [%s] gave one key", a, b, a+b)
		}
	})
}
