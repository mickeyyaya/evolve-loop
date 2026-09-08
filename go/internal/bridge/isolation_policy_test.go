package bridge

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLaunchProfileDenialsReachSandbox(t *testing.T) {
	fx := newFixture(t, "codex", "")
	root, wt := t.TempDir(), t.TempDir()
	body := `{"name":"fixture","model":"auto","sandbox":{"enabled":true,"allow_network":true,"deny_subpaths":[".evolve/evals"],"deny_read_subpaths":["docs/private"]}}`
	if err := os.WriteFile(fx.profile, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "done"}
	wrap := defaultSandboxWrapWithProbe(Deps{}, fakeProbe("darwin", true))
	rc, output := runWithSandbox(t, fr, wrap, fx.args("codex", "--project-root="+root, "--worktree="+wt))
	if rc != ExitOK {
		t.Fatalf("launch rc=%d: %s", rc, output)
	}
	if len(fr.calls) == 0 {
		t.Fatal("no child invocation")
	}
	call := fr.calls[len(fr.calls)-1]
	if call.name != "sandbox-exec" {
		t.Fatalf("child not sandboxed: %+v", call)
	}
	profile, err := os.ReadFile(call.args[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, base := range []string{root, wt} {
		for _, rule := range []string{
			`(deny file-write* (subpath "` + filepath.Join(base, ".evolve/evals") + `"))`,
			`(deny file-read* (subpath "` + filepath.Join(base, "docs/private") + `"))`,
		} {
			if !strings.Contains(string(profile), rule) {
				t.Errorf("actual launch missing %s", rule)
			}
		}
		if strings.Contains(string(profile), `(deny file-read* (subpath "`+filepath.Join(base, ".evolve/evals")) {
			t.Error("eval sources must remain readable")
		}
	}
}

func TestSandboxDenialsCoverSymlinksAndMissingDescendants(t *testing.T) {
	root, wt, target := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Symlink(target, filepath.Join(wt, "protected")); err != nil {
		t.Fatal(err)
	}
	paths, err := resolveSandboxDenials([]string{"protected/future"}, root, wt, false)
	if err != nil {
		t.Fatal(err)
	}
	realTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{filepath.Join(root, "protected/future"), filepath.Join(wt, "protected/future"), filepath.Join(realTarget, "future")} {
		if !slices.Contains(paths, want) {
			t.Errorf("missing deny alias %s from %v", want, paths)
		}
	}
	for _, bad := range []string{"../escape", "docs/*", "", "{unknown}"} {
		if _, err := resolveSandboxDenials([]string{bad}, root, wt, false); err == nil {
			t.Errorf("accepted unsupported deny %q", bad)
		}
	}
	if _, err := resolveSandboxDenials([]string{filepath.Join(root, "absolute"), "relative"}, "", "", false); err == nil {
		t.Fatal("absolute entry must not hide unresolved relative denial")
	}
	if got, err := resolveSandboxDenials([]string{".git"}, root, root, true); err != nil || len(got) == 0 {
		t.Fatalf("same-root .git denial lost: %v %v", got, err)
	}
}

func TestLinuxSandboxDenialsAndUnsupportedTargets(t *testing.T) {
	root := t.TempDir()
	protected := filepath.Join(root, "evals")
	private := filepath.Join(root, "private-fixture")
	for _, p := range []string{protected, private} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	wrap := defaultSandboxWrapWithProbe(Deps{}, fakeProbe("linux", true))
	req := SandboxWrapRequest{RepoRoot: root, Worktree: root, DenyPaths: []string{protected}, DenyReadPaths: []string{private}}
	args, ok := wrap(req)
	if !ok {
		t.Fatal("valid directory restrictions must be supported")
	}
	if len(args) == 0 || args[0] != "bwrap" {
		t.Errorf("Linux prefix must launch bwrap, got %v", args)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--ro-bind "+protected+" "+protected) || !strings.Contains(joined, "--tmpfs "+private) || !strings.Contains(joined, "--remount-ro "+private) {
		t.Fatalf("missing restrictions: %v", args)
	}
	req.DenyPaths = []string{filepath.Join(root, "absent")}
	if _, ok := wrap(req); ok {
		t.Fatal("missing target must not silently discard restriction")
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	req.DenyPaths = nil
	req.DenyReadPaths = []string{file}
	if _, ok := wrap(req); ok {
		t.Fatal("unsupported read-denied file must not claim directory confinement")
	}
}

func TestMandatorySandboxDoesNotTrustNestedEnvironment(t *testing.T) {
	deps := Deps{Env: map[string]string{"CLAUDECODE": "1"}}
	if !sandboxRequiredButUnavailable(deps, &Config{RequireSandbox: true}, false) {
		t.Fatal("nested marker is not proof of required filesystem restrictions")
	}
	deps.Env[envSandboxMode] = "off"
	if sandboxRequiredButUnavailable(deps, &Config{RequireSandbox: true}, false) {
		t.Fatal("explicit operator opt-out must remain supported")
	}
}

func TestLinuxLinkedGitMetadataIsolationFailsClosed(t *testing.T) {
	root, worktree, metadata := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+metadata+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metadata, "HEAD"), []byte("ref: refs/heads/fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	writes, denies, err := sandboxGitWritePaths(worktree, "linux")
	if err == nil || len(writes) != 0 || len(denies) != 0 {
		t.Fatalf("Linux must reject coarse Git metadata policy: %v %v %v", writes, denies, err)
	}
	var log strings.Builder
	deps := Deps{Stderr: &log}
	wrap := defaultSandboxWrapWithProbe(deps, fakeProbe("linux", true))
	_, wrapped := wrap(SandboxWrapRequest{Phase: "builder", RepoRoot: root, Worktree: worktree, Workspace: t.TempDir()})
	if wrapped || !strings.Contains(log.String(), "Git metadata isolation unsupported") {
		t.Fatalf("missing explicit capability failure: wrapped=%v %s", wrapped, log.String())
	}
	if !sandboxRequiredButUnavailable(deps, &Config{RequireSandbox: true}, wrapped) {
		t.Fatal("mandatory launch must fail closed")
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(worktree); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(previous); err != nil {
			t.Error(err)
		}
	}()
	if writes, denies, err := sandboxGitWritePaths("", "linux"); err != nil || len(writes) != 0 || len(denies) != 0 {
		t.Fatalf("empty worktree must not adopt process cwd metadata: %v %v %v", writes, denies, err)
	}
}
