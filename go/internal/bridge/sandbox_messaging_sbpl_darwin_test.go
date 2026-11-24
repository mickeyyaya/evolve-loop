//go:build darwin

package bridge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/channelstore"
	"github.com/mickeyyaya/evolve-loop/go/internal/agentmsg/outbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

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
