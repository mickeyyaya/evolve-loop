package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/coversplit"
)

type passCall struct {
	pkgs []string
	tags []string
}

func stubCoverPass(t *testing.T) *[]passCall {
	t.Helper()
	var mu sync.Mutex
	var calls []passCall
	orig := coverPass
	coverPass = func(_ context.Context, _ int, _ string, profile string, args, tags []string) (string, int) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, passCall{slices.Clone(args), slices.Clone(tags)})
		if err := os.WriteFile(profile, []byte("mode: atomic\n"), 0o644); err != nil {
			return err.Error(), coverStatusPlumbingError
		}
		return "", coverStatusOK
	}
	t.Cleanup(func() { coverPass = orig })
	return &calls
}

func coverTierModule(t *testing.T) string {
	t.Helper()
	return tinyModule(t, map[string]string{"internal/bridge/a.go": "package bridge\n", "cmd/evolve/a.go": "package main\n"})
}

func TestScopedCoverProfile_OnlyTheTieredPackageRunsUnderItsExclusionTag(t *testing.T) {
	calls := stubCoverPass(t)
	in := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &strings.Builder{}}

	profile, _, status, _ := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./internal/bridge", "./cmd/evolve"}, []string{"integration"})
	defer os.RemoveAll(filepath.Dir(profile))

	if status != coverStatusOK || len(*calls) != 2 {
		t.Fatalf("status %d, calls %+v, want two go test calls", status, *calls)
	}
	want := []passCall{
		{[]string{"./cmd/evolve"}, []string{"integration"}},
		{[]string{"./internal/bridge"}, []string{"integration", "notmux"}},
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %+v, want %+v", *calls, want)
	}
}

func TestScopedCoverProfile_AHubPackageAloneRunsUnderIntegrationOnly(t *testing.T) {
	calls := stubCoverPass(t)
	in := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &strings.Builder{}}

	profile, _, status, _ := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./cmd/evolve"}, []string{"integration"})
	defer os.RemoveAll(filepath.Dir(profile))

	want := []passCall{{[]string{"./cmd/evolve"}, []string{"integration"}}}
	if status != coverStatusOK || !reflect.DeepEqual(*calls, want) {
		t.Fatalf("status %d, calls %+v, want %+v", status, *calls, want)
	}
}

func TestScopedCoverProfile_APackageWithoutAnExclusionTagKeepsTheGivenTagsInOneRun(t *testing.T) {
	calls := stubCoverPass(t)
	in := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &strings.Builder{}}

	profile, _, _, _ := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./cmd/evolve"}, nil)
	defer os.RemoveAll(filepath.Dir(profile))

	want := []passCall{{[]string{"./cmd/evolve"}, nil}}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatalf("calls = %+v, want %+v", *calls, want)
	}
}

func TestTierRuns_KeepsTheCoverPkgOfASplitRunAndDropsAnEmptiedRun(t *testing.T) {
	t.Parallel()
	runs := []coversplit.Run{{Packages: []string{"./internal/bridge"}, CoverPkg: "./x,./internal/bridge"}, {Packages: []string{"./p", "./internal/bridge"}}}
	got := tierRuns(runs)
	want := []coversplit.Run{
		{Packages: []string{"./p"}},
		{Packages: []string{"./internal/bridge"}, CoverPkg: "./x,./internal/bridge", Tags: []string{"integration", "notmux"}},
		{Packages: []string{"./internal/bridge"}, Tags: []string{"integration", "notmux"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tierRuns = %+v, want %+v", got, want)
	}
}

func TestPackageCoverTags_AreTheDefaultsPlusThePackageExclusionTag(t *testing.T) {
	t.Parallel()
	if got := packageCoverTags("./internal/bridge"); !reflect.DeepEqual(got, []string{"integration", "notmux"}) {
		t.Errorf("bridge tags = %v", got)
	}
	got := packageCoverTags("./cmd/evolve")
	got[0] = "mutated"
	if want := []string{"integration"}; !reflect.DeepEqual(packageCoverTags("./cmd/evolve"), want) {
		t.Errorf("hub tags = %v, want %v", packageCoverTags("./cmd/evolve"), want)
	}
}

func stubFailingCoverPass(t *testing.T, failing string) {
	t.Helper()
	orig := coverPass
	coverPass = func(_ context.Context, _ int, _ string, profile string, args, _ []string) (string, int) {
		if slices.Contains(args, failing) {
			return "FAIL\texample.com/m/internal/bridgex\t0.1s\n", coverStatusTestsFailed
		}
		if err := os.WriteFile(profile, []byte("mode: atomic\n"), 0o644); err != nil {
			return err.Error(), coverStatusPlumbingError
		}
		return "", coverStatusOK
	}
	t.Cleanup(func() { coverPass = orig })
}

func TestScopedCoverProfile_AFailingRunReturnsItsOwnTagSet(t *testing.T) {
	for name, tc := range map[string]struct {
		failing string
		want    []string
	}{
		"the tiered package": {"./internal/bridge", []string{"integration", "notmux"}},
		"the hub package":    {"./cmd/evolve", []string{"integration"}},
	} {
		stubFailingCoverPass(t, tc.failing)
		in := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &strings.Builder{}}

		profile, _, status, failed := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./internal/bridge", "./cmd/evolve"}, []string{"integration"})
		_ = os.RemoveAll(filepath.Dir(profile))

		if status != coverStatusTestsFailed || !reflect.DeepEqual(failed, tc.want) {
			t.Errorf("%s: status %d, failed tags %v, want a failure under %v", name, status, failed, tc.want)
		}
	}
}

func TestScopedCoverProfile_AGreenRunNamesNoFailedTags(t *testing.T) {
	stubCoverPass(t)
	in := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &strings.Builder{}}

	profile, _, status, failed := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./internal/bridge", "./cmd/evolve"}, []string{"integration"})
	_ = os.RemoveAll(filepath.Dir(profile))

	if status != coverStatusOK || failed != nil {
		t.Errorf("status %d, failed tags %v, want green and none", status, failed)
	}
}

func TestScopedCoverProfile_SplitPartsOfDifferentModesArePlumbingErrors(t *testing.T) {
	orig := coverPass
	t.Cleanup(func() { coverPass = orig })
	var calls int
	coverPass = func(_ context.Context, _ int, _ string, profile string, _, _ []string) (string, int) {
		calls++
		mode := "mode: atomic\n"
		if calls == 2 {
			mode = "mode: set\n"
		}
		if err := os.WriteFile(profile, []byte(mode), 0o644); err != nil {
			return err.Error(), coverStatusPlumbingError
		}
		return "", coverStatusOK
	}
	in := ReviewInput{ProjectRoot: t.TempDir(), Stderr: &strings.Builder{}}

	_, out, status, _ := scopedCoverProfile(context.Background(), in, coverTierModule(t), []string{"./internal/bridge", "./cmd/evolve"}, []string{"integration"})

	if status != coverStatusPlumbingError || !strings.Contains(out, "want") {
		t.Errorf("status %d, out %q, want a plumbing error that names the mode mismatch", status, out)
	}
}
