package covchanged

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParsePackageSet_ReadsOneCleanDirectoryPerLineAndSkipsComments(t *testing.T) {
	got, err := ParsePackageSet([]byte("# header\n\n./internal/bridge\n  cmd/evolve/  \n./internal/bridge\ninternal/x"))

	if err != nil {
		t.Fatal(err)
	}
	if want := (PackageSet{"internal/bridge": true, "cmd/evolve": true, "internal/x": true}); !reflect.DeepEqual(got, want) {
		t.Errorf("ParsePackageSet = %v, want %v", got, want)
	}
}

func TestParsePackageSet_RefusesALineThatNamesNoSinglePackageAndAnEmptySet(t *testing.T) {
	for name, text := range map[string]string{
		"a parent path":       "./internal/a\n../outside\n",
		"an absolute path":    "/internal/a\n",
		"a recursive pattern": "./internal/...\n",
		"the module root":     "./\n",
		"only comments":       "# none\n\n",
	} {
		if got, err := ParsePackageSet([]byte(text)); err == nil || !strings.Contains(err.Error(), "package") {
			t.Errorf("%s: ParsePackageSet = %v, %v; want an error", name, got, err)
		}
	}
	if _, err := ParsePackageSet([]byte("ok\n../bad\n")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("the error = %v, want it to name line 2", err)
	}
}

func TestPackageSetHolds_ANilSetHoldsEveryFile(t *testing.T) {
	set := PackageSet{"internal/a": true}
	if !set.Holds("internal/a/x.go") || set.Holds("internal/a/b/x.go") || set.Holds("internal/b/x.go") {
		t.Error("a set holds exactly the files of its own directories")
	}
	if !set.HasDir("internal/a") || set.HasDir("internal/b") || !PackageSet(nil).HasDir("any") {
		t.Error("HasDir must match a directory of the set, and a nil set must hold every directory")
	}
	if !PackageSet(nil).Holds("anything/x.go") {
		t.Error("a nil set must hold every file, so an unscoped check stays whole")
	}
}

func TestMeasure_ExemptsAChangedFileOutsideThePackageSet(t *testing.T) {
	src := "package p\n\nfunc F() int {\n\treturn 1\n}\n"
	fx := fixture{
		diff:    added("p/a.go", 4, "\treturn 1") + added("q/a.go", 4, "\treturn 1"),
		files:   map[string]string{"p/a.go": src, "q/a.go": src},
		profile: "example.com/m/p/a.go:3.14,4.10 1 1\n",
	}
	sources := fx.sources(t)
	sources.Packages = PackageSet{"p": true}

	got, err := Measure(sources)

	if err != nil || !reflect.DeepEqual(summarize(got.Funcs), []measured{{"p.F", 3, 1, 1, true}}) {
		t.Errorf("Measure = %+v, %v; want only p.F measured", got, err)
	}
	if want := []Exemption{{"q/a.go", ReasonOutsidePackages}}; !reflect.DeepEqual(got.Exempt, want) {
		t.Errorf("Exempt = %+v, want %+v", got.Exempt, want)
	}
}

func TestWorkspaceCheck_PassesThePackageSetToTheMeasure(t *testing.T) {
	fx := newWorkspace(t, wsFiles(nil), nil)
	ws := Workspace{ModuleDir: fx.dir, Run: fx.exec.Run}

	v, err := ws.Check(context.Background(), Options{Base: "main", Profile: filepath.Join(fx.dir, "coverage.txt"), Tags: DefaultTags, Packages: PackageSet{"other": true}})

	if err != nil || !v.Clean() || v.Changed != 0 || !reflect.DeepEqual(v.Exempt, []Exemption{{"p/a.go", ReasonOutsidePackages}}) {
		t.Errorf("Check = %+v, %v; want p/a.go exempt as outside the set", v, err)
	}
}
