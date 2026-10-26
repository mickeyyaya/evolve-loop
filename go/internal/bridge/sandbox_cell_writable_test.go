package bridge

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchcred"
)

type cellLayout struct {
	name                                string
	root, worktree, workspace, artifact string
	store                               func() (dispatchcred.Store, error)
}

func contractTempDir(t *testing.T, parent, pattern string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, pattern)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	return real
}

func mkdirs(t *testing.T, dirs ...string) {
	t.Helper()
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func integrationCellLayout(t *testing.T) cellLayout {
	t.Helper()
	root := contractTempDir(t, "", "itcell")
	l := cellLayout{name: "integration itConfig layout, package default store", root: root, worktree: root,
		workspace: filepath.Join(root, "ws"), artifact: filepath.Join(root, "artifact"), store: defaultDispatchStore}
	mkdirs(t, l.workspace)
	return l
}

func contractCellLayout(t *testing.T) cellLayout {
	t.Helper()
	root, home := contractTempDir(t, "", "sbc-root"), contractTempDir(t, "", "sbc-home")
	l := cellLayout{name: "contract layout, cell home store", root: root, worktree: contractTempDir(t, "", "sbc-wt"),
		workspace: filepath.Join(root, ".evolve", "runs", "cycle-7")}
	l.artifact = filepath.Join(l.workspace, "build-report.md")
	s := dispatchcred.Store{Dir: filepath.Join(home, ".evolve", "dispatch")}
	l.store = func() (dispatchcred.Store, error) { return s, nil }
	mkdirs(t, l.workspace)
	return l
}

func incidentCellLayout(t *testing.T) cellLayout {
	t.Helper()
	tmp := contractTempDir(t, "", "incident")
	root := filepath.Join(tmp, "TestRealTmux")
	l := cellLayout{name: "incident layout: the store sits next to the cell", root: root, worktree: root,
		workspace: filepath.Join(root, "ws"), artifact: filepath.Join(root, "artifact")}
	s := dispatchcred.Store{Dir: filepath.Join(tmp, "evolve-bridge-test-dispatch")}
	l.store = func() (dispatchcred.Store, error) { return s, nil }
	mkdirs(t, l.workspace)
	return l
}

func wrapCellProfile(t *testing.T, deps Deps, goos string, l cellLayout) string {
	t.Helper()
	deps.Env, deps.ExecStage, deps.DispatchStore = map[string]string{"EVOLVE_SANDBOX": "on"}, "enforce", l.store
	prefix, ok := defaultSandboxWrapWithProbe(deps, fakeProbe(goos, true))(SandboxWrapRequest{
		Phase: "itest", CLI: "claude-tmux", Worktree: l.worktree, Workspace: l.workspace, RepoRoot: l.root,
	})
	if !ok {
		t.Fatalf("%s on %s: the wrap was refused", l.name, goos)
	}
	if goos == "linux" {
		return strings.Join(prefix, "\x00")
	}
	sbpl, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatal(err)
	}
	return string(sbpl)
}

var sbplWriteRule = regexp.MustCompile(`^\((allow|deny) file-write\* \((subpath|literal) "([^"]*)"\)\)$`)

func sbplCellWritable(profile, path string) bool {
	writable := false
	for _, line := range strings.Split(profile, "\n") {
		m := sbplWriteRule.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil && (m[2] == "subpath" && withinDir(m[3], path) || m[3] == path) {
			writable = m[1] == "allow"
		}
	}
	return writable
}

var bwrapMountArity = map[string]int{"--bind": 2, "--bind-try": 2, "--ro-bind": 2, "--ro-bind-try": 2, "--tmpfs": 1, "--remount-ro": 1}

func bwrapCellWritable(argv []string, path string) bool {
	writable := false
	for i := 0; i < len(argv); i++ {
		n, ok := bwrapMountArity[argv[i]]
		if !ok || i+n >= len(argv) {
			continue
		}
		if withinDir(argv[i+n], path) {
			writable = argv[i] == "--bind" || argv[i] == "--bind-try"
		}
		i += n
	}
	return writable
}

func cellWritable(goos, profile, path string) bool {
	if goos == "linux" {
		return bwrapCellWritable(strings.Split(profile, "\x00"), path)
	}
	return sbplCellWritable(profile, path)
}

func TestSandboxProfile_NoCredentialDenyOrMaskCoversTheCellOfATestLayout(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux"} {
		for _, layout := range []func(*testing.T) cellLayout{integrationCellLayout, contractCellLayout} {
			l := layout(t)
			profile := wrapCellProfile(t, Deps{}, goos, l)
			for _, path := range []string{l.workspace, l.artifact, l.worktree} {
				if !cellWritable(goos, profile, path) {
					t.Errorf("%s on %s: the generated profile denies the write of %s, so the sandboxed agent cannot write its artifact:\n%s", l.name, goos, path, profile)
				}
			}
		}
	}
}

func TestSandboxProfile_TheCellCheckSeesTheIncidentDeny(t *testing.T) {
	t.Parallel()
	for _, goos := range []string{"darwin", "linux"} {
		l := incidentCellLayout(t)
		profile := wrapCellProfile(t, Deps{}, goos, l)
		for _, path := range []string{l.workspace, l.artifact, l.worktree} {
			if cellWritable(goos, profile, path) {
				t.Errorf("%s on %s: the check must report the write of %s as denied by the store parent deny:\n%s", l.name, goos, path, profile)
			}
		}
	}
}

func TestCellWritable_TheLastRuleThatCoversThePathDecides(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, goos, profile, path string
		want                      bool
	}{
		{"sbpl allow after deny", "darwin", "(deny file-write* (subpath \"/r\"))\n(allow file-write* (subpath \"/r/w\"))", "/r/w/a", true},
		{"sbpl deny after allow", "darwin", "(allow file-write* (subpath \"/r/w\"))\n(deny file-write* (subpath \"/r\"))", "/r/w/a", false},
		{"sbpl literal deny of the path", "darwin", "(allow file-write* (subpath \"/r\"))\n(deny file-write* (literal \"/r/a\"))", "/r/a", false},
		{"sbpl literal deny of an ancestor", "darwin", "(allow file-write* (subpath \"/r\"))\n(deny file-write* (literal \"/r\"))", "/r/a", true},
		{"sbpl no rule", "darwin", "(allow file-write* (subpath \"/other\"))", "/r/a", false},
		{"bwrap bind after ro-bind", "linux", "--ro-bind\x00/r\x00/r\x00--bind\x00/r/w\x00/r/w", "/r/w/a", true},
		{"bwrap ro-bind after bind", "linux", "--bind\x00/r/w\x00/r/w\x00--ro-bind\x00/r\x00/r", "/r/w/a", false},
		{"bwrap tmpfs over the path", "linux", "--bind\x00/r\x00/r\x00--tmpfs\x00/r/w", "/r/w/a", false},
		{"bwrap ro-bind of /dev/null on another path", "linux", "--bind\x00/r\x00/r\x00--ro-bind\x00/dev/null\x00/r/b", "/r/a", true},
		{"bwrap truncated mount", "linux", "--bind\x00/r\x00/r\x00--ro-bind\x00/r", "/r/a", true},
	}
	for _, tc := range cases {
		if got := cellWritable(tc.goos, tc.profile, tc.path); got != tc.want {
			t.Errorf("%s: cellWritable(%s) = %v, want %v", tc.name, tc.path, got, tc.want)
		}
	}
}
