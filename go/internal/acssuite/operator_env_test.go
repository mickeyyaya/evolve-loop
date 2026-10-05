package acssuite

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

const flagCampaignKey = "EVOLVE_FLAG_CAMPAIGN"

func planeWithPolicy(t *testing.T, policyJSON string) string {
	t.Helper()
	plane := t.TempDir()
	writeFixtureModule(t, plane, map[string]string{".evolve/policy.json": policyJSON})
	return plane
}

func planeWithTheCheckedInPolicy(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("read the checked-in policy: %v", err)
	}
	return planeWithPolicy(t, string(body))
}

func TestRun_PredicatesSeeTheOperatorKeysThePolicyNamesAndNoOther(t *testing.T) {
	setLaneIPCEnv(t)
	t.Setenv(flagCampaignKey, "1")
	t.Setenv("EVOLVE_OPERATOR_KEY_THE_POLICY_DOES_NOT_NAME", "1")
	plane := planeWithPolicy(t, `{"acs":{"predicate_env":["`+flagCampaignKey+`"]}}`)
	var envs [][]string
	if _, err := Run(Options{Root: t.TempDir(), ProjectRoot: plane, Cycle: 9, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}
	want := []string{flagCampaignKey, "EVOLVE_PROJECT_ROOT", ipcenv.WorktreeRootKey}
	sort.Strings(want)
	for _, raw := range envs {
		env := effectiveEnv(raw)
		if got := evolveKeys(env); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("predicate EVOLVE_ keys = %v, want %v: the suite's exports plus exactly the operator keys acs.predicate_env names", got, want)
		}
		if env[flagCampaignKey] != "1" {
			t.Errorf("%s = %q, want the operator's value 1", flagCampaignKey, env[flagCampaignKey])
		}
	}
}

func TestRun_ACycleWorktreesPolicyCannotWidenTheForwardedKeys(t *testing.T) {
	setLaneIPCEnv(t)
	t.Setenv(flagCampaignKey, "1")
	t.Setenv("EVOLVE_KEY_ONLY_THE_WORKTREE_NAMES", "1")
	root := planeWithPolicy(t, `{"acs":{"predicate_env":["EVOLVE_KEY_ONLY_THE_WORKTREE_NAMES","`+ipcenv.FleetKey+`"]}}`)
	plane := planeWithPolicy(t, `{"acs":{"predicate_env":["`+flagCampaignKey+`"]}}`)
	var envs [][]string
	if _, err := Run(Options{Root: root, ProjectRoot: plane, Cycle: 9, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range envs {
		env := effectiveEnv(raw)
		for _, key := range []string{"EVOLVE_KEY_ONLY_THE_WORKTREE_NAMES", ipcenv.FleetKey} {
			if _, leaked := env[key]; leaked {
				t.Errorf("%s reached a predicate because the cycle worktree's own policy named it; only the state root's policy decides what is forwarded", key)
			}
		}
		if env[flagCampaignKey] != "1" {
			t.Errorf("%s = %q, want the plane policy's forward to hold", flagCampaignKey, env[flagCampaignKey])
		}
	}
}

func TestRun_APolicyCannotForwardALaneProtocolKeyOrASuiteExport(t *testing.T) {
	setLaneIPCEnv(t)
	t.Setenv(flagCampaignKey, "1")
	t.Setenv("EVOLVE_PROJECT_ROOT", "/caller/plane")
	t.Setenv(ipcenv.WorktreeRootKey, "/caller/worktree")
	t.Setenv("CHANGED_PACKAGES", "./caller/pkg")
	refused := []string{ipcenv.FleetKey, ipcenv.CycleStateFileKey, ipcenv.TmuxSocketKey, "EVOLVE_PROJECT_ROOT", ipcenv.WorktreeRootKey, "CHANGED_PACKAGES"}
	root := t.TempDir()
	plane := planeWithPolicy(t, `{"acs":{"predicate_env":["`+strings.Join(append(refused, flagCampaignKey), `","`)+`"]}}`)
	var envs [][]string
	v, err := Run(Options{Root: root, ProjectRoot: plane, Cycle: 9, GoExec: captureScopeEnvs(&envs)})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range envs {
		env := effectiveEnv(raw)
		for _, key := range []string{ipcenv.FleetKey, ipcenv.CycleStateFileKey, ipcenv.TmuxSocketKey, "CHANGED_PACKAGES"} {
			if _, leaked := env[key]; leaked {
				t.Errorf("%s=%q reached a predicate through acs.predicate_env; a lane protocol key or a suite export is never forwarded", key, env[key])
			}
		}
		if env["EVOLVE_PROJECT_ROOT"] != plane || env[ipcenv.WorktreeRootKey] != root || env[flagCampaignKey] != "1" {
			t.Errorf("state root=%q source root=%q campaign=%q, want the suite's %q and %q and the forwarded 1", env["EVOLVE_PROJECT_ROOT"], env[ipcenv.WorktreeRootKey], env[flagCampaignKey], plane, root)
		}
	}
	for _, key := range refused {
		if !slices.ContainsFunc(v.Warnings, func(w string) bool { return strings.Contains(w, "acs.predicate_env") && strings.Contains(w, key) }) {
			t.Errorf("warnings %q do not name the refused key %s; a refused entry must be visible in the verdict", v.Warnings, key)
		}
	}
}

func TestCheckedInPredicateEnvNamesOnlyKeysACuratedPredicateReads(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pol struct {
		ACS struct {
			PredicateEnv []string `json:"predicate_env"`
		} `json:"acs"`
	}
	if err := json.Unmarshal(body, &pol); err != nil {
		t.Fatal(err)
	}
	read := curatedPredicateEnvReads(t, filepath.Join("..", "..", "acs"))
	for _, key := range pol.ACS.PredicateEnv {
		if !slices.Contains(read, key) {
			t.Errorf("the checked-in acs.predicate_env forwards %s, but no curated predicate reads it (reads: %v); drop the entry so the forward list stays the set the gates need", key, read)
		}
	}
}

func TestRun_ARealRegressionPredicateInAFleetLaneSeesTheFlagCampaignKey(t *testing.T) {
	if testing.Short() {
		t.Skip("runs a real go test subprocess")
	}
	setLaneIPCEnv(t)
	t.Setenv(flagCampaignKey, "1")
	root := t.TempDir()
	writeFixtureModule(t, root, map[string]string{
		"go/go.mod": "module fixture\n\ngo 1.23\n",
		"go/acs/regression/campaign/acs_test.go": "//go:build acs\n\npackage campaign\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\n" +
			"func TestCampaignKeyReachesThePredicate(t *testing.T) {\n" +
			"\tif got := os.Getenv(" + strconv.Quote(flagCampaignKey) + "); got != \"1\" {\n\t\tt.Errorf(\"flag campaign key = %q\", got)\n\t}\n}\n",
	})
	v, err := Run(Options{Root: root, ProjectRoot: planeWithTheCheckedInPolicy(t), Cycle: 7})
	if err != nil {
		t.Fatal(err)
	}
	if v.RedCount != 0 || v.GreenCount != 1 {
		t.Fatalf("green=%d red=%d reds=%+v, want the regression predicate green: under the checked-in policy an active flag campaign must reach the gate that reads it", v.GreenCount, v.RedCount, redsOf(v))
	}
}

func TestCuratedPredicatesReceiveEveryEvolveKeyTheyRead(t *testing.T) {
	keys := curatedPredicateEnvReads(t, filepath.Join("..", "..", "acs"))
	if !slices.Contains(keys, "EVOLVE_PROJECT_ROOT") {
		t.Fatalf("scan found %v; the red team reads EVOLVE_PROJECT_ROOT, so the scan is broken", keys)
	}
	for _, key := range keys {
		t.Setenv(key, "set-by-the-operator")
	}
	var envs [][]string
	if _, err := Run(Options{Root: t.TempDir(), ProjectRoot: planeWithTheCheckedInPolicy(t), Cycle: 9, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}
	env := effectiveEnv(envs[0])
	for _, key := range keys {
		if env[key] == "" {
			t.Errorf("a curated predicate reads %s, but the suite strips it under the checked-in policy, so that predicate runs as if it were unset: export it or name it in acs.predicate_env", key)
		}
	}
}

func curatedPredicateEnvReads(t *testing.T, acsDir string) []string {
	t.Helper()
	dirs := []string{filepath.Join(acsDir, "redteam")}
	entries, err := os.ReadDir(filepath.Join(acsDir, "regression"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(acsDir, "regression", e.Name()))
		}
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		for _, key := range evolveEnvReadsIn(t, dir) {
			seen[key] = true
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func evolveEnvReadsIn(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	files := make([]*ast.File, 0, len(paths))
	consts := map[string]string{}
	for _, p := range paths {
		file, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
		collectStringConsts(file, consts)
	}
	var keys []string
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isOSEnvRead(call.Fun) || len(call.Args) == 0 {
				return true
			}
			key, resolved := constantString(call.Args[0], consts)
			if !resolved {
				t.Errorf("%s: an environment read whose key is not a constant of its package; spell it as one so this guard can check it", fset.Position(call.Pos()))
			}
			if strings.HasPrefix(key, "EVOLVE_") {
				keys = append(keys, key)
			}
			return true
		})
	}
	return keys
}

func collectStringConsts(file *ast.File, consts map[string]string) {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			vs := spec.(*ast.ValueSpec)
			for i, name := range vs.Names {
				if i < len(vs.Values) {
					if value, ok := constantString(vs.Values[i], nil); ok {
						consts[name.Name] = value
					}
				}
			}
		}
	}
}

func isOSEnvRead(fun ast.Expr) bool {
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv")
}

func constantString(expr ast.Expr, consts map[string]string) (string, bool) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		value, err := strconv.Unquote(e.Value)
		return value, e.Kind == token.STRING && err == nil
	case *ast.Ident:
		value, ok := consts[e.Name]
		return value, ok
	}
	return "", false
}

func TestRun_ACallersChangedPackagesNeverReachAPredicate(t *testing.T) {
	t.Setenv("CHANGED_PACKAGES", "./caller/stale/pkg")
	var envs [][]string
	if _, err := Run(Options{Root: t.TempDir(), Cycle: 9, GoExec: captureScopeEnvs(&envs)}); err != nil {
		t.Fatal(err)
	}
	for _, raw := range envs {
		if got, ok := effectiveEnv(raw)["CHANGED_PACKAGES"]; ok {
			t.Errorf("CHANGED_PACKAGES = %q reached a predicate from the caller's environment; it is the suite's own export, set only from the cycle's build handoff", got)
		}
	}
}
