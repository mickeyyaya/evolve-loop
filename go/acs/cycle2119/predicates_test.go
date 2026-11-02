//go:build acs

package cycle2119

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	identityPackage = "./internal/bridge/phaseidentity"
	identitySource  = "internal/bridge/phaseidentity/identity.go"
	ttyLineLimit    = 1024
)

var (
	guardTestName = regexp.MustCompile(`(?i)tty|limit|length`)
	failedTestRE  = regexp.MustCompile(`(?m)^\s*--- FAIL: (\S+)`)
	passedTestRE  = regexp.MustCompile(`(?m)^\s*--- PASS: (\S+)`)
)

type mutation struct {
	block string
	old   string
	with  func(padding string) string
}

var mutations = []mutation{
	{
		block: "Authority",
		old:   `AuthorityHeading + "\n\n" +`,
		with:  func(p string) string { return `AuthorityHeading + "\n\n" + ` + p + ` + "\n" +` },
	},
	{
		block: "Block",
		old:   `b.WriteString(Heading + "\n\n")`,
		with:  func(p string) string { return `b.WriteString(Heading + "\n\n" + ` + p + ` + "\n")` },
	},
	{
		block: "WrapPasted",
		old:   `"<pasted_content id=%q>\n%s\n</pasted_content id=%q>\n"`,
		with: func(p string) string {
			return `"<pasted_content id=%q>\n%s\n</pasted_content id=%q>\n" + ` + p + ` + "\n"`
		},
	},
}

type packageRun struct {
	failed []string
	passed []string
	output string
}

func moduleDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir, err := acsassert.ModuleDir(wd)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func runIdentityPackage(t *testing.T, overlay string) packageRun {
	t.Helper()
	args := []string{"test", "-count=1", "-v"}
	if overlay != "" {
		args = append(args, "-overlay", overlay)
	}
	args = append(args, identityPackage)
	cmd := exec.CommandContext(context.Background(), "go", args...)
	cmd.Dir = moduleDir(t)
	cmd.WaitDelay = 10 * time.Second
	raw, _ := cmd.CombinedOutput()
	out := string(raw)
	run := packageRun{output: out}
	for _, m := range failedTestRE.FindAllStringSubmatch(out, -1) {
		run.failed = append(run.failed, m[1])
	}
	for _, m := range passedTestRE.FindAllStringSubmatch(out, -1) {
		run.passed = append(run.passed, m[1])
	}
	return run
}

func guardNamed(names []string) []string {
	var hit []string
	for _, n := range names {
		if guardTestName.MatchString(n) {
			hit = append(hit, n)
		}
	}
	return hit
}

func overlayWithPaddingLine(t *testing.T, m mutation, width int) string {
	t.Helper()
	mod := moduleDir(t)
	source := filepath.Join(mod, identitySource)
	raw, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	original := string(raw)
	if n := strings.Count(original, m.old); n != 1 {
		t.Fatalf("mutation site for %s matches %d times in %s, want exactly 1", m.block, n, identitySource)
	}
	padding := fmt.Sprintf("strings.Repeat(%q, %d)", "x", width)
	mutated := strings.Replace(original, m.old, m.with(padding), 1)
	dir := t.TempDir()
	mutatedFile := filepath.Join(dir, "identity_mutant.go")
	if err := os.WriteFile(mutatedFile, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {source: mutatedFile}})
	if err != nil {
		t.Fatal(err)
	}
	overlayFile := filepath.Join(dir, "overlay.json")
	if err := os.WriteFile(overlayFile, overlay, 0o644); err != nil {
		t.Fatal(err)
	}
	return overlayFile
}

func mutantRun(t *testing.T, m mutation, width int) packageRun {
	t.Helper()
	run := runIdentityPackage(t, overlayWithPaddingLine(t, m, width))
	if strings.Contains(run.output, "[build failed]") || strings.Contains(run.output, "cannot use") || strings.Contains(run.output, "undefined:") {
		t.Fatalf("the %s mutant did not compile, so it proves nothing:\n%s", m.block, acsassert.OutputTail(run.output, 20))
	}
	return run
}

func TestC2119_001_PackageStillPassesAndNamesALineLimitGuard(t *testing.T) {
	run := runIdentityPackage(t, "")
	if len(run.failed) != 0 {
		t.Fatalf("phaseidentity tests fail before any mutation: %v\n%s", run.failed, acsassert.OutputTail(run.output, 20))
	}
	if len(guardNamed(run.passed)) == 0 {
		t.Errorf("no passing test of phaseidentity has tty, limit or length in its name; the line-limit guard is not discoverable")
	}
}

func TestC2119_002_GuardFailsWhenAnyInjectedBlockPassesTheLimit(t *testing.T) {
	for _, m := range mutations {
		t.Run(m.block, func(t *testing.T) {
			run := mutantRun(t, m, ttyLineLimit)
			if len(guardNamed(run.failed)) == 0 {
				t.Errorf("a %d-byte line injected into %s fails no line-limit guard test; failing tests were %v", ttyLineLimit, m.block, run.failed)
			}
		})
	}
}

func TestC2119_003_GuardAcceptsALineJustUnderTheLimit(t *testing.T) {
	for _, m := range mutations {
		t.Run(m.block, func(t *testing.T) {
			run := mutantRun(t, m, ttyLineLimit-1)
			if hit := guardNamed(run.failed); len(hit) != 0 {
				t.Errorf("a %d-byte line injected into %s is under the limit, yet the guard failed: %v", ttyLineLimit-1, m.block, hit)
			}
		})
	}
}

func TestC2119_004_GuardFailureNamesTheOffendingBlockAndTheLimit(t *testing.T) {
	for _, m := range mutations {
		t.Run(m.block, func(t *testing.T) {
			run := mutantRun(t, m, ttyLineLimit+500)
			if len(guardNamed(run.failed)) == 0 {
				t.Fatalf("no line-limit guard test failed for an oversized %s line; failing tests were %v", m.block, run.failed)
			}
			if !strings.Contains(run.output, fmt.Sprint(ttyLineLimit)) {
				t.Errorf("the guard failure does not state the %d-byte limit:\n%s", ttyLineLimit, acsassert.OutputTail(run.output, 20))
			}
			if !strings.Contains(strings.ToLower(run.output), strings.ToLower(m.block)) {
				t.Errorf("the guard failure does not name the %s block:\n%s", m.block, acsassert.OutputTail(run.output, 20))
			}
		})
	}
}
