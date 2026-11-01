//go:build integration

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const splitTestOfF = "//go:build integration\n\npackage ptest_test\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/p\"\n)\n\nfunc TestF(t *testing.T) {\n\t_ = p.F\n%BODY%}\n"

func coverSplitModule(t *testing.T, splitBody string) string {
	t.Helper()
	module := coverChangedModule(t, "")
	writeFile(t, filepath.Join(module, "p", "ptest", "f_test.go"), strings.Replace(splitTestOfF, "%BODY%", splitBody, 1))
	writeFile(t, filepath.Join(module, ".cover-split"), "./p/ptest ./p\n")
	return module
}

func TestScopedCoverProfile_ASplitTestPackageKeepsTheCoverageOfThePackageItTests(t *testing.T) {
	t.Parallel()
	module := coverSplitModule(t, "\tif p.F(1) != 1 {\n\t\tt.Fatal(\"F(1)\")\n\t}\n")

	profile, out, status := scopedCoverProfile(context.Background(), ReviewInput{}, module, []string{"./p"}, []string{"integration"})
	defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()

	if branch, hit := fCoverage(t, profile); status != coverStatusOK || !branch || !hit {
		t.Errorf("status %d out %q branch hit %v: the split package's test of F must count for p", status, out, branch)
	}
}

func TestScopedCoverProfile_EveryFaultOfASplitPassIsNamed(t *testing.T) {
	const findProfile = "p=\"\"; prev=\"\"; split=\"\"; for a in \"$@\"; do [ \"$prev\" = -coverprofile ] && p=\"$a\"; case \"$a\" in -coverpkg=*) split=1;; esac; prev=\"$a\"; done\n"
	for name, tc := range map[string]struct {
		script, splitBody, want string
		status                  int
	}{
		"the split test fails":                 {"exec REALGO \"$@\"\n", "\tt.Fatal(\"split red\")\n", "split red", coverStatusTestsFailed},
		"the split pass writes no profile":     {findProfile + "[ -n \"$split\" ] && exit 0\nexec REALGO \"$@\"\n", "", "no such file", coverStatusPlumbingError},
		"the split profile has another mode":   {findProfile + "[ -n \"$split\" ] && { printf 'mode: count\\n' > \"$p\"; exit 0; }\nexec REALGO \"$@\"\n", "", "mode: count", coverStatusPlumbingError},
		"the merged profile cannot be written": {findProfile + "REALGO \"$@\" || exit $?\n[ -n \"$split\" ] && mkdir -p \"$(dirname \"$p\")/cover.out\"\nexit 0\n", "", "cover.out", coverStatusPlumbingError},
	} {
		t.Run(name, func(t *testing.T) {
			fakeGoOnPath(t, tc.script)
			module := coverSplitModule(t, tc.splitBody)
			profile, out, status := scopedCoverProfile(context.Background(), ReviewInput{}, module, []string{"./p"}, []string{"integration"})
			defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()
			if status != tc.status || !strings.Contains(out, tc.want) {
				t.Errorf("status %d out %q, want status %d naming %q", status, out, tc.status, tc.want)
			}
		})
	}
}

func TestScopedCoverProfile_ABadSplitListOrNoPackageIsAPlumbingError(t *testing.T) {
	t.Parallel()
	module := coverSplitModule(t, "")
	writeFile(t, filepath.Join(module, ".cover-split"), "./p/ptest\n")
	if profile, out, status := scopedCoverProfile(context.Background(), ReviewInput{}, module, []string{"./p"}, nil); status != coverStatusPlumbingError || profile != "" || !strings.Contains(out, ".cover-split") {
		t.Errorf("a bad split list: status %d profile %q out %q, want a plumbing error naming the list", status, profile, out)
	}
	empty := coverChangedModule(t, "")
	profile, out, status := scopedCoverProfile(context.Background(), ReviewInput{}, empty, nil, nil)
	defer func() { _ = os.RemoveAll(filepath.Dir(profile)) }()
	if status != coverStatusPlumbingError || !strings.Contains(out, "no coverage profile") {
		t.Errorf("no package: status %d out %q, want a plumbing error", status, out)
	}
}
