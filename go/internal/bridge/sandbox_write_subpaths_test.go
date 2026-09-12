package bridge

// sandbox_write_subpaths_test.go — profile.sandbox.write_subpaths is honored.
//
// Every phase profile declares its sandbox write surface in
// sandbox.write_subpaths, and the persona renders its instructions from the
// same profile. The bash-era dispatcher (subagent-run.sh, v8.21–v8.23) granted
// those paths; the Go port kept the SBPL glob-widening they fed
// (adapters/sandbox: "Globs widen to parent dir (bash:520)") but dropped the
// consumer: launch.go projects deny_subpaths and deny_read_subpaths from the
// profile and never reads write_subpaths, so the wrapper invents its own
// allow set (worktree + workspace + /tmp) plus a Go-literal copy of ONE
// profile's declaration (retrospective's lesson dir).
//
// The consequence is the P0 in inbox triage-sandbox-denies-its-own-claim-write:
// triage declares ".evolve/inbox/processing", its persona instructs
// `evolve inbox-mover claim` as the atomic hand-off, the profile never grants
// it, mkdir fails, and cycle 1623 ran twelve phases and shipped 922 lines
// against an EMPTY commitment. The same gap is latent for every other
// declared write no literal happens to cover (doc-sync's docs/ and
// knowledge-base/, orchestrator's ledger, scout's eval materialization).
//
// These tests read the REAL profiles, so a declaration that drifts from its
// grant fails here rather than in a live cycle.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// realProfilesDir is the checked-in profile set. Tests that read it need
// -count=1 (the Go test cache does not see reads that escape the module).
func realProfilesDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", ".evolve", "profiles"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "triage.json")); err != nil {
		t.Skipf("real profiles not reachable from the test cwd: %v", err)
	}
	return dir
}

// renderSBPL runs the real darwin wrapper for req and returns the profile text.
func renderSBPL(t *testing.T, req SandboxWrapRequest) string {
	t.Helper()
	wrap := defaultSandboxWrapWithProbe(Deps{}, fakeProbe("darwin", true))
	prefix, ok := wrap(req)
	if !ok {
		t.Fatalf("wrapper refused the launch for phase %q (request %+v)", req.Phase, req)
	}
	raw, err := os.ReadFile(prefix[2])
	if err != nil {
		t.Fatalf("read SBPL: %v", err)
	}
	return string(raw)
}

func allowWriteLine(path string) string {
	return "(allow file-write* (subpath " + strconv.Quote(path) + "))"
}

// TestDefaultSandboxWrap_GrantsTriagesDeclaredInboxClaimDir is the P0 pin: the
// triage profile's own write_subpaths declaration must appear as a write
// grant in the rendered profile. Before the fix the request carries the
// declaration and the wrapper ignores it — the exact shape of cycle 1623's
// sandbox-triage.sb (deny on the plane, allow-back set without the inbox).
func TestDefaultSandboxWrap_GrantsTriagesDeclaredInboxClaimDir(t *testing.T) {
	prof, err := LoadProfile(filepath.Join(realProfilesDir(t), "triage.json"))
	if err != nil {
		t.Fatal(err)
	}
	if prof.Sandbox == nil || len(prof.Sandbox.WriteSubpaths) == 0 {
		t.Fatal("precondition: the triage profile must declare sandbox.write_subpaths")
	}
	root := t.TempDir()
	canonicalRoot, err := canonicalSandboxPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox", "processing"), 0o755); err != nil {
		t.Fatal(err)
	}

	sbpl := renderSBPL(t, SandboxWrapRequest{
		Phase: "triage", RepoRoot: root, Worktree: t.TempDir(), Workspace: t.TempDir(),
		WriteSubpaths: prof.Sandbox.WriteSubpaths,
	})

	want := allowWriteLine(filepath.Join(canonicalRoot, ".evolve", "inbox", "processing"))
	if !strings.Contains(sbpl, want) {
		t.Fatalf("the triage profile declares write_subpaths %v but the rendered sandbox never grants the inbox claim dir — "+
			"`evolve inbox-mover claim` fails at mkdir and the cycle runs its whole spine against an empty commitment (cycle 1623).\nwant line: %s\nprofile:\n%s",
			prof.Sandbox.WriteSubpaths, want, sbpl)
	}
}

// realSandboxProfiles returns every checked-in profile that enables the
// sandbox, keyed by file name. Profiles without a sandbox block never reach
// the wrapper's grant path and are out of scope by construction.
func realSandboxProfiles(t *testing.T) map[string]Profile {
	t.Helper()
	dir := realProfilesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]Profile{}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		// Decide membership from the data, not a skip list: the directory
		// also holds non-phase documents (tool-policy.json) that the strict
		// profile loader rightly rejects. Only a file that enables the
		// sandbox can reach the wrapper's grant path.
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var peek struct {
			Sandbox *ProfileSandbox `json:"sandbox"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil || peek.Sandbox == nil || !peek.Sandbox.Enabled {
			continue
		}
		prof, err := LoadProfile(path)
		if err != nil {
			t.Fatalf("load %s: %v", e.Name(), err)
		}
		out[e.Name()] = prof
	}
	if len(out) < 50 {
		t.Fatalf("only %d sandbox-enabled profiles found under %s — the real profile set is not being read", len(out), dir)
	}
	return out
}

// TestLaunchArgs_ProjectsProfileWriteSubpathsToTheWrapper is the wiring proof
// for the first hop: the REAL launch path (Engine.LaunchArgs → profile load →
// Config → sandboxPrefixForLaunch) carries the profile's declared
// write_subpaths, verbatim, to the sandbox wrapper. It uses the launch
// fixture's own profile shape with a sandbox block, because the point is the
// projection, not any one profile's content — the real profiles' declarations
// are proven at the second hop by TestDefaultSandboxWrap_RendersEveryDeclaredWriteSubpath.
// Before the fix the launch path projected deny_subpaths and dropped this
// field on the floor; this test fails for that regression by assertion.
func TestLaunchArgs_ProjectsProfileWriteSubpathsToTheWrapper(t *testing.T) {
	declared := []string{".evolve/inbox/processing", "{worktree_path}/tests"}
	fx := newFixture(t, "claude-p", "")
	body := `{
  "name": "test-claude-p",
  "model": "haiku",
  "allowed_tools": ["Read", "Write"],
  "auto_respond": {"destructive_ops": false, "timeout_s": 60},
  "prompt_overrides": [],
  "sandbox": {"enabled": true, "read_only_repo": true, "write_subpaths": ["` + strings.Join(declared, `", "`) + `"]}
}
`
	if err := os.WriteFile(fx.profile, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var captured []SandboxWrapRequest
	capture := func(req SandboxWrapRequest) ([]string, bool) {
		captured = append(captured, req)
		// The profile REQUIRES confinement, so declining would (correctly)
		// fail the launch closed; report a wrap as available.
		return []string{"sandbox-exec", "-f", "/tmp/x.sb"}, true
	}
	fr := &fakeRunner{writeArtifactPath: fx.artifact, writeArtifactBody: "ok"}
	code, stderr := runWithSandbox(t, fr, capture, fx.args("claude-p", "--worktree="+t.TempDir()))
	if code != ExitOK {
		t.Fatalf("launch exit=%d: %s", code, stderr)
	}
	if len(captured) != 1 {
		t.Fatalf("sandbox wrapper consulted %d times, want 1", len(captured))
	}
	if got := captured[0].WriteSubpaths; !slices.Equal(got, declared) {
		t.Fatalf("the launch path handed the wrapper write_subpaths %v, but the profile declares %v — a declared write that is not granted is the cycle-1623 defect class", got, declared)
	}
}

// TestDefaultSandboxWrap_RendersEveryDeclaredWriteSubpath is the wiring proof
// for the second hop, and the durable regression gate the P0 asked for: for
// every sandbox-enabled profile, every declared write_subpaths entry — as the
// production resolver resolves it and the generator widens it — appears as a
// write grant in the rendered SBPL. A profile whose declaration the generated
// profile does not honor fails here, naming the profile and the path.
func TestDefaultSandboxWrap_RendersEveryDeclaredWriteSubpath(t *testing.T) {
	for name, prof := range realSandboxProfiles(t) {
		t.Run(strings.TrimSuffix(name, ".json"), func(t *testing.T) {
			root, worktree := t.TempDir(), t.TempDir()
			sbpl := renderSBPL(t, SandboxWrapRequest{
				Phase: strings.TrimSuffix(name, ".json"), RepoRoot: root, Worktree: worktree, Workspace: t.TempDir(),
				WriteSubpaths: prof.Sandbox.WriteSubpaths,
			})
			grants, err := resolveSandboxWriteGrants(prof.Sandbox.WriteSubpaths, root, worktree)
			if err != nil {
				t.Fatalf("resolver refused the profile's own declarations: %v", err)
			}
			if len(grants) == 0 {
				t.Fatalf("profile declares %v but nothing resolved — every profile declares at least its run dir", prof.Sandbox.WriteSubpaths)
			}
			// The adapter's contract is "literal absolute paths only": the resolver
			// owns glob widening, so what it returns must appear VERBATIM, and no
			// rendered subpath may still carry a glob (SBPL would match nothing;
			// bwrap would fail the bind).
			for _, m := range subpathArgRe.FindAllStringSubmatch(sbpl, -1) {
				if strings.ContainsAny(m[1], "*?[") {
					t.Errorf("a glob reached the sandbox generator, which cannot interpret it: subpath %q", m[1])
				}
			}
			for _, g := range grants {
				if !strings.Contains(sbpl, allowWriteLine(g)) {
					t.Errorf("declared write %q (resolved %q) is absent from the rendered sandbox — the persona may instruct a write the kernel will refuse\nprofile:\n%s", prof.Sandbox.WriteSubpaths, g, sbpl)
				}
			}
		})
	}
}

// TestResolveSandboxWriteGrants pins the resolver's contract entry by entry:
// each base kind, the climb refusal, the retarget refusal, the skipped
// template-without-worktree, and — the projection the adapters no longer
// own — glob widening to the longest glob-free ancestor, terminal or not.
func TestResolveSandboxWriteGrants(t *testing.T) {
	root, wt := t.TempDir(), t.TempDir()
	canonicalRoot, err := canonicalSandboxPath(root)
	if err != nil {
		t.Fatal(err)
	}
	canonicalWT, err := canonicalSandboxPath(wt)
	if err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(t.TempDir(), "elsewhere")
	if err := os.MkdirAll(abs, 0o755); err != nil {
		t.Fatal(err)
	}
	canonicalAbs, err := canonicalSandboxPath(abs)
	if err != nil {
		t.Fatal(err)
	}
	join := func(base string, rel ...string) string { return filepath.Join(append([]string{base}, rel...)...) }
	for _, tc := range []struct {
		name     string
		declared []string
		worktree string
		want     []string
		wantErr  string
	}{
		{"repo-relative", []string{".evolve/inbox/processing"}, wt, []string{join(canonicalRoot, ".evolve", "inbox", "processing")}, ""},
		{"worktree template alone grants the worktree", []string{"{worktree_path}"}, wt, []string{canonicalWT}, ""},
		{"worktree template with subpath", []string{"{worktree_path}/tests"}, wt, []string{join(canonicalWT, "tests")}, ""},
		{"worktree template without a worktree is skipped", []string{"{worktree_path}/tests"}, "", nil, ""},
		{"absolute is granted as declared, cleaned", []string{abs + "/./"}, wt, []string{canonicalAbs}, ""},
		{"terminal glob widens to its parent", []string{".evolve/runs/cycle-*"}, wt, []string{join(canonicalRoot, ".evolve", "runs")}, ""},
		{"non-terminal glob widens to the glob-free ancestor", []string{".evolve/runs/cycle-*/learn"}, wt, []string{join(canonicalRoot, ".evolve", "runs")}, ""},
		{"glob in a name widens to its parent", []string{".evolve/cycle-state*"}, wt, []string{join(canonicalRoot, ".evolve")}, ""},
		{"worktree glob widens inside the worktree", []string{"{worktree_path}/scripts/*-test.sh"}, wt, []string{join(canonicalWT, "scripts")}, ""},
		{"duplicates collapse", []string{".evolve/runs/cycle-*", ".evolve/runs/cycle-*/workers"}, wt, []string{join(canonicalRoot, ".evolve", "runs")}, ""},
		{"climb above the root is refused", []string{"../x"}, wt, nil, "escapes its base"},
		{"climb above the worktree is refused", []string{"{worktree_path}/../x"}, wt, nil, "escapes its base"},
		{"empty entry is refused", []string{""}, wt, nil, "must be a path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveSandboxWriteGrants(tc.declared, root, tc.worktree)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v (grants %v)", tc.wantErr, err, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("grants = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestResolveSandboxWriteGrants_RefusesASymlinkRetargetBelowTheBase is the
// generalized form of retro_lessons_test.go: any declared grant whose path
// below its base is a symlink is refused, whatever profile declared it.
func TestResolveSandboxWriteGrants_RefusesASymlinkRetargetBelowTheBase(t *testing.T) {
	root := t.TempDir()
	target := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(root, ".evolve", "inbox", "processing")); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveSandboxWriteGrants([]string{".evolve/inbox/processing"}, root, t.TempDir()); err == nil {
		t.Fatalf("a retargeted grant was accepted: %v — honoring it would hand the phase write access to %s", got, target)
	}
}

// subpathArgRe captures the quoted path of every SBPL (subpath "...") form.
var subpathArgRe = regexp.MustCompile(`\(subpath "([^"]*)"\)`)
