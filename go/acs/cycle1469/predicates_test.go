//go:build acs

package cycle1469

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const shipPkg = "./internal/phases/ship"

const manifestPkg = "./internal/shipmanifest"

var redContractFiles = []string{
	"go/internal/phases/ship/gitops_collider_quotepath_test.go",
	"go/internal/phases/ship/stage_quotepath_test.go",
}

func runShipTests(t *testing.T, root, pattern string) (string, int) {
	t.Helper()
	return runPkgTests(t, root, shipPkg, pattern)
}

func runPkgTests(t *testing.T, root, pkg, pattern string) (string, int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput(
		"go", "test", "-C", filepath.Join(root, "go"), pkg, "-run", pattern, "-count=1")
	out := stdout + stderr
	if code == -1 {
		t.Fatalf("could not run `go test -run %s`: %v\n%s", pattern, err, out)
	}
	return out, code
}

func assertShippable(t *testing.T, root, rel string) {
	t.Helper()
	if !acsassert.FileExists(t, filepath.Join(root, rel)) {
		t.Fatalf("RED: %s missing on disk — the contract it encodes does not exist", rel)
	}
	if _, _, code, _ := acsassert.SubprocessOutput(
		"git", "-C", root, "check-ignore", "-q", rel); code == 0 {
		t.Errorf("%s is gitignored — it will be dropped at ship and the RED contract it encodes evaporates", rel)
	}
}

func TestC1469_001_ColliderQuotePathDecodesGitOutput(t *testing.T) {
	root := acsassert.RepoRoot(t)
	assertShippable(t, root, redContractFiles[0])

	out, code := runShipTests(t, root, "TestDetectColliders_QuotePath")
	if code != 0 {
		t.Errorf("collider quote-path contract RED (exit %d) — a C-quoted incoming filename still fails to compare as its literal path, so the ff-merge pre-flight and the repair ladder both miss a real untracked main-side collider:\n%s", code, out)
	}
	for _, name := range []string{
		"TestDetectColliders_QuotePathDecodesPorcelainEntries",
		"TestDetectColliders_QuotePathDecodesDiffNameOnly",
		"TestDetectColliders_QuotePathNonCollidersStaySilent",
		"TestDetectColliders_QuotePathDisabledOnGitReads",
	} {
		if _, c := runShipTests(t, root, "^"+name+"$"); c != 0 {
			t.Errorf("%s did not pass (exit %d) — this predicate requires every named collider case to run and pass, not just the pattern to match nothing", name, c)
		}
	}
}

func TestC1469_002_RenameArrowParsedStructurally(t *testing.T) {
	root := acsassert.RepoRoot(t)
	assertShippable(t, root, redContractFiles[1])

	for _, name := range []string{
		"TestPorcelainChangedPaths_QuotedRenameArrowKeepsBothEndpoints",
		"TestStagedGonePaths_QuotedRenameArrowSourceDecodes",
		"TestPorcelainChangedPaths_RenameArrowMalformedIsSafe",
		"TestPorcelainChangedPaths_OrdinaryRenameArrowUnchanged",
	} {
		out, code := runPkgTests(t, root, manifestPkg, "^"+name+"$")
		if code != 0 {
			t.Errorf("%s RED (exit %d) — a quoted rename endpoint holding the delimiter is still torn into unbalanced-quote fragments, which `git add` rejects rc=128 and which fails the whole staging:\n%s", name, code, out)
		}
		if !strings.Contains(out, name) && !strings.Contains(out, "ok ") {
			t.Errorf("%s produced no recognisable go-test output — the pattern may have matched nothing:\n%s", name, out)
		}
	}
}

func TestC1469_003_ExistingQuotePathAndStagingContractsHold(t *testing.T) {
	root := acsassert.RepoRoot(t)

	if out, code := runPkgTests(t, root, manifestPkg, "TestPorcelainChangedPaths_QuotePath"); code != 0 {
		t.Errorf("pre-existing contract %q regressed (exit %d):\n%s", "TestPorcelainChangedPaths_QuotePath", code, out)
	}
	for _, pattern := range []string{
		"TestStageExplicitPaths_QuotePathDisabledOnGitReads",
		"TestDropIgnoredPaths_QuotePath",
		"TestShipDirect_CycleClass_StagesDeclaredPathsNotAddAll",
	} {
		out, code := runShipTests(t, root, pattern)
		if code != 0 {
			t.Errorf("pre-existing contract %q regressed (exit %d) — the quote-path fixes must not change ordinary staging or collider classification:\n%s", pattern, code, out)
		}
	}
}
