//go:build acs

package cycle1518

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const promotedPkgRel = "./acs/regression/cycle1515"

const promotedPkgDir = "go/acs/regression/cycle1515"

var promotedTests = []string{
	"TestC1515_001_ParkReleasesBindingAndPreservesPointer",
	"TestC1515_002_ParkOfUnboundItemInventsNoAnnotation",
	"TestC1515_003_ScopeResolveRefusesRetiredBinding",
	"TestC1515_004_ContinuationListShowsBindings",
	"TestC1515_005_ContinuationListOnEmptyRegistryIsCleanExit",
	"TestC1515_006_ContinuationReleaseReleasesAndAnnotates",
	"TestC1515_007_ContinuationReleaseRejectsUnknownScope",
	"TestC1515_008_ContinuationRejectsMalformedInvocations",
}

var productionPaths = []string{
	"go/cmd/evolve/",
	"go/internal/continuation/",
	"go/internal/inboxmover/",
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("no go.mod found walking up from %s", wd)
	return ""
}

func repoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Dir(moduleRoot(t))
}

func runIn(t *testing.T, dir, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %s %v in %s: %v\n%s", name, args, dir, err, buf.String())
	}
	return buf.String(), code
}

func TestC1518_001_PromotedPredicatesRunGreenOnTheDurablePath(t *testing.T) {
	goRoot := moduleRoot(t)

	out, code := runIn(t, goRoot, "go", "test", "-count=1", "-tags", "acs", "-v", promotedPkgRel)
	if code != 0 {
		t.Fatalf("RED: `go test -tags acs %s` exited %d — the cycle-1515 continuation predicates are not present-and-green on the durable path, so `acs-durable` CI still gives the shipped continuation CLI and park/consume release binding ZERO standing protection.\n%s",
			promotedPkgRel, code, out)
	}
	for _, name := range promotedTests {
		if !strings.Contains(out, "--- PASS: "+name) {
			t.Errorf("RED: %s did not report `--- PASS: %s` — that predicate was lost or renamed in the promotion, silently narrowing the durable gate.\n%s",
				promotedPkgRel, name, out)
		}
	}
	if strings.Contains(out, "--- SKIP: TestC1515_") {
		t.Errorf("RED: a promoted cycle-1515 predicate SKIPPED on the durable path — a gate that skips is not a gate.\n%s", out)
	}
}

func TestC1518_002_PromotedPackageIsSweptByTheDurableGateGlob(t *testing.T) {
	goRoot := moduleRoot(t)

	out, code := runIn(t, goRoot, "go", "list", "-tags", "acs", "-f", "{{.Dir}}", promotedPkgRel)
	if code != 0 {
		t.Fatalf("RED: `go list -tags acs %s` exited %d — the cycle-1515 predicates do not resolve as a package under the durable regression tree; they are still per-cycle-only (go/acs/cycle1515), which CI never sweeps.\n%s",
			promotedPkgRel, code, out)
	}
	dir := strings.TrimSpace(out)
	sweptRoot := filepath.Join(goRoot, "acs", "regression") + string(filepath.Separator)
	if !strings.HasPrefix(dir, sweptRoot) {
		t.Errorf("RED: the promoted package resolves to %q, which is OUTSIDE %q — the acs-durable job's `./acs/regression/...` pattern would not reach it, so the promotion protects nothing.", dir, sweptRoot)
	}
	if filepath.Base(dir) != "cycle1515" {
		t.Errorf("RED: promoted package directory is %q, expected basename %q — the pinned durable destination is %s.", dir, "cycle1515", promotedPkgDir)
	}
}

func TestC1518_003_PromotedPredicatesStillDriveProductionSeams(t *testing.T) {
	goRoot := moduleRoot(t)

	out, code := runIn(t, goRoot, "go", "list", "-tags", "acs",
		"-f", "{{join .TestImports \"\\n\"}}", promotedPkgRel)
	if code != 0 {
		t.Fatalf("RED: `go list -tags acs -f TestImports %s` exited %d — the promoted package does not resolve.\n%s", promotedPkgRel, code, out)
	}
	for _, want := range []string{
		"github.com/mickeyyaya/evolve-loop/go/internal/continuation",
		"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RED: the promoted package's test imports do not include %q — the promotion kept the test NAMES but lost the production seam they drive, so the durable gate protects nothing.\nTestImports:\n%s", want, out)
		}
	}

	const negative = "TestC1515_007_ContinuationReleaseRejectsUnknownScope"
	runOut, runCode := runIn(t, goRoot, "go", "test", "-count=1", "-tags", "acs", "-v",
		"-run", "^"+negative+"$", promotedPkgRel)
	if runCode != 0 {
		t.Fatalf("RED: the promoted negative predicate %s exited %d — the anti-no-op pin (a typo'd scope id must not read as a successful release) does not survive on the durable path.\n%s", negative, runCode, runOut)
	}
	if !strings.Contains(runOut, "--- PASS: "+negative) {
		t.Errorf("RED: `-run ^%s$` on %s ran NO such test — the negative predicate was dropped in the promotion.\n%s", negative, promotedPkgRel, runOut)
	}
}

func TestC1518_004_PromotionTouchesNoProductionCode(t *testing.T) {
	root := repoRoot(t)

	out, code := runIn(t, root, "git", "-C", root, "status", "--porcelain")
	if code != 0 {
		t.Fatalf("`git -C %s status --porcelain` exited %d:\n%s", root, code, out)
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		path := fields[len(fields)-1]
		for _, prod := range productionPaths {
			if strings.HasPrefix(path, prod) {
				t.Errorf("RED: this cycle's working tree modifies production path %q (%s) — the promotion is test-only; touching the already-shipped continuation code risks regressing the very behaviour the predicates pin.", path, line)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(root, promotedPkgDir)); err != nil {
		t.Errorf("RED: destination %s does not exist (%v) — nothing was promoted.", promotedPkgDir, err)
	}
}

// acs-predicate: config-check — this is an inherent configuration-presence
func TestC1518_005_DurableGateStillSweepsTheRegressionTree(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, ".github", "workflows", "ci.yml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	const want = "go test -count=1 -tags acs ./acs/regression/..."
	if !strings.Contains(string(raw), want) {
		t.Errorf("RED: %s no longer runs %q — the durable gate's recursive sweep is what makes promoting a predicate into acs/regression/ meaningful; without it the promotion protects nothing.", path, want)
	}
	if _, serr := os.Stat(filepath.Join(root, ".github")); serr != nil {
		t.Fatalf("no .github tree at %s: %v", root, serr)
	}
}
