//go:build acs

package cycle2475

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentclass"
	"github.com/mickeyyaya/evolve-loop/go/internal/ciwatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/gc"
	"github.com/mickeyyaya/evolve-loop/go/internal/integration"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	ghLiteral  = `"gh"`
	callsDoc   = "docs/operations/cell-gh-calls.md"
	bridgePkg  = "./internal/bridge"
	sandboxPkg = "./internal/adapters/sandbox"
)

var cellVerbSites = map[string]string{
	"status":            "cmd/evolve/cmd_status.go",
	"release-preflight": "internal/releasepreflight/releasepreflight.go",
	"ci classify":       "internal/ciwatch/classify_source.go",
}

func goDir(t *testing.T) string {
	t.Helper()
	dir, err := acsassert.ModuleDir(".")
	if err != nil {
		t.Fatalf("module dir: %v", err)
	}
	return dir
}

func ghSpawnSites(t *testing.T) []string {
	t.Helper()
	root := goDir(t)
	files, err := acsassert.WalkModule(root, func(rel string) bool {
		return strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && !strings.HasPrefix(rel, "acs/")
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}
	var sites []string
	for _, rel := range files {
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, rel), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", rel, err)
		}
		found := false
		ast.Inspect(parsed, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == ghLiteral {
				found = true
			}
			return !found
		})
		if found {
			sites = append(sites, filepath.ToSlash(rel))
		}
	}
	slices.Sort(sites)
	return sites
}

func stubGh(t *testing.T) (binDir, marker string) {
	t.Helper()
	binDir = t.TempDir()
	marker = filepath.Join(binDir, "gh-started")
	script := "#!/bin/sh\necho started > " + marker + "\necho '[]'\n"
	if err := os.WriteFile(filepath.Join(binDir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir, marker
}

func started(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

func leaveCell(t *testing.T) {
	t.Helper()
	for _, key := range ipcenv.CellMarkers() {
		t.Setenv(key, "")
	}
}

func enterCell(t *testing.T, marker string, value string) {
	t.Helper()
	leaveCell(t)
	t.Setenv(marker, value)
}

func cellEntries() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"phase marker":          func(t *testing.T) { enterCell(t, ipcenv.PhaseKey, "build") },
		"dispatch depth marker": func(t *testing.T) { enterCell(t, ipcenv.DispatchDepthKey, "1") },
	}
}

func TestC2475_001_EveryGhCallSiteIsListedWithItsVerbAndPurpose(t *testing.T) {
	calls := sysexec.GhCalls()
	if len(calls) == 0 {
		t.Fatal("sysexec.GhCalls() lists no gh call")
	}
	listed := map[string]bool{}
	cellSites := map[string]string{}
	for _, c := range calls {
		if c.Site == "" || c.Verb == "" || !strings.HasPrefix(c.Command, "gh ") || len(strings.Fields(c.Purpose)) < 3 {
			t.Errorf("incomplete gh call entry: %+v", c)
			continue
		}
		if _, err := os.Stat(filepath.Join(goDir(t), filepath.FromSlash(c.Site))); err != nil {
			t.Errorf("entry site %s does not exist: %v", c.Site, err)
		}
		listed[c.Site] = true
		if agentclass.Of(strings.Fields(c.Verb)).Class == agentclass.Allowed {
			cellSites[c.Verb] = c.Site
		}
	}
	for _, site := range ghSpawnSites(t) {
		if !listed[site] {
			t.Errorf("production file %s names gh and has no entry in the gh call table", site)
		}
		delete(listed, site)
	}
	for stale := range listed {
		t.Errorf("gh call table lists %s, which no longer names gh", stale)
	}
	for verb, site := range cellVerbSites {
		if cellSites[verb] != site {
			t.Errorf("cell verb %q must list its gh call at %s, got %q", verb, site, cellSites[verb])
		}
	}
}

func TestC2475_002_TheCellGhCallDocListsEveryCallWithItsPurpose(t *testing.T) {
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, filepath.FromSlash(callsDoc))
	if !acsassert.FileExists(t, doc) {
		t.Fatalf("%s is missing on disk", callsDoc)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", callsDoc); code != 0 {
		t.Errorf("%s is not tracked by git; it can be dropped at ship", callsDoc)
	}
	for _, c := range sysexec.GhCalls() {
		if !acsassert.LineContainsAll(doc, c.Site, c.Command, c.Purpose) {
			t.Errorf("%s has no line with the site %q, the command %q and the purpose %q", callsDoc, c.Site, c.Command, c.Purpose)
		}
	}
	if !acsassert.FileContains(t, doc, "ErrGhInCell") {
		t.Errorf("%s must state what a gh call returns inside a cell", callsDoc)
	}
}

func TestC2475_003_AGhCommandInACellFailsBeforeGhStarts(t *testing.T) {
	binDir, marker := stubGh(t)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	leaveCell(t)
	if err := sysexec.Command(context.Background(), "gh", "auth", "status").Run(); err != nil {
		t.Fatalf("control: gh outside a cell must start: %v", err)
	}
	if !started(marker) {
		t.Fatal("control: the stub gh did not run, so the proof means nothing")
	}
	for name, enter := range cellEntries() {
		caseBin, caseMarker := stubGh(t)
		t.Setenv("PATH", caseBin+string(os.PathListSeparator)+os.Getenv("PATH"))
		enter(t)
		for _, bin := range []string{"gh", filepath.Join(caseBin, "gh")} {
			err := sysexec.Command(context.Background(), bin, "auth", "status").Run()
			if !errors.Is(err, sysexec.ErrGhInCell) {
				t.Errorf("%s: %s in a cell returned %v, want sysexec.ErrGhInCell", name, bin, err)
			}
		}
		if started(caseMarker) {
			t.Errorf("%s: gh started inside the cell", name)
		}
		if err := sysexec.Command(context.Background(), "true").Run(); err != nil {
			t.Errorf("%s: a command other than gh must still run in a cell: %v", name, err)
		}
	}
}

func TestC2475_004_EveryLibraryGhCallerFailsInACellWithoutStartingGh(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir := t.TempDir()
	callers := map[string]func() error{
		"ciwatch.LatestRequiredRunOnBranch": func() error {
			_, err := ciwatch.LatestRequiredRunOnBranch(ctx, dir, "main")
			return err
		},
		"integration.SysGh": func() error {
			_, _, err := integration.SysGh()(ctx, dir, "pr", "list")
			return err
		},
		"gc.GhRemoteAPI.PullRequests": func() error {
			_, err := gc.GhRemoteAPI{Exec: sysexec.DefaultRunner, Dir: dir}.PullRequests(ctx)
			return err
		},
	}
	for entry, enter := range cellEntries() {
		for name, call := range callers {
			binDir, marker := stubGh(t)
			t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			enter(t)
			if err := call(); err == nil {
				t.Errorf("%s (%s): a gh call in a cell must return an error", name, entry)
			}
			if started(marker) {
				t.Errorf("%s (%s): gh started inside the cell", name, entry)
			}
		}
	}
}

func evolveBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "evolve")
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/evolve")
	build.Dir = goDir(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/evolve: %v\n%s", err, out)
	}
	return bin
}

func envWithoutCellMarkers(extra ...string) []string {
	markers := ipcenv.CellMarkers()
	env := ipcenv.Keep(os.Environ(), func(name string) bool { return !slices.Contains(markers, name) })
	return append(env, extra...)
}

func runStatus(t *testing.T, bin, root, binDir string, extra ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "status")
	cmd.Dir = root
	cmd.Env = envWithoutCellMarkers(append([]string{"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH")}, extra...)...)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

func TestC2475_005_TheStatusVerbOfACellNeverStartsGh(t *testing.T) {
	bin := evolveBinary(t)
	root := acsassert.RepoRoot(t)

	controlBin, controlMarker := stubGh(t)
	runStatus(t, bin, root, controlBin)
	if !started(controlMarker) {
		t.Fatal("control: evolve status outside a cell must run gh, or the proof means nothing")
	}

	cells := map[string][]string{
		"phase marker":          {ipcenv.PhaseKey + "=build"},
		"dispatch depth marker": {ipcenv.DispatchDepthKey + "=1"},
	}
	for name, extra := range cells {
		binDir, marker := stubGh(t)
		out := runStatus(t, bin, root, binDir, extra...)
		if started(marker) {
			t.Errorf("%s: evolve status started gh inside the cell", name)
		}
		if !strings.Contains(out, sysexec.ErrGhInCell.Error()) {
			t.Errorf("%s: evolve status must report why gh did not run:\n%s", name, out)
		}
	}
}

func TestC2475_006_TheHostGhConfigDirIsDeniedAndNoGrantReachesIt(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: bridgePkg,
		Pattern: "^(TestReadIsolation_TheHostGhConfigDirIsDeniedAndNoGrantReachesIt|TestReadIsolation_AMissingHomeOrAMissingGhDirLeavesNoGhPathInTheGrants)$",
		Names: []string{
			"TestReadIsolation_TheHostGhConfigDirIsDeniedAndNoGrantReachesIt",
			"TestReadIsolation_AMissingHomeOrAMissingGhDirLeavesNoGhPathInTheGrants",
		},
	})
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: sandboxPkg,
		Pattern: "^TestReadIsolation_ShadowDeniesTheSecretClass$",
		Names:   []string{"TestReadIsolation_ShadowDeniesTheSecretClass"},
	})
}

func TestC2475_007_NoGitHubTokenKeyReachesACellUnapproved(t *testing.T) {
	want := []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}
	unset := ipcenv.CredentialIsolation("gh", "git").Unset
	for _, key := range want {
		if !slices.Contains(unset, key) {
			t.Errorf("CredentialIsolation.Unset lacks %s: %v", key, unset)
		}
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: bridgePkg,
		Pattern: "^(TestCellEnv_NoGitHubTokenReachesAnyCarrier|TestCellEnv_DropsEachUnsetKeyFromEverySource|TestCellEnv_IsolatesHostCredentialsOnEachCarrier|TestCellEnv_NoOptInOrPassthroughLetsAGitHubTokenIntoACell)$",
		Names: []string{
			"TestCellEnv_NoGitHubTokenReachesAnyCarrier",
			"TestCellEnv_DropsEachUnsetKeyFromEverySource",
			"TestCellEnv_IsolatesHostCredentialsOnEachCarrier",
			"TestCellEnv_NoOptInOrPassthroughLetsAGitHubTokenIntoACell",
		},
	})
}
