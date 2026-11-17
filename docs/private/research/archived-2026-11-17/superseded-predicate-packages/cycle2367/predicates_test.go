//go:build acs

package cycle2367

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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

const ghLiteral = `"gh"`

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

func TestC2367_001_EveryGhCallSiteIsListedWithItsVerbAndPurpose(t *testing.T) {
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

func TestC2367_002_AGhCommandInACellFailsBeforeGhStarts(t *testing.T) {
	marker := stubGhOnPath(t)
	if err := sysexec.Command(context.Background(), "gh", "auth", "status").Run(); err != nil {
		t.Fatalf("control: gh outside a cell must start: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("control: the stub gh did not run, so the proof means nothing: %v", err)
	}
	for name, setup := range cellMarkers() {
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
		setup(t)
		err := sysexec.Command(context.Background(), "gh", "auth", "status").Run()
		if !errors.Is(err, sysexec.ErrGhInCell) {
			t.Errorf("%s: gh in a cell returned %v, want sysexec.ErrGhInCell", name, err)
		}
		if _, statErr := os.Stat(marker); statErr == nil {
			t.Errorf("%s: gh started inside the cell", name)
		}
		if err := sysexec.Command(context.Background(), "true").Run(); err != nil {
			t.Errorf("%s: a command other than gh must still run in a cell: %v", name, err)
		}
	}
}

func TestC2367_003_EveryProductionGhCallerFailsInACellWithoutStartingGh(t *testing.T) {
	marker := stubGhOnPath(t)
	ipcenvCell(t)
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
	for name, call := range callers {
		if err := call(); err == nil {
			t.Errorf("%s: a gh call in a cell must return an error", name)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Errorf("%s: gh started inside the cell", name)
		}
	}
}

func TestC2367_004_NoGitHubTokenKeyReachesACellUnapproved(t *testing.T) {
	want := []string{"GH_TOKEN", "GITHUB_TOKEN", "GH_ENTERPRISE_TOKEN", "GITHUB_ENTERPRISE_TOKEN"}
	unset := ipcenv.CredentialIsolation("gh", "git").Unset
	for _, key := range want {
		if !slices.Contains(unset, key) {
			t.Errorf("CredentialIsolation.Unset lacks %s: %v", key, unset)
		}
	}
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: "./internal/bridge",
		Pattern: "^(TestCellEnv_NoGitHubTokenReachesAnyCarrier|TestCellEnv_DropsEachUnsetKeyFromEverySource|TestCellEnv_IsolatesHostCredentialsOnEachCarrier)$",
		Names: []string{
			"TestCellEnv_NoGitHubTokenReachesAnyCarrier",
			"TestCellEnv_DropsEachUnsetKeyFromEverySource",
			"TestCellEnv_IsolatesHostCredentialsOnEachCarrier",
		},
	})
}

func TestC2367_005_ACellGetsAnEmptyGhConfigDirAndTheSecretClassDeniesAtShadow(t *testing.T) {
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: "./internal/bridge",
		Pattern: "^TestCellEnv_IsolatesHostCredentialsOnEachCarrier$",
		Names:   []string{"TestCellEnv_IsolatesHostCredentialsOnEachCarrier"},
	})
	acsassert.GoTests(t, acsassert.GoTestSpec{
		Dir:     goDir(t),
		Package: "./internal/adapters/sandbox",
		Pattern: "^TestReadIsolation_ShadowDeniesTheSecretClass$",
		Names:   []string{"TestReadIsolation_ShadowDeniesTheSecretClass"},
	})
}

func stubGhOnPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	marker := filepath.Join(dir, "gh-started")
	script := "#!/bin/sh\necho started > " + marker + "\necho '[]'\n"
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return marker
}

func ipcenvCell(t *testing.T) {
	t.Helper()
	t.Setenv(ipcenv.PhaseKey, "build")
}

func cellMarkers() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"phase marker": ipcenvCell,
		"dispatch depth marker": func(t *testing.T) {
			t.Helper()
			t.Setenv(ipcenv.PhaseKey, "")
			t.Setenv(ipcenv.DispatchDepthKey, "1")
		},
	}
}
