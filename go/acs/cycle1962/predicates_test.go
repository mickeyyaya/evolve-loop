//go:build acs

package cycle1962

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	fixtureGoMod     = "module example.com/floorfixture\n\ngo 1.21\n"
	misformattedGo   = "package fixture\n\nfunc  Add( a,b int )int{return a+b}\n"
	formattedGo      = "package fixture\n\nfunc Add(a, b int) int { return a + b }\n"
	simplifiableOnly = "package fixture\n\nfunc Tail(s []int) []int {\n\treturn s[1:len(s)]\n}\n"
	vetFailingGo     = "package fixture\n\nimport \"fmt\"\n\nfunc Label() string {\n\treturn fmt.Sprintf(\"%d\", \"text\")\n}\n"
)

var evolveBuild struct {
	once      sync.Once
	dir, path string
	failure   string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if evolveBuild.dir != "" {
		if err := os.RemoveAll(evolveBuild.dir); err != nil {
			fmt.Fprintf(os.Stderr, "cycle1962: remove %s: %v\n", evolveBuild.dir, err)
		}
	}
	os.Exit(code)
}

func evolveBin(t *testing.T) string {
	t.Helper()
	goDir := filepath.Join(acsassert.RepoRoot(t), "go")
	evolveBuild.once.Do(func() { evolveBuild.failure = buildEvolve(goDir) })
	if evolveBuild.failure != "" {
		t.Fatalf("go build ./cmd/evolve: %s", evolveBuild.failure)
	}
	return evolveBuild.path
}

func buildEvolve(goDir string) string {
	dir, err := os.MkdirTemp("", "cycle1962-evolve-")
	if err != nil {
		return err.Error()
	}
	evolveBuild.dir, evolveBuild.path = dir, filepath.Join(dir, "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", evolveBuild.path, "./cmd/evolve")
	cmd.Dir = goDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Sprintf("%v\n%s", err, out)
	}
	return ""
}

type result struct {
	stdout, stderr string
	code           int
}

func (r result) String() string {
	return fmt.Sprintf("exit=%d\n--- stdout ---\n%s--- stderr ---\n%s", r.code, r.stdout, r.stderr)
}

func (r result) hasVerdict(status, tier string) bool {
	return regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(status) + ` +` + regexp.QuoteMeta(tier) + `\b`).MatchString(r.stdout)
}

func (r result) all() string { return r.stdout + r.stderr }

func isolatedEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		if strings.HasPrefix(name, "EVOLVE_") || strings.HasPrefix(name, "GIT_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func evolveFloor(t *testing.T, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, evolveBin(t), append([]string{"floor", "run"}, args...)...)
	cmd.Env = isolatedEnv()
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("evolve floor run %v: %v", args, err)
		}
		code = exitErr.ExitCode()
	}
	return result{stdout: stdout.String(), stderr: stderr.String(), code: code}
}

func hostToolchain() string {
	return strings.TrimPrefix(runtime.Version(), "go")
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func floorFixture(t *testing.T, goSource string, withCIConfig bool) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go", "go.mod"), fixtureGoMod)
	writeFile(t, filepath.Join(root, "go", "fixture.go"), goSource)
	if withCIConfig {
		writeFile(t, filepath.Join(root, ".github", "workflows", "go.yml"), fmt.Sprintf(`name: go
on: workflow_dispatch
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@v5
        with:
          go-version: '%s'
      - run: gofmt -d -s .
`, hostToolchain()))
	}
	return root
}

func TestC1962_001_FmtTierFlagsAFileGofmtWouldRewrite(t *testing.T) {
	root := floorFixture(t, misformattedGo, true)

	got := evolveFloor(t, "--tier", "fmt", "--root", root)

	if got.code != 1 {
		t.Errorf("RED: a misformatted file must exit 1\n%s", got)
	}
	if !got.hasVerdict("RED", "fmt") {
		t.Errorf("RED: no `RED fmt` verdict line\n%s", got)
	}
	if !strings.Contains(got.all(), "fixture.go") {
		t.Errorf("RED: the verdict never names the file gofmt would rewrite\n%s", got)
	}
}

func TestC1962_002_FmtTierFlagsASimplifyOnlyRewrite(t *testing.T) {
	root := floorFixture(t, simplifiableOnly, true)

	got := evolveFloor(t, "--tier", "fmt", "--root", root)

	if got.code != 1 || !got.hasVerdict("RED", "fmt") {
		t.Errorf("RED: CI runs gofmt -s, so a slice that only -s rewrites must be red\n%s", got)
	}
}

func TestC1962_003_FmtTierPassesAFormattedFile(t *testing.T) {
	root := floorFixture(t, formattedGo, true)

	got := evolveFloor(t, "--tier", "fmt", "--root", root)

	if got.code != 0 {
		t.Errorf("RED: a gofmt-clean module must exit 0\n%s", got)
	}
	if !got.hasVerdict("PASS", "fmt") {
		t.Errorf("RED: no `PASS fmt` verdict line\n%s", got)
	}
	if got.hasVerdict("RED", "fmt") {
		t.Errorf("RED: a clean file was flagged\n%s", got)
	}
}

func TestC1962_004_RedFmtTierDoesNotStopTheVetTier(t *testing.T) {
	root := floorFixture(t, misformattedGo, true)

	got := evolveFloor(t, "--tier", "fmt,vet", "--root", root)

	if got.code != 1 {
		t.Errorf("RED: one red tier must exit 1\n%s", got)
	}
	if !got.hasVerdict("RED", "fmt") {
		t.Errorf("RED: the red tier is not named\n%s", got)
	}
	if !got.hasVerdict("PASS", "vet") {
		t.Errorf("RED: the tier after the red one did not run and report\n%s", got)
	}
}

func TestC1962_005_RedVetTierIsNamedAndTheNextTierStillReports(t *testing.T) {
	root := floorFixture(t, vetFailingGo, true)

	got := evolveFloor(t, "--tier", "vet,fmt", "--root", root)

	if got.code != 1 {
		t.Errorf("RED: a vet failure must exit 1\n%s", got)
	}
	if !got.hasVerdict("RED", "vet") {
		t.Errorf("RED: the red tier is not named\n%s", got)
	}
	if !got.hasVerdict("PASS", "fmt") {
		t.Errorf("RED: the tier after the red one did not run and report\n%s", got)
	}
	if !strings.Contains(got.all(), "fixture.go") {
		t.Errorf("RED: the verdict carries no tail of the failing vet output\n%s", got)
	}
}

func TestC1962_006_AllGreenTiersExitZero(t *testing.T) {
	root := floorFixture(t, formattedGo, true)

	got := evolveFloor(t, "--tier", "fmt,vet", "--root", root)

	if got.code != 0 {
		t.Errorf("RED: every tier green must exit 0\n%s", got)
	}
	if !got.hasVerdict("PASS", "fmt") || !got.hasVerdict("PASS", "vet") {
		t.Errorf("RED: each green tier must print its own PASS line\n%s", got)
	}
}

func TestC1962_007_TierThatCannotRunExitsTwoNamingIt(t *testing.T) {
	root := floorFixture(t, formattedGo, false)

	got := evolveFloor(t, "--tier", "fmt", "--root", root)

	if got.code != 2 {
		t.Errorf("RED: with no CI config to pin the toolchain the fmt tier cannot run, want exit 2\n%s", got)
	}
	if !got.hasVerdict("ERROR", "fmt") {
		t.Errorf("RED: no `ERROR fmt` verdict line\n%s", got)
	}
	if got.hasVerdict("PASS", "fmt") {
		t.Errorf("RED: a tier that could not run was reported as passing\n%s", got)
	}
}

func TestC1962_008_UnknownTierExitsTwoWithoutRunningAnything(t *testing.T) {
	root := floorFixture(t, misformattedGo, true)

	got := evolveFloor(t, "--tier", "fmt,bogus", "--root", root)

	if got.code != 2 {
		t.Errorf("RED: an unknown tier must exit 2\n%s", got)
	}
	if !strings.Contains(got.stderr, "bogus") {
		t.Errorf("RED: stderr never names the unknown tier\n%s", got)
	}
	if got.hasVerdict("RED", "fmt") || got.hasVerdict("PASS", "fmt") {
		t.Errorf("RED: a refused invocation still ran the fmt tier\n%s", got)
	}
}
