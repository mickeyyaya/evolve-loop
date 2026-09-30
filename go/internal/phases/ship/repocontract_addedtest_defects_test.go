package ship

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

// newLaneRepo builds a temporary lane worktree that is GREEN for the four
// fixed guard suites, so anything a test observes afterwards is attributable
// to the added-test backstop alone.
func newLaneRepo(t *testing.T) string {
	t.Helper()
	repo := makeRepo(t)
	goDir := filepath.Join(repo, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	writeGreenGuardSuites(t, goDir)
	runGit(t, repo, "add", "go")
	runGit(t, repo, "commit", "-qm", "baseline: green guard suites")
	return repo
}

// readScanLog returns the run-dir scanner artifact, failing the test when it
// is absent — every gate run, green or red, owes one.
func readScanLog(t *testing.T, workspace string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(workspace, scanLogName))
	if err != nil {
		t.Fatalf("scan log must be written on every gate run: %v", err)
	}
	return string(body)
}

func containsAny(haystack string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

// TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip: a deliberately
// red-first reproducer that lands `t.Skip`-ped is the SAFE intermediate state
// and must ship — a gate that graded skip as failure would push lanes back
// to landing bare reds.
func TestRepoContractGate_NewlyAddedSkippedTestDoesNotBlockShip(t *testing.T) {
	repo := newLaneRepo(t)
	const path = "go/internal/reproduction/skip_test.go"
	mustWrite(t, filepath.Join(repo, path),
		"package reproduction\n\nimport \"testing\"\n\nfunc TestNewlyAddedSkipped(t *testing.T) { t.Skip(\"pending fix: tracked red-first reproducer\") }\n")
	runGit(t, repo, "add", path)

	ws := t.TempDir()
	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, io.Discard); err != nil {
		t.Fatalf("a t.Skip-ped newly added reproducer must not block the ship, got %v", err)
	}
	if log := readScanLog(t, ws); strings.Contains(log, "--- FAIL: TestNewlyAddedSkipped") {
		t.Fatalf("a skipped reproducer must not be recorded as a failure, got:\n%s", log)
	}
}

// TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen: an added
// file carrying `//go:build integration` and failing must not be silently
// graded green because its package also holds an untagged passing test — the
// backstop must run each added candidate under the build tags that file
// actually declares, and fail closed naming the failing test.
func TestRepoContractGate_NewlyAddedTaggedFailingTestIsNotSilentlyGreen(t *testing.T) {
	repo := newLaneRepo(t)
	pkgDir := filepath.Join(repo, "go", "internal", "reproduction")
	mustWrite(t, filepath.Join(pkgDir, "green_test.go"),
		"package reproduction\n\nimport \"testing\"\n\nfunc TestUntaggedGreen(t *testing.T) {}\n")
	const failingTest = "TestNewlyAddedIntegrationRed"
	mustWrite(t, filepath.Join(pkgDir, "integration_test.go"),
		"//go:build integration\n\npackage reproduction\n\nimport \"testing\"\n\nfunc "+failingTest+"(t *testing.T) { t.Fatal(\"deliberate red behind a build tag\") }\n")
	runGit(t, repo, "add", "go/internal/reproduction")

	err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
	if err == nil {
		t.Fatalf("a newly added FAILING test behind //go:build integration must block the ship; the untagged run reporting the package green is exactly incident 25040cea")
	}
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractGate {
		t.Fatalf("error = %v, want structured REPO_CONTRACT_GATE", err)
	}
	if !strings.Contains(err.Error(), failingTest) {
		t.Fatalf("gate error must name the tag-guarded failing test %s, got %q", failingTest, err)
	}
}

// TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed: a lone
// tag-guarded predicate package (the shape every cycle's own go/acs/cycleNNNN
// mints, this one included) has zero files under the default tag set, which
// `go test` reports as a build/setup failure that the classifier would
// otherwise grade a genuine RED. It must run under its own tag instead, and
// the scan log must say what was done with it rather than dropping it
// silently.
func TestRepoContractGate_NewlyAddedTagGuardedGreenPackageIsNotFalseRed(t *testing.T) {
	repo := newLaneRepo(t)
	const path = "go/acs/cycle9999/predicates_test.go"
	mustWrite(t, filepath.Join(repo, path),
		"//go:build acs\n\npackage cycle9999\n\nimport \"testing\"\n\nfunc TestC9999_001_Healthy(t *testing.T) {}\n")
	runGit(t, repo, "add", path)

	ws := t.TempDir()
	if err := runRepoContractGate(context.Background(), "enforce", repo, ws, io.Discard); err != nil {
		t.Fatalf("a tag-guarded added test package that is GREEN under its own tag must not red the gate (this shape is minted by every cycle, including this one), got %v", err)
	}
	log := readScanLog(t, ws)
	if !strings.Contains(log, "acs/cycle9999") {
		t.Fatalf("scan log must record how the tag-guarded candidate was handled (rerun under its tag, or an explicit exclusion) — silent omission is the false-green half of the same defect; got:\n%s", log)
	}
}

// TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded: a backstop that
// can disable itself invisibly is worse than no backstop — the ship report
// would claim coverage it never had.
func TestRepoContractGate_AddedTestDiscoveryFailureIsRecorded(t *testing.T) {
	root := t.TempDir() // a real Go module but not a git repo — how the underlying git query fails
	goDir := filepath.Join(root, "go")
	mustWrite(t, filepath.Join(goDir, "go.mod"), "module example.com/lane\n\ngo 1.24\n")
	writeGreenGuardSuites(t, goDir)

	ws := t.TempDir()
	err := runRepoContractGate(context.Background(), "enforce", root, ws, io.Discard)
	// An undiscoverable diff is an infrastructure gap, not a contract
	// violation — the distinct, re-dispatchable INFRA class, never a silent
	// green.
	se, ok := shiperr.AsShipError(err)
	if !ok || se.Code != shiperr.CodeRepoContractInfra {
		t.Fatalf("an undiscoverable diff is the INFRA class, got %v", err)
	}
	log := readScanLog(t, ws)
	if !strings.Contains(log, "added-test") {
		t.Fatalf("scan log must name the added-test backstop when its discovery step fails, got:\n%s", log)
	}
	if !containsAny(log, "unavailable", "failed", "disabled", "skipped") {
		t.Fatalf("scan log must state that the added-test backstop did NOT run (unavailable/failed/disabled), never leave the reader to assume it did; got:\n%s", log)
	}
}

// TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests: the
// operator must be able to tell which scan went red — the fixed-pack message
// must keep naming every guard suite, and an added-test message must
// identify itself as an added-test failure.
func TestRepoContractGate_RedMessagesDistinguishFixedPackFromAddedTests(t *testing.T) {
	t.Run("fixed pack RED still names every guard suite", func(t *testing.T) {
		swapRepoContractTest(t, redPack("internal/phasespec.TestCatalogParity"))
		err := runRepoContractGate(context.Background(), "enforce", evolveLoopLane(t), t.TempDir(), io.Discard)
		if err == nil {
			t.Fatal("fixed-pack RED must fail the ship")
		}
		for _, suite := range []string{"phasespec", "profiles", "phasecoherence", "routingtest", "rawgitratchet"} {
			if !strings.Contains(err.Error(), suite) {
				t.Errorf("fixed-pack RED must keep naming the guard suite %q so the operator knows where to look, got %q", suite, err)
			}
		}
	})

	t.Run("added-test RED identifies itself as an added-test failure", func(t *testing.T) {
		repo := newLaneRepo(t)
		const path = "go/internal/reproduction/red_test.go"
		const failingTest = "TestNewlyAddedRedNeedsAttribution"
		mustWrite(t, filepath.Join(repo, path),
			"package reproduction\n\nimport \"testing\"\n\nfunc "+failingTest+"(t *testing.T) { t.Fatal(\"deliberate red\") }\n")
		runGit(t, repo, "add", path)

		err := runRepoContractGate(context.Background(), "enforce", repo, t.TempDir(), io.Discard)
		if err == nil {
			t.Fatal("added-test RED must fail the ship")
		}
		if !containsAny(err.Error(), "added-test", "newly added") {
			t.Errorf("added-test RED must say the NEWLY ADDED test is what went red, not report it as the fixed scanner pack, got %q", err)
		}
		if !strings.Contains(err.Error(), failingTest) {
			t.Errorf("added-test RED must name %s, got %q", failingTest, err)
		}
	})
}

// TestRepoContractTestArgs_CarriesAnExplicitTimeout asserts the argv rather
// than an observed timeout, since the defect this guards is the missing flag
// itself — a test that actually waited out a deadline would have to burn one.
func TestRepoContractTestArgs_CarriesAnExplicitTimeout(t *testing.T) {
	args := repoContractTestArgs([]string{"./internal/core"}, nil)

	i := indexOfArg(args, "-timeout")
	if i < 0 {
		t.Fatalf("the gate must not leave `go test` on its 10m default — a slow-but-green pack "+
			"times out under fleet load and the panic reads as a real contract RED. args=%v", args)
	}
	if i+1 >= len(args) || args[i+1] != repoContractTestTimeout {
		t.Fatalf("-timeout must carry %q, got args=%v", repoContractTestTimeout, args)
	}
	if d, err := time.ParseDuration(repoContractTestTimeout); err != nil {
		t.Fatalf("repoContractTestTimeout %q is not a duration go test accepts: %v", repoContractTestTimeout, err)
	} else if d <= 10*time.Minute {
		t.Fatalf("repoContractTestTimeout %v is no headroom over Go's 10m default — ./internal/core "+
			"already measured 355.8s at ship, and the whole point is to clear it under load", d)
	}
	// The flag must precede the package list, or `go test` reads it as a package.
	if p := indexOfArg(args, "./internal/core"); p >= 0 && p < i {
		t.Fatalf("-timeout must come before the package operands, got args=%v", args)
	}
}

// TestRepoContractTestArgs_KeepsTagsAndPackagesAfterTheTimeout pins that
// extracting the argv builder did not disturb the flags the backstop's
// tag-grouped run depends on (the `//go:build acs` predicate packages).
func TestRepoContractTestArgs_KeepsTagsAndPackagesAfterTheTimeout(t *testing.T) {
	args := repoContractTestArgs([]string{"./acs/cycle1679"}, []string{"acs"})

	want := []string{"test", "-json", "-count=1", "-timeout", repoContractTestTimeout, "-tags", "acs", "./acs/cycle1679"}
	if len(args) != len(want) {
		t.Fatalf("argv shape changed: got %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("argv[%d] = %q, want %q (full: %v)", i, args[i], want[i], args)
		}
	}
}

func indexOfArg(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}
