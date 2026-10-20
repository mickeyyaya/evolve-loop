//go:build acs

package cycle1997

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/repocontract"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	modulePath       = "github.com/mickeyyaya/evolve-loop/go/"
	pinName          = "TestGoTestSpawners_EveryProductionSpawnerUsesTheNamedProjection"
	plantedPath      = "internal/planted/planted.go"
	plantedViolation = plantedPath + ":plant spawns go test without building its env from ipcenv.GoTestEnv or ciparity.CIEnv"
)

func goDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(acsassert.RepoRoot(t), "go")
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	body, err := os.ReadFile(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(to, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func stagePlantedModule(t *testing.T, plantedSource string) string {
	t.Helper()
	src, dst := goDir(t), t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		copyFile(t, filepath.Join(src, name), filepath.Join(dst, name))
	}
	sources, err := filepath.Glob(filepath.Join(src, "internal", "ipcenv", "*.go"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("ipcenv sources = %v, err = %v", sources, err)
	}
	for _, source := range sources {
		copyFile(t, source, filepath.Join(dst, "internal", "ipcenv", filepath.Base(source)))
	}
	planted := filepath.Join(dst, filepath.FromSlash(plantedPath))
	if err := os.MkdirAll(filepath.Dir(planted), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(planted, []byte(plantedSource), 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}

func runPin(t *testing.T, moduleDir string) (output string, passed bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-run", "^"+pinName+"$", "./internal/ipcenv")
	cmd.Dir = moduleDir
	cmd.Env = ipcenv.GoTestEnv()
	cmd.WaitDelay = 10 * time.Second
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

func plantedSource(body string) string {
	return "package planted\n\nimport (\n\t\"context\"\n\t\"os\"\n\t\"os/exec\"\n\n" +
		"\t\"github.com/mickeyyaya/evolve-loop/go/internal/ciparity\"\n" +
		"\t\"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv\"\n" +
		"\t\"github.com/mickeyyaya/evolve-loop/go/internal/sysexec\"\n)\n\n" +
		"var _, _, _, _ = context.Background, os.Environ, exec.Command, ciparity.CIEnv\nvar _ = ipcenv.Scrub\nvar _ = sysexec.Command\n\n" + body + "\n"
}

func runNamed(t *testing.T, pkg, tags string, names ...string) {
	t.Helper()
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: modulePath + pkg,
		Pattern: "^(" + strings.Join(names, "|") + ")$",
		Names:   names,
		Tags:    tags,
	})
}

func TestC1997_001_AGoTestSpawnerOnTheNamedProjectionsPassesThePin(t *testing.T) {
	accepted := map[string]string{
		"goTestEnv":      `func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = ipcenv.GoTestEnv("EVOLVE_X=1"); _ = cmd.Run() }`,
		"ciEnv":          `func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = ciparity.CIEnv(os.Environ()); _ = cmd.Run() }`,
		"projectionHeld": `func plant() { env := ipcenv.GoTestEnv(); cmd := exec.Command("go", "test", "./..."); cmd.Env = env; _ = cmd.Run() }`,
		"notGoTest":      `func plant() { cmd := exec.Command("go", "build", "./..."); cmd.Env = os.Environ(); _ = cmd.Run() }`,
	}
	for name, body := range accepted {
		t.Run(name, func(t *testing.T) {
			out, passed := runPin(t, stagePlantedModule(t, plantedSource(body)))
			if !passed {
				t.Errorf("the pin rejected a spawner that is on the named projection:\n%s", out)
			}
		})
	}
}

func TestC1997_002_AGoTestSpawnerThatBuildsItsEnvAnyOtherWayFailsThePinByName(t *testing.T) {
	flagged := map[string]string{
		"noEnvAtAll":          `func plant() { cmd := exec.Command("go", "test", "./..."); _ = cmd.Run() }`,
		"ambientEnviron":      `func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = os.Environ(); _ = cmd.Run() }`,
		"scrubOnly":           `func plant(ctx context.Context) { cmd := sysexec.Command(ctx, "go", "test", "./..."); cmd.Env = ipcenv.Scrub(os.Environ()); _ = cmd.Run() }`,
		"nilEnvCapture":       `func plant(ctx context.Context, run sysexec.RunFunc) { _, _, _, _ = sysexec.Capture(ctx, run, ".", "go", "test", "./...") }`,
		"viaProgramVariable":  `func plant(ctx context.Context, bin string, extra []string) { cmd := sysexec.Command(ctx, bin, append([]string{"test"}, extra...)...); _ = cmd.Run() }`,
		"projectionNeverUsed": `func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = os.Environ(); _ = ipcenv.GoTestEnv(); _ = cmd.Run() }`,
		"ambientAppendedBack": `func plant() { cmd := exec.Command("go", "test", "./..."); cmd.Env = append(ipcenv.GoTestEnv(), os.Environ()...); _ = cmd.Run() }`,
	}
	for name, body := range flagged {
		t.Run(name, func(t *testing.T) {
			out, passed := runPin(t, stagePlantedModule(t, plantedSource(body)))
			if passed {
				t.Fatalf("the pin accepted a planted spawner that builds its env some other way than the named projection")
			}
			if !strings.Contains(out, plantedViolation) {
				t.Errorf("the pin failed without naming the planted spawner %q:\n%s", plantedViolation, out)
			}
		})
	}
}

func TestC1997_003_EveryProductionGoTestSpawnerOfTheRealTreePassesThePin(t *testing.T) {
	runNamed(t, "internal/ipcenv", "", pinName, "TestGoTestSpawners_TheTreeScanFindsEveryKnownProductionSpawner")
}

func TestC1997_004_PkgAcsassertGoTestsRunsItsChildWithoutTheCallersEvolveKeys(t *testing.T) {
	runNamed(t, "pkg/acsassert", "integration", "TestGoTests_TheChildGoTestSeesNoEvolveKeyOfItsCaller")
}

func TestC1997_005_ThePinsPackageIsInTheRepoContractPack(t *testing.T) {
	inPack := slices.ContainsFunc(repocontract.Packages(), func(pattern string) bool {
		return strings.TrimPrefix(strings.TrimSuffix(pattern, "..."), "./") == "internal/ipcenv/"
	})
	if !inPack {
		t.Errorf("repocontract.Packages() = %q, want internal/ipcenv because the pin reads the whole tree", repocontract.Packages())
	}
	runNamed(t, "internal/repocontract", "", "TestPackages_HoldEveryTestThatReadsTheWholeTree")
}

func TestC1997_006_TheSizeRatchetOffendersAreNotTouched(t *testing.T) {
	root := acsassert.RepoRoot(t)
	out, _, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "--name-only", "HEAD", "--", "go/internal/sizeratchet/offenders.json")
	if err != nil || code != 0 {
		t.Fatalf("git diff exit=%d: %v", code, err)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("go/internal/sizeratchet/offenders.json changed: %s", out)
	}
}
