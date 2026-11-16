package ship

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/testverdict"
)

const (
	purityModule   = "example.com/purity"
	purityAmbient  = "PURITY_FIXTURE_AMBIENT"
	puritySafe     = purityModule + "/internal/pure"
	purityEnv      = purityModule + "/internal/envreader"
	purityGit      = purityModule + "/internal/gitrunner"
	purityViaEnv   = purityModule + "/internal/viaenv"
	purityNotThere = purityModule + "/internal/ghost"
)

func purityFixture(t *testing.T) string {
	t.Helper()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module "+purityModule+"\n\ngo 1.24\n")
	mustWrite(t, filepath.Join(goDir, "internal", "pure", "pure.go"), "package pure\n\nfunc Add(a, b int) int { return a + b }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "pure", "pure_test.go"), "package pure\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Fatal(\"sum\")\n\t}\n}\n")
	mustWrite(t, filepath.Join(goDir, "internal", "envreader", "envreader.go"), "package envreader\n\nimport \"os\"\n\nfunc Ambient() string { return os.Getenv(\""+purityAmbient+"\") }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "envreader", "envreader_test.go"), "package envreader\n\nimport \"testing\"\n\nfunc TestAmbientIsUnset(t *testing.T) {\n\tif Ambient() != \"\" {\n\t\tt.Fatal(\"the ambient switch is on\")\n\t}\n}\n")
	mustWrite(t, filepath.Join(goDir, "internal", "gitrunner", "gitrunner.go"), "package gitrunner\n\nimport \"os/exec\"\n\nfunc Head() ([]byte, error) { return exec.Command(\"git\", \"rev-parse\", \"HEAD\").Output() }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "gitrunner", "gitrunner_test.go"), "package gitrunner\n\nimport \"testing\"\n\nfunc TestHeadExists(t *testing.T) { _ = Head }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "viaenv", "viaenv.go"), "package viaenv\n\nimport \""+purityEnv+"\"\n\nfunc Seen() string { return envreader.Ambient() }\n")
	mustWrite(t, filepath.Join(goDir, "internal", "viaenv", "viaenv_test.go"), "package viaenv\n\nimport \"testing\"\n\nfunc TestSeenExists(t *testing.T) { _ = Seen }\n")
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "purity fixture: pure, envreader, gitrunner, viaenv")
	return repo
}

func purityShadow(store string, center *signalcenter.Center) *scanShadow {
	return newScanShadow(testverdict.NewStore(store, time.Now), sysexec.DefaultRunner, center, shadowCycle, enforceLimits)
}

func isOnTheHandList(pkg string) bool {
	return slices.ContainsFunc(repoContractSelections, func(sel repocontract.TestSelection) bool {
		return strings.HasSuffix(pkg, strings.TrimPrefix(sel.Package, "."))
	})
}

func TestCacheable_RefusesPackagesThatReadTheEnvironmentOrGit(t *testing.T) {
	t.Parallel()
	repo := purityFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	center := signalcenter.New()
	s := purityShadow(store, center)
	s.begin(context.Background(), repo)
	s.observe(packOutcome{pkgs: []pkgResult{green(puritySafe), green(purityEnv), green(purityGit), green(purityViaEnv)}})
	s.finish()
	sum := summaryFields(t, center)
	if sum["packages"] != "4" || sum["uncacheable"] != "3" || sum["recorded"] != "1" {
		t.Fatalf("the env reader, the git runner and the importer of the env reader are uncacheable; only the pure package is stored; got %v", sum)
	}
	if st := storeStats(t, store); st.Entries != 1 {
		t.Fatalf("only the pure package may reach the store; got %+v", st)
	}
}

func TestCacheable_DefaultDenyForUnlistedImpurePackage(t *testing.T) {
	t.Parallel()
	repo := purityFixture(t)
	for _, pkg := range []string{purityGit, purityNotThere} {
		if isOnTheHandList(pkg) {
			t.Fatalf("fixture error: %s is on the hand list, so it proves no default-deny", pkg)
		}
		store := filepath.Join(t.TempDir(), shadowStoreDir)
		center := signalcenter.New()
		s := purityShadow(store, center)
		s.begin(context.Background(), repo)
		s.observe(packOutcome{pkgs: []pkgResult{green(pkg)}})
		s.finish()
		sum := summaryFields(t, center)
		if sum["uncacheable"] != "1" || sum["recorded"] != "0" || sum["unkeyed"] != "0" {
			t.Errorf("%s: a package that no list names and that purity does not prove is uncacheable; got %v", pkg, sum)
		}
		if st := storeStats(t, store); st.Entries != 0 {
			t.Errorf("%s: nothing may reach the store; got %+v", pkg, st)
		}
	}
}

func TestCacheable_AHandListedOverrideStillDeniesAPurePackage(t *testing.T) {
	t.Parallel()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	listed := strings.TrimPrefix(repoContractSelections[0].Package, "./")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module "+purityModule+"\n\ngo 1.24\n")
	mustWrite(t, filepath.Join(goDir, filepath.FromSlash(listed), "listed.go"), "package "+filepath.Base(listed)+"\n\nfunc Add(a, b int) int { return a + b }\n")
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "a pure package that sits on the hand list")
	pkg := purityModule + "/" + listed
	if !isOnTheHandList(pkg) {
		t.Fatalf("fixture error: %s must sit on the hand list", pkg)
	}
	center := signalcenter.New()
	s := purityShadow(filepath.Join(t.TempDir(), shadowStoreDir), center)
	s.begin(context.Background(), repo)
	s.observe(packOutcome{pkgs: []pkgResult{green(pkg)}})
	s.finish()
	if sum := summaryFields(t, center); sum["uncacheable"] != "1" || sum["recorded"] != "0" {
		t.Fatalf("an override on the hand list denies a package that purity proves; got %v", sum)
	}
}

func TestScanShadow_AmbientEnvChangeDoesNotHit(t *testing.T) {
	repo := purityFixture(t)
	store := filepath.Join(t.TempDir(), shadowStoreDir)
	args := repoContractTestArgs([]string{"./internal/pure", "./internal/envreader"}, nil)
	var notes strings.Builder
	scan := func() (reds []string, err error) {
		notes.Reset()
		testFn := func(ctx context.Context, moduleDir string, _ packLog) packOutcome {
			return runGoTestJSON(ctx, moduleDir, packLog{notes: &notes, raw: io.Discard}, args)
		}
		reds, _, err = runRecordedPack(context.Background(), purityShadow(store, signalcenter.New()), repo, testFn, io.Discard)
		return reds, err
	}

	if reds, err := scan(); err != nil || len(reds) != 0 {
		t.Fatalf("with the ambient switch off both packages are green: reds=%q err=%v\n%s", reds, err, notes.String())
	}

	t.Setenv(purityAmbient, "on")
	reds, err := scan()
	if err == nil || !strings.Contains(strings.Join(reds, " "), "envreader") {
		t.Fatalf("a changed environment must run the env-reading package again and fail it; it must not hit the stored green: reds=%q err=%v\nnotes: %s", reds, err, notes.String())
	}
	if !strings.Contains(notes.String(), "skipped 1 package(s)") || !strings.Contains(notes.String(), puritySafe) || strings.Contains(notes.String(), purityEnv) {
		t.Fatalf("only the pure package is skipped; got %q", notes.String())
	}
}
