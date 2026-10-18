package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/coverstrict"
)

func TestChangedStrictResults_JudgesOnlyListedChangedPackagesAndFailsLoudOnBadInput(t *testing.T) {
	t.Parallel()

	const goMod = "module example.com/m\n\ngo 1.23\n"
	const profile = "mode: set\nexample.com/m/p/a.go:3.19,4.11 1 1\nexample.com/m/p/b.go:3.14,5.2 1 0\n"
	for name, tc := range map[string]struct {
		goMod       string
		floors      string
		floorsIsDir bool
		profile     string
		pkgs        []string
		wantPkgs    []string
		wantErr     string
	}{
		"no floors file is silent":                 {goMod: goMod, profile: profile, pkgs: []string{"./p"}},
		"an unlisted changed package is silent":    {goMod: goMod, floors: "./q 100\n", profile: profile, pkgs: []string{"./p"}},
		"a listed changed package is measured":     {goMod: goMod, floors: "./p 100\n./q 100\n", profile: profile, pkgs: []string{"./p"}, wantPkgs: []string{"p"}},
		"an unreadable floors file fails loud":     {goMod: goMod, floorsIsDir: true, profile: profile, pkgs: []string{"./p"}, wantErr: ".cover-strict"},
		"a malformed floor line fails loud":        {goMod: goMod, floors: "./p\n", profile: profile, pkgs: []string{"./p"}, wantErr: "floors line 1"},
		"a module without go.mod fails loud":       {floors: "./p 100\n", profile: profile, pkgs: []string{"./p"}, wantErr: "go.mod"},
		"a missing profile fails loud":             {goMod: goMod, floors: "./p 100\n", pkgs: []string{"./p"}, wantErr: "cover.out"},
		"a profile without a mode line fails loud": {goMod: goMod, floors: "./p 100\n", profile: "garbage\n", pkgs: []string{"./p"}, wantErr: "mode line"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			module := t.TempDir()
			if tc.goMod != "" {
				writeFile(t, filepath.Join(module, "go.mod"), tc.goMod)
			}
			if tc.floors != "" {
				writeFile(t, filepath.Join(module, ".cover-strict"), tc.floors)
			}
			if tc.floorsIsDir {
				if err := os.MkdirAll(filepath.Join(module, ".cover-strict"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			profilePath := filepath.Join(module, "cover.out")
			if tc.profile != "" {
				writeFile(t, profilePath, tc.profile)
			}

			results, err := changedStrictResults(module, profilePath, tc.pkgs)

			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want one naming %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			var got []string
			for _, r := range results {
				got = append(got, r.Pkg)
			}
			if strings.Join(got, ",") != strings.Join(tc.wantPkgs, ",") {
				t.Errorf("measured packages = %v, want %v", got, tc.wantPkgs)
			}
			if len(results) == 1 && results[0].Line() != "cover strict: p: 50.0% < 100.0% FAIL" {
				t.Errorf("line = %q, want p at 50.0%% below its 100%% floor", results[0].Line())
			}
		})
	}
}

func TestCoverStrictFailures_ARunnerErrorWarnsAndLeavesTheHandoffAlone(t *testing.T) {
	orig := coverStrictRunner
	coverStrictRunner = func(string, string, []string) ([]coverstrict.Result, error) {
		return nil, errors.New("floors unreadable")
	}
	t.Cleanup(func() { coverStrictRunner = orig })

	var failures []string
	stderr := captureStderr(t, func() {
		failures = coverStrictFailures(config.StageEnforce, t.TempDir(), "cover.out", []string{"./p"}, blockNoter{})
	})

	if failures != nil {
		t.Errorf("failures = %v, want none on a plumbing fault", failures)
	}
	if !strings.Contains(stderr, "WARN cover-changed could not check go/.cover-strict (floors unreadable)") {
		t.Errorf("stderr = %q, want a warning that names the runner error", stderr)
	}
}
