package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/covchanged"
	"github.com/mickeyyaya/evolve-loop/go/test/fixtures"
)

const coverSource = "package p\n\nfunc F(x int) int {\n\tif x > 0 {\n\t\treturn 1\n\t}\n\treturn 0\n}\n"

const coverDiff = "diff --git a/p/a.go b/p/a.go\n--- a/p/a.go\n+++ b/p/a.go\n@@ -4,0 +5 @@\n+\t\treturn 1\n"

const coverProfile = "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 1\nexample.com/m/p/a.go:4.11,6.3 1 0\nexample.com/m/p/a.go:7.2,7.10 1 1\n"

func coverModule(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{"go/go.mod": "module example.com/m\n\ngo 1.23\n", "go/p/a.go": coverSource, "go/coverage.txt": coverProfile}
	for k, v := range files {
		all[k] = v
	}
	for rel, text := range all {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	coverProfileAt(t, root, time.Now().Add(time.Hour))
	return root
}

func coverProfileAt(t *testing.T, root string, at time.Time) {
	t.Helper()
	if err := os.Chtimes(filepath.Join(root, "go", "coverage.txt"), at, at); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

func fakeCoverGit(t *testing.T, scripts map[string]fixtures.ExecResponse) {
	t.Helper()
	all := map[string]fixtures.ExecResponse{
		"git rev-parse":  {Stdout: "abc\n"},
		"git merge-base": {Stdout: "main\n"},
		"git diff":       {Stdout: coverDiff},
	}
	for k, v := range scripts {
		all[k] = v
	}
	orig := coverChangedRunner
	coverChangedRunner = (&fixtures.FakeExec{Scripts: all}).Run
	t.Cleanup(func() { coverChangedRunner = orig })
}

func TestCoverChanged_RefusesWithExit1AndNamesTheFunctionItsPercentAndCounts(t *testing.T) {
	fakeCoverGit(t, nil)
	root := coverModule(t, nil)

	out, errOut, code := runDispatch("cover", "changed", "--base", "main", "--root", root)

	if code != 1 {
		t.Fatalf("exit = %d, want 1; out=%s err=%s", code, out, errOut)
	}
	for _, want := range []string{"REFUSED p/a.go:3 p.F 66.7% (2/3)", "1 changed, 0 full, 1 refused, 0 exempt"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not hold %q:\n%s", want, out)
		}
	}
}

func TestCoverChanged_PassesWithExit0WhenEveryChangedFunctionIsFull(t *testing.T) {
	fakeCoverGit(t, nil)
	root := coverModule(t, map[string]string{"go/coverage.txt": strings.ReplaceAll(coverProfile, " 1 0\n", " 1 1\n")})

	out, errOut, code := runDispatch("cover", "changed", "--base", "main", "--root", filepath.Join(root, "go"))

	if code != 0 || !strings.Contains(out, "1 changed, 1 full, 0 refused") {
		t.Errorf("exit = %d, want 0 and a full count; out=%s err=%s", code, out, errOut)
	}
}

func TestCoverChanged_NamesAFunctionWithNoProfile(t *testing.T) {
	fakeCoverGit(t, map[string]fixtures.ExecResponse{
		"git diff": {Stdout: "diff --git a/p/b.go b/p/b.go\n--- /dev/null\n+++ b/p/b.go\n@@ -0,0 +3 @@\n+func G() {}\n"},
	})
	root := coverModule(t, map[string]string{"go/p/b.go": "package p\n\nfunc G() {}\n"})

	out, _, code := runDispatch("cover", "changed", "--base", "main", "--root", root)

	if code != 1 || !strings.Contains(out, "REFUSED p/b.go:3 p.G no profile") {
		t.Errorf("exit = %d, want 1 and the no-profile line:\n%s", code, out)
	}
}

func TestCoverChanged_JSONCarriesTheVerdictAndTheCleanBit(t *testing.T) {
	fakeCoverGit(t, nil)
	root := coverModule(t, nil)

	out, _, code := runDispatch("cover", "changed", "--base", "main", "--root", root, "--json", "--profile", filepath.Join(root, "go", "coverage.txt"))

	var got struct {
		Clean   bool              `json:"clean"`
		Refused []covchanged.Func `json:"refused"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 1 || got.Clean || len(got.Refused) != 1 || got.Refused[0].Key != "p.F" {
		t.Errorf("json = %+v (%v), exit %d; want p.F refused and clean=false:\n%s", got, err, code, out)
	}
}

func TestCoverChanged_ExitCodesSeparateUsageFromIO(t *testing.T) {
	fakeCoverGit(t, nil)
	root := coverModule(t, nil)
	noProfile := coverModule(t, nil)
	if err := os.Remove(filepath.Join(noProfile, "go", "coverage.txt")); err != nil {
		t.Fatal(err)
	}
	stale := coverModule(t, nil)
	coverProfileAt(t, stale, time.Now().Add(-time.Hour))
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"no verb":              {[]string{"cover"}, exitUsage, "usage"},
		"an unknown verb":      {[]string{"cover", "all"}, exitUsage, "usage"},
		"no base":              {[]string{"cover", "changed", "--root", root}, exitUsage, "--base"},
		"an unknown flag":      {[]string{"cover", "changed", "--base", "main", "--nope"}, exitUsage, "usage"},
		"the removed --allow":  {[]string{"cover", "changed", "--base", "main", "--root", root, "--allow", "x"}, exitUsage, "usage"},
		"an extra argument":    {[]string{"cover", "changed", "--base", "main", "--root", root, "extra"}, exitUsage, "extra"},
		"no module":            {[]string{"cover", "changed", "--base", "main", "--root", t.TempDir()}, exitIO, "go.mod"},
		"no profile yet":       {[]string{"cover", "changed", "--base", "main", "--root", noProfile}, exitIO, "make -C go test-integration"},
		"a stale profile":      {[]string{"cover", "changed", "--base", "main", "--root", stale}, exitIO, "make -C go test-integration"},
		"a named profile gone": {[]string{"cover", "changed", "--base", "main", "--root", root, "--profile", filepath.Join(root, "none.txt")}, exitIO, "none.txt"},
	} {
		t.Run(name, func(t *testing.T) {
			_, errOut, code := runDispatch(tc.args...)
			if code != tc.code || !strings.Contains(errOut, tc.want) {
				t.Errorf("exit = %d, want %d; stderr %q must hold %q", code, tc.code, errOut, tc.want)
			}
		})
	}
}

func TestCoverChanged_AVerdictItCannotWriteExitsIO(t *testing.T) {
	fakeCoverGit(t, nil)
	root := coverModule(t, nil)
	for name, tc := range map[string]struct {
		args []string
		w    *failingWriter
	}{
		"text": {[]string{"changed", "--base", "main", "--root", root}, &failingWriter{okWrites: 1}},
		"json": {[]string{"changed", "--base", "main", "--root", root, "--json"}, &failingWriter{}},
	} {
		var errOut strings.Builder
		if code := runCover(tc.args, nil, tc.w, &errOut); code != exitIO || !strings.Contains(errOut.String(), "broken pipe") {
			t.Errorf("%s: exit = %d, stderr %q; want exitIO naming the write error", name, code, errOut.String())
		}
	}
}

const strictFloorsPass = "./p 60\n"

func coverStrictModule(t *testing.T, floors string) string {
	t.Helper()
	root := coverModule(t, map[string]string{"go/.cover-strict": floors})
	return root
}

func TestCoverStrict_PassesWhenEveryPackageMeetsItsFloor(t *testing.T) {
	t.Parallel()
	root := coverStrictModule(t, strictFloorsPass)

	out, errOut, code := runDispatch("cover", "strict", "--root", root)

	if code != 0 || !strings.Contains(out, "cover strict: p: 66.7% >= 60.0% ok") {
		t.Errorf("exit = %d, want 0 and the ok line; out=%s err=%s", code, out, errOut)
	}
}

func TestCoverStrict_RefusesAPackageBelowItsFloorWithExit1(t *testing.T) {
	t.Parallel()
	root := coverStrictModule(t, "./p 100\n")

	out, _, code := runDispatch("cover", "strict", "--root", filepath.Join(root, "go"))

	if code != 1 || !strings.Contains(out, "cover strict: p: 66.7% < 100.0% FAIL") {
		t.Errorf("exit = %d, want 1 and the FAIL line:\n%s", code, out)
	}
}

func TestCoverStrict_JSONCarriesTheResultsAndTheCleanBit(t *testing.T) {
	t.Parallel()
	root := coverStrictModule(t, "./p 100\n")
	profile := filepath.Join(root, "go", "coverage.txt")
	floors := filepath.Join(root, "go", ".cover-strict")

	out, _, code := runDispatch("cover", "strict", "--root", root, "--json", "--profile", profile, "--floors", floors)

	var got struct {
		Clean   bool `json:"clean"`
		Results []struct {
			Pkg     string `json:"pkg"`
			Problem string `json:"problem"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil || code != 1 || got.Clean || len(got.Results) != 1 || got.Results[0].Pkg != "p" || got.Results[0].Problem == "" {
		t.Errorf("json = %+v (%v), exit %d; want p refused and clean=false:\n%s", got, err, code, out)
	}
}

func TestCoverStrict_ExitCodesSeparateUsageFromRefusalFromIO(t *testing.T) {
	t.Parallel()
	root := coverStrictModule(t, strictFloorsPass)
	noProfile := coverStrictModule(t, strictFloorsPass)
	if err := os.Remove(filepath.Join(noProfile, "go", "coverage.txt")); err != nil {
		t.Fatal(err)
	}
	stale := coverStrictModule(t, strictFloorsPass)
	coverProfileAt(t, stale, time.Now().Add(-time.Hour))
	noFloors := coverStrictModule(t, strictFloorsPass)
	if err := os.Remove(filepath.Join(noFloors, "go", ".cover-strict")); err != nil {
		t.Fatal(err)
	}
	badFloor := coverStrictModule(t, "./p 101\n")
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"an unknown flag":   {[]string{"cover", "strict", "--nope"}, exitUsage, "usage"},
		"an extra argument": {[]string{"cover", "strict", "--root", root, "extra"}, exitUsage, "extra"},
		"no module":         {[]string{"cover", "strict", "--root", t.TempDir()}, exitIO, "go.mod"},
		"no profile yet":    {[]string{"cover", "strict", "--root", noProfile}, exitRefused, "make -C go test-integration"},
		"a stale profile":   {[]string{"cover", "strict", "--root", stale}, exitRefused, "older than"},
		"no floors file":    {[]string{"cover", "strict", "--root", noFloors}, exitRefused, "floors file"},
		"a floor over 100":  {[]string{"cover", "strict", "--root", badFloor}, exitRefused, "over 100"},
	} {
		t.Run(name, func(t *testing.T) {
			_, errOut, code := runDispatch(tc.args...)
			if code != tc.code || !strings.Contains(errOut, tc.want) {
				t.Errorf("exit = %d, want %d; stderr %q must hold %q", code, tc.code, errOut, tc.want)
			}
		})
	}
}

func TestCoverStrict_AVerdictItCannotWriteExitsIO(t *testing.T) {
	t.Parallel()
	root := coverStrictModule(t, strictFloorsPass)
	for name, tc := range map[string]struct {
		args []string
		w    *failingWriter
	}{
		"text": {[]string{"strict", "--root", root}, &failingWriter{}},
		"json": {[]string{"strict", "--root", root, "--json"}, &failingWriter{}},
	} {
		var errOut strings.Builder
		if code := runCover(tc.args, nil, tc.w, &errOut); code != exitIO || !strings.Contains(errOut.String(), "broken pipe") {
			t.Errorf("%s: exit = %d, stderr %q; want exitIO naming the write error", name, code, errOut.String())
		}
	}
}

func TestCoverUsage_NamesBothVerbs(t *testing.T) {
	t.Parallel()
	_, errOut, code := runDispatch("cover")

	if code != exitUsage || !strings.Contains(errOut, "cover changed") || !strings.Contains(errOut, "cover strict") {
		t.Errorf("exit = %d; stderr %q must name both verbs", code, errOut)
	}
}
