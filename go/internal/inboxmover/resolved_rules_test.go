package inboxmover

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestResolved_Rules(t *testing.T) {
	plane := t.TempDir()
	other := t.TempDir()
	planeInbox := filepath.Join(plane, ".evolve", "inbox")
	wtPlane := makeRepo(t)
	wt := cycleWorktreeOf(t, wtPlane)
	rows := []struct {
		name    string
		opts    Options
		wantErr error
		wantRoo string
	}{
		{"an inbox under the plane names its root", Options{InboxDir: planeInbox}, nil, plane},
		{"an inbox with a trailing slash names its root", Options{InboxDir: planeInbox + "/"}, nil, plane},
		{"an explicit root with no inbox takes the plane inbox", Options{ProjectRoot: plane}, nil, plane},
		{"an inbox off the plane shape names no root", Options{InboxDir: filepath.Join(plane, "custom-inbox")}, errNoProjectRoot, ""},
		{"an inbox under .evolve/queue names no root", Options{InboxDir: filepath.Join(plane, ".evolve", "queue")}, errNoProjectRoot, ""},
		{"no root and no inbox", Options{}, errNoProjectRoot, ""},
		{"a relative inbox gives a relative root", Options{InboxDir: filepath.Join(".evolve", "inbox")}, errNoProjectRoot, ""},
		{"an explicit relative root", Options{ProjectRoot: "."}, errNoProjectRoot, ""},
		{"two planes", Options{ProjectRoot: plane, InboxDir: filepath.Join(other, ".evolve", "inbox")}, errRootInboxMismatch, ""},
		{"a cycle worktree root with the plane inbox", Options{ProjectRoot: wt, InboxDir: filepath.Join(wtPlane, ".evolve", "inbox")}, errRootInboxMismatch, ""},
		{"a trailing dot under a cycle worktree", Options{ProjectRoot: wt, InboxDir: filepath.Join(wt, ".evolve", "inbox") + "/."}, errCycleWorktreeInbox, ""},
		{"a foreign ledger path with no ledger", Options{ProjectRoot: plane, LedgerPath: filepath.Join(other, "ledger.jsonl")}, errRootInboxMismatch, ""},
		{"a foreign ledger path with a ledger given", Options{ProjectRoot: plane, LedgerPath: filepath.Join(other, "ledger.jsonl"), Ledger: &recordingAppender{}}, nil, plane},
		{"the plane ledger path", Options{ProjectRoot: plane, LedgerPath: filepath.Join(plane, ".evolve", "ledger.jsonl")}, nil, plane},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			got, err := row.opts.resolved()
			if !errors.Is(err, row.wantErr) {
				t.Fatalf("err = %v, want %v", err, row.wantErr)
			}
			if row.wantErr != nil {
				return
			}
			if got.ProjectRoot != row.wantRoo {
				t.Errorf("ProjectRoot = %q, want %q", got.ProjectRoot, row.wantRoo)
			}
			if want := filepath.Join(row.wantRoo, ".evolve", "inbox"); got.InboxDir != want && filepath.Clean(got.InboxDir) != want {
				t.Errorf("InboxDir = %q, want %q", got.InboxDir, want)
			}
		})
	}
}

func quietStrayPlane(t *testing.T) {
	t.Helper()
	_, existed := os.Stat(".evolve")
	t.Cleanup(func() {
		if _, err := os.Stat(".evolve"); err == nil && existed != nil {
			_ = os.RemoveAll(".evolve")
			t.Errorf("a call wrote .evolve/ under the working directory: the lifecycle row went to the wrong tree")
		}
	})
}

func offShapeOptions(t *testing.T) (Options, string) {
	t.Helper()
	dir := t.TempDir()
	inbox := filepath.Join(dir, "custom-inbox")
	if err := os.MkdirAll(filepath.Join(inbox, "processing", "cycle-9"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{filepath.Join(inbox, "task-1.json"), filepath.Join(inbox, "processing", "cycle-9", "task-2.json")} {
		if err := os.WriteFile(p, []byte(`{"id":"`+strings.TrimSuffix(filepath.Base(p), ".json")+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return Options{InboxDir: inbox}, dir
}

func snapshotTree(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	return paths
}

func TestFacades_RefuseAnOffShapeInboxAndWriteNothing(t *testing.T) {
	quietStrayPlane(t)
	opts, dir := offShapeOptions(t)
	before := snapshotTree(t, dir)
	for name, call := range facadeCalls(opts) {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, errNoProjectRoot) {
				t.Fatalf("err = %v, want errNoProjectRoot", err)
			}
		})
	}
	if after := snapshotTree(t, dir); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("a refused call changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestScopeFacades_RefuseWithAnError(t *testing.T) {
	quietStrayPlane(t)
	opts, dir := offShapeOptions(t)
	before := snapshotTree(t, dir)
	for name, call := range scopeCalls(opts) {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, errNoProjectRoot) {
				t.Fatalf("err = %v, want errNoProjectRoot", err)
			}
		})
	}
	if after := snapshotTree(t, dir); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("a refused call changed the tree:\nbefore %v\nafter  %v", before, after)
	}
}

func TestFacades_ARootInboxMismatchRefuses(t *testing.T) {
	plane, other := makeRepo(t), makeRepo(t)
	wt := cycleWorktreeOf(t, plane)
	cases := map[string]Options{
		"two planes":                        {ProjectRoot: plane, InboxDir: filepath.Join(other, ".evolve", "inbox")},
		"a worktree root and a plane inbox": {ProjectRoot: wt, InboxDir: filepath.Join(plane, ".evolve", "inbox")},
	}
	for label, opts := range cases {
		for name, call := range facadeCalls(opts) {
			t.Run(label+"/"+name, func(t *testing.T) {
				if err := call(); !errors.Is(err, errRootInboxMismatch) {
					t.Fatalf("err = %v, want errRootInboxMismatch", err)
				}
			})
		}
		for name, call := range scopeCalls(opts) {
			t.Run(label+"/"+name, func(t *testing.T) {
				if err := call(); !errors.Is(err, errRootInboxMismatch) {
					t.Fatalf("err = %v, want errRootInboxMismatch", err)
				}
			})
		}
	}
}

var noRefusalFacades = map[string]bool{
	"ResolveContinuation":    true,
	"ClassifyHolder":         true,
	"FindScopeItemFile":      true,
	"FilterPromotionScope":   true,
	"ReadFailureCount":       true,
	"ReadLastFailureClass":   true,
	"ResolveDispatchability": true,
	"PendingDispatchability": true,
	"ResolveDispatchState":   true,
	"PlaceOnLaneMenu":        true,
	"PartitionLaneMenu":      true,
	"RankLaneMenu":           true,
	"ClaimLaneScope":         true,
}

func exportedFacadesTakingOptions(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool { return !strings.HasSuffix(fi.Name(), "_test.go") }, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || !fn.Name.IsExported() {
					continue
				}
				for _, p := range fn.Type.Params.List {
					if id, ok := p.Type.(*ast.Ident); ok && id.Name == "Options" {
						names = append(names, fn.Name.Name)
					}
				}
			}
		}
	}
	return names
}

func TestFacadeTable_CoversTheExportedAPI(t *testing.T) {
	refusing := map[string]bool{}
	for name := range facadeCalls(Options{}) {
		refusing[name] = true
	}
	for name := range scopeCalls(Options{}) {
		refusing[name] = true
	}
	seen := map[string]bool{}
	for _, name := range exportedFacadesTakingOptions(t) {
		seen[name] = true
		if refusing[name] == noRefusalFacades[name] {
			t.Errorf("%s takes an Options: list it in exactly one of the refusal tables and the no-refusal list", name)
		}
	}
	for name := range refusing {
		if !seen[name] {
			t.Errorf("%s is in a refusal table but is no exported facade that takes an Options", name)
		}
	}
	for name := range noRefusalFacades {
		if !seen[name] {
			t.Errorf("%s is in the no-refusal list but is no exported facade that takes an Options", name)
		}
	}
}

func TestHolderJudge_UsesTheResolvedRoot(t *testing.T) {
	quietStrayPlane(t)
	opts, dir := offShapeOptions(t)
	claim := filepath.Join(opts.InboxDir, "processing", "cycle-9", "task-2.json")
	_, err := ReleaseStaleClaims(opts, "stale")
	if !errors.Is(err, errNoProjectRoot) {
		t.Fatalf("err = %v, want errNoProjectRoot: the judge must not infer Dir(Dir(InboxDir)) = %s", err, dir)
	}
	if _, serr := os.Stat(claim); serr != nil {
		t.Errorf("the claim moved: %v", serr)
	}
}
