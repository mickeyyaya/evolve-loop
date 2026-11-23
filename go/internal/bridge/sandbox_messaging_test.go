package bridge

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/channelstore"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

type wrapFixture struct{ home, wt, ws, root string }

func newWrapFixture(t *testing.T) wrapFixture {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	return wrapFixture{home: home, wt: realTempDir(t), ws: realTempDir(t), root: realTempDir(t)}
}

func (f wrapFixture) wrap(t *testing.T, id *MessagingIdentity) ([]string, WrapOutcome) {
	t.Helper()
	return defaultSandboxWrapWithProbe(Deps{GuaranteeStage: "off", ExecStage: "off", SandboxMode: "on"}, fakeProbe("darwin", true))(SandboxWrapRequest{
		Phase: "build", CLI: "agy-tmux", Worktree: f.wt, Workspace: f.ws, RepoRoot: f.root, Tag: "t1", Messaging: id,
	})
}

func (f wrapFixture) normalized(t *testing.T, sbpl string) string {
	t.Helper()
	b, err := os.ReadFile(sbpl)
	if err != nil {
		t.Fatal(err)
	}
	var keep []string
	for _, l := range strings.Split(string(b), "\n") {
		if !strings.Contains(l, "tmux-") && !strings.Contains(l, f.home) {
			keep = append(keep, l)
		}
	}
	return strings.NewReplacer(f.home, "<HOME>", f.wt, "<WORKTREE>", f.ws, "<WORKSPACE>", f.root, "<ROOT>").Replace(strings.Join(keep, "\n"))
}

func readGolden(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/sandbox-profile-no-messaging.golden")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSandboxWrap_NoMessagingIdentity_WritesTheSameProfileAsBefore(t *testing.T) {
	t.Parallel()
	f := newWrapFixture(t)
	prefix, out := f.wrap(t, nil)
	if !out.Applied {
		t.Fatalf("the wrap was refused: %+v", out)
	}
	if got := f.normalized(t, prefix[2]); got != readGolden(t) {
		t.Fatalf("a dispatch with no messaging identity must keep the profile of main byte for byte:\n%s", got)
	}
}

func TestSandboxWrap_MessagingIdentityAppendsTheRulesBuiltFromTheLayoutHome(t *testing.T) {
	t.Parallel()
	f := newWrapFixture(t)
	prefix, out := f.wrap(t, &MessagingIdentity{Cycle: 7, Dispatch: "r/7/build/p1"})
	if !out.Applied {
		t.Fatalf("the wrap was refused: %+v", out)
	}
	layout := outbox.Layout{Root: "<ROOT>", Cycle: 7}
	chRoot := paths.EventChannelsRoot(filepath.Join("<ROOT>", ".evolve"))
	want := readGolden(t) +
		`(deny file-read* (subpath "` + layout.TreeDir() + `"))` + "\n" +
		`(allow file-read* (subpath "` + layout.DispatchDir("r/7/build/p1") + `"))` + "\n" +
		`(deny file-write* (subpath "` + layout.TreeDir() + `"))` + "\n" +
		`(allow file-write* (subpath "` + layout.OutDir("r/7/build/p1") + `"))` + "\n" +
		`(deny file-write* (prefix "` + chRoot + `/` + channelstore.ChannelPrefix + `"))` + "\n" +
		`(deny file-read* (prefix "` + chRoot + `/` + channelstore.ChannelPrefix + `"))` + "\n"
	if got := f.normalized(t, prefix[2]); got != want {
		t.Fatalf("the messaging rules must come from outbox.Layout and channelstore, last in the profile:\n%s", got)
	}
}

func TestSandboxWrap_MessagingIdentityCreatesTheStoreChannelDirsBeforeLaunch(t *testing.T) {
	t.Parallel()
	f := newWrapFixture(t)
	if _, out := f.wrap(t, &MessagingIdentity{Cycle: 7, Dispatch: "d"}); !out.Applied {
		t.Fatalf("the wrap was refused: %+v", out)
	}
	for _, dir := range append([]string{filepath.Join(f.root, filepath.FromSlash(outbox.TreeRel))}, storeDirs(f.root, 7)...) {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("bwrap masks the tree and the store with tmpfs, so %s must exist before launch: %v", dir, err)
		}
	}
}

func TestSandboxWrap_RefusesAMessagingIdentityWithNoDispatch(t *testing.T) {
	t.Parallel()
	f := newWrapFixture(t)
	prefix, out := f.wrap(t, &MessagingIdentity{Cycle: 7})
	if out.Applied || out.Cause != CauseProfile || prefix != nil {
		t.Fatalf("a messaging identity with no dispatch must fail closed with a profile refusal: %+v", out)
	}
}

func storeDirs(root string, cycle int) []string {
	var dirs []string
	for _, name := range channelstore.Channels(cycle) {
		dirs = append(dirs, filepath.Join(paths.EventChannelsRoot(filepath.Join(root, ".evolve")), name))
	}
	return dirs
}

func TestMessagingScope_NilIdentityGivesNoScope(t *testing.T) {
	t.Parallel()
	scope, err := messagingScope(nil, "/nonexistent")
	if scope != nil || err != nil {
		t.Fatalf("no identity, no scope: %v %v", scope, err)
	}
}

func TestMessagingScope_RefusesARootThatIsNotAbsolute(t *testing.T) {
	t.Parallel()
	for _, root := range []string{"", "relative/root", "."} {
		if _, err := messagingScope(&MessagingIdentity{Cycle: 1, Dispatch: "d"}, root); err == nil {
			t.Fatalf("root %q must be refused", root)
		}
	}
}

func TestMessagingScope_RefusesATreeThatIsASymlinkOut(t *testing.T) {
	t.Parallel()
	root, elsewhere := realTempDir(t), realTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(root, ".evolve", "msg")); err != nil {
		t.Fatal(err)
	}
	if _, err := messagingScope(&MessagingIdentity{Cycle: 1, Dispatch: "d"}, root); !errors.Is(err, errOutsideScope) {
		t.Fatalf("a symlinked tree would make the profile rules miss: %v", err)
	}
}

func TestMessagingScope_RefusesAStoreThatIsASymlinkOut(t *testing.T) {
	t.Parallel()
	root, elsewhere := realTempDir(t), realTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(root, ".evolve", "events")); err != nil {
		t.Fatal(err)
	}
	if _, err := messagingScope(&MessagingIdentity{Cycle: 1, Dispatch: "d"}, root); !errors.Is(err, errOutsideScope) {
		t.Fatalf("a symlinked store would make the profile rules miss: %v", err)
	}
}

func TestEnsureMessagingDirs_FailsWhenTheTreeDirCannotBeCreated(t *testing.T) {
	t.Parallel()
	root := realTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "msg"), []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	scope, err := messagingScope(&MessagingIdentity{Cycle: 1, Dispatch: "d"}, root)
	if err != nil {
		t.Fatalf("the scope is pure and names the paths: %v", err)
	}
	if err := ensureMessagingDirs(scope); err == nil {
		t.Fatal("a tree dir that cannot exist must fail closed, not run with a mask that cannot bind")
	}
}

func TestEnsureMessagingDirs_NilScopeCreatesNothing(t *testing.T) {
	t.Parallel()
	if err := ensureMessagingDirs(nil); err != nil {
		t.Fatal(err)
	}
}

func TestMessagingScope_IsPureAndCreatesNoDirectory(t *testing.T) {
	t.Parallel()
	root := realTempDir(t)
	if _, err := messagingScope(&MessagingIdentity{Cycle: 7, Dispatch: "d"}, root); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(root); err != nil || len(entries) != 0 {
		t.Fatalf("a query must not touch the host: %v %v", entries, err)
	}
}

func TestMessagingScope_StoreDirsCoverEveryExistingMessageChannelAndThisCyclesTwo(t *testing.T) {
	t.Parallel()
	root := realTempDir(t)
	ch := paths.EventChannelsRoot(paths.EvolveDirOf(root))
	for _, dir := range []string{"msg.6", "msg.dead.6", "msg.7", "other.6"} {
		if err := os.MkdirAll(filepath.Join(ch, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(ch, "msg.6.lock"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	scope, err := messagingScope(&MessagingIdentity{Cycle: 7, Dispatch: "d"}, root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"msg.6", "msg.7", "msg.dead.6", "msg.dead.7"}
	var got []string
	for _, d := range scope.StoreDirs {
		got = append(got, filepath.Base(d))
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("every msg.* channel directory, once, and no lock file or other channel: got %q want %q", got, want)
	}
}

func TestMessagingScope_BwrapMasksTheChannelOfAnotherCycle(t *testing.T) {
	t.Parallel()
	root := realTempDir(t)
	ch := paths.EventChannelsRoot(paths.EvolveDirOf(root))
	if err := os.MkdirAll(filepath.Join(ch, "msg.6"), 0o755); err != nil {
		t.Fatal(err)
	}
	scope, err := messagingScope(&MessagingIdentity{Cycle: 7, Dispatch: "d"}, root)
	if err != nil {
		t.Fatal(err)
	}
	argv := sandbox.BwrapPrefix(sandbox.Config{Messaging: scope})
	if at := slices.Index(argv, filepath.Join(ch, "msg.6")); at < 1 || argv[at-1] != "--tmpfs" {
		t.Fatalf("the SBPL prefix denies msg.6, so bwrap must mask it too: %q", argv)
	}
}

func TestMessagingScope_FailsWhenTheStoreRootIsNotADirectory(t *testing.T) {
	t.Parallel()
	root := realTempDir(t)
	ch := paths.EventChannelsRoot(paths.EvolveDirOf(root))
	if err := os.MkdirAll(filepath.Dir(ch), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ch, []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := messagingScope(&MessagingIdentity{Cycle: 7, Dispatch: "d"}, root); err == nil {
		t.Fatal("a store root that cannot be listed must fail closed")
	}
}

func TestSandboxPrefixForLaunch_ForwardsTheMessagingIdentityToTheWrap(t *testing.T) {
	t.Parallel()
	var got SandboxWrapRequest
	deps := Deps{SandboxWrap: func(req SandboxWrapRequest) ([]string, WrapOutcome) { got = req; return nil, WrapOutcome{} }}
	id := &MessagingIdentity{Cycle: 7, Dispatch: "d"}
	sandboxPrefixForLaunch(deps, &Config{Worktree: "/wt", Messaging: id}, "")
	if got.Messaging != id {
		t.Fatalf("the launch must hand the identity to the wrap: %+v", got.Messaging)
	}
}

func TestCanonicalWithin_ReturnsTheCanonicalJoinOfAnExistingOrMissingPath(t *testing.T) {
	t.Parallel()
	base := realTempDir(t)
	got, err := canonicalWithin(base, "a/b")
	if err != nil || got != filepath.Join(base, "a", "b") {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCanonicalWithin_RefusesARelativeThatResolvesOutsideItsBase(t *testing.T) {
	t.Parallel()
	base, elsewhere := realTempDir(t), realTempDir(t)
	if err := os.Symlink(elsewhere, filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalWithin(base, "link"); !errors.Is(err, errOutsideScope) {
		t.Fatalf("a symlink out of the base must be refused: %v", err)
	}
}

func TestCanonicalWithin_RefusesABaseThatCannotBeResolved(t *testing.T) {
	t.Parallel()
	file := filepath.Join(realTempDir(t), "f")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalWithin(filepath.Join(file, "x"), "y"); err == nil {
		t.Fatal("a base under a regular file cannot be resolved")
	}
	if _, err := canonicalWithin(file, "y/z"); err == nil {
		t.Fatal("a path under a regular file cannot be resolved")
	}
}
