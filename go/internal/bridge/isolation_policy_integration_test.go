//go:build integration

package bridge

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/plane"
)

// The /tmp repository is intentional: ordinary t.TempDir on macOS uses
// /var/folders and misses the broad /tmp write-grant regression.
func TestSandboxPreservesLinkedWorktreeGitIndex(t *testing.T) {
	probe := sandbox.Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED native sandbox: %+v", probe)
	}
	if probe.OS != "darwin" {
		t.Skip("unsupported: linked-worktree metadata requires precise file grants; mandatory Linux launch fails closed")
	}
	root, err := os.MkdirTemp("/tmp", "sandbox-git-fixture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	wt, ws := filepath.Join(t.TempDir(), "worktree"), t.TempDir()
	git := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid"}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v %s", err, out)
		}
	}
	git(root, "init", "--quiet")
	git(root, "commit", "--allow-empty", "-m", "fixture")
	git(root, "worktree", "add", "-b", "cycle-fixture", wt)
	info, err := plane.Classify(wt)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"hooks", "info"} {
		if err := os.MkdirAll(filepath.Join(info.GitDir, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(wt, "sample"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	denies, err := resolveSandboxDenials([]string{".git"}, root, wt, true)
	if err != nil {
		t.Fatal(err)
	}
	wrap := defaultSandboxWrapWithProbe(Deps{}, func() sandbox.ProbeResult { return probe })
	prefix, ok := wrap(SandboxWrapRequest{Phase: "builder", RepoRoot: root, Worktree: wt, Workspace: ws, DenyPaths: denies, AllowNetwork: true})
	if !ok {
		t.Fatal("supported fixture policy declined")
	}
	writeMain := append(append([]string{}, prefix[1:]...), "/bin/sh", "-c", `printf forbidden > "$1"`, "fixture", filepath.Join(root, "outside-worktree"))
	if out, err := exec.Command(prefix[0], writeMain...).CombinedOutput(); err == nil {
		t.Fatalf("main checkout write escaped confinement: %s", out)
	}
	args := append(prefix[1:], "git", "-C", wt, "add", "sample")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if out, err := exec.CommandContext(ctx, prefix[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("legitimate worktree staging denied: %v %s", err, out)
	}
	git(wt, "ls-files", "--error-unmatch", "sample")
	commitArgs := append(append([]string{}, prefix[1:]...), "git", "-C", wt, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-m", "fixture checkpoint")
	if out, err := exec.CommandContext(ctx, prefix[0], commitArgs...).CombinedOutput(); err != nil {
		t.Fatalf("legitimate worktree checkpoint denied: %v %s", err, out)
	}
	writeConfig := append(append([]string{}, prefix[1:]...), "/bin/sh", "-c", `printf forbidden > "$1"`, "fixture", filepath.Join(root, ".git/config"))
	if out, err := exec.CommandContext(ctx, prefix[0], writeConfig...).CombinedOutput(); err == nil {
		t.Fatalf("git grants exposed shared config: %s", out)
	}
	if probe.OS == "darwin" {
		otherRef := append(append([]string{}, prefix[1:]...), "git", "-C", wt, "update-ref", "refs/heads/sibling", "HEAD")
		if out, err := exec.CommandContext(ctx, prefix[0], otherRef...).CombinedOutput(); err == nil {
			t.Fatalf("macOS exact-ref policy exposed sibling ref: %s", out)
		}
	}
	// Open for append without writing bytes: this checks write permission on
	// harmless temporary routing metadata without changing its contents.
	for _, control := range []string{
		filepath.Join(info.GitDir, "commondir"), filepath.Join(info.GitDir, "gitdir"),
		filepath.Join(info.GitDir, "config.worktree"), filepath.Join(info.GitDir, "config"),
		filepath.Join(info.GitDir, "hooks", "fixture"), filepath.Join(info.GitDir, "info", "attributes"),
		filepath.Join(root, ".git", "objects", "info", "alternates"),
	} {
		openControl := append(append([]string{}, prefix[1:]...), "/bin/sh", "-c", `: >> "$1"`, "fixture", control)
		if out, err := exec.CommandContext(ctx, prefix[0], openControl...).CombinedOutput(); err == nil {
			t.Errorf("control metadata writable: %s (%s)", control, out)
		}
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("sandbox denial probes did not complete: %v (timeout is not confinement evidence)", err)
	}
}

func TestLaunchProfilePolicyWithFixtureChild(t *testing.T) {
	probe := sandbox.Probe()
	if !probe.Available || !probe.CapabilityChecked || !probe.Capable {
		t.Skipf("UNVERIFIED: native sandbox cannot apply: %+v", probe)
	}
	fx := newFixture(t, "codex", "")
	root := t.TempDir()
	for _, p := range []string{"private-fixture", "evals"} {
		if err := os.Mkdir(filepath.Join(root, p), 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"private-fixture/sample", "evals/sample"} {
		if err := os.WriteFile(filepath.Join(root, p), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(root, "private-fixture"), filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	stub := filepath.Join(root, "fixture-cli")
	// A real harmless child; it accepts the driver's arguments but uses only
	// the generated fixture tree. Exit 0 proves both positive and negative cases.
	script := "#!/bin/sh\nset -eu\n" +
		`cat evals/sample >/dev/null
printf allowed > allowed
if cat private-fixture/sample >/dev/null 2>&1; then exit 41; fi
if cat alias/sample >/dev/null 2>&1; then exit 42; fi
if (printf forbidden > evals/sample) 2>/dev/null; then exit 43; fi
printf done > ` + shellQuotePOSIX(fx.artifact) + "\n"
	if err := os.WriteFile(stub, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fx.profile, []byte(`{"name":"fixture","sandbox":{"enabled":true,"allow_network":true,"deny_subpaths":["evals"],"deny_read_subpaths":["private-fixture"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	var log strings.Builder
	deps := Deps{Env: map[string]string{"BRIDGE_TESTING": "1", "BRIDGE_CODEX_BINARY": stub, "PATH": "/var/run/codex.system/bootstrap/usr/bin:" + os.Getenv("PATH")}, Stderr: &log, LookupEnv: mapLookup(nil)}
	if !sandbox.DetectNested(depEnvGetter(deps)) {
		t.Fatal("fixture must retain the Codex bootstrap session hint")
	}
	deps.SandboxWrap = defaultSandboxWrapWithProbe(deps, func() sandbox.ProbeResult { return probe })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rc := newTestEngine(deps).LaunchArgs(ctx, fx.args("codex", "--project-root="+root, "--worktree="+root), nil, io.Discard, &log)
	if rc != ExitOK {
		t.Fatalf("fixture child failed (not containment evidence): rc=%d %s", rc, log.String())
	}
	if b, err := os.ReadFile(filepath.Join(root, "evals/sample")); err != nil || string(b) != "fixture" {
		t.Fatalf("eval source changed: %q %v", b, err)
	}
}
