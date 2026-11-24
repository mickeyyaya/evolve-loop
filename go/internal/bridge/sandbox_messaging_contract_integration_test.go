//go:build integration && darwin && !notmux

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

const (
	ownDispatch     = "r/7/build/p1"
	siblingDispatch = "r/7/audit/p2"
	foreignDispatch = "r/8/build/p9"
)

type messagingHost struct {
	contractHost
	own, sibling, foreign outbox.Layout
	store                 string
}

func newMessagingHost(t *testing.T, h contractHost) messagingHost {
	t.Helper()
	h.messaging = &MessagingIdentity{Cycle: 7, Dispatch: ownDispatch}
	m := messagingHost{contractHost: h, own: outbox.Layout{Root: h.root, Cycle: 7}, foreign: outbox.Layout{Root: h.root, Cycle: 8}}
	m.sibling = m.own
	for _, e := range []struct {
		l outbox.Layout
		d string
	}{{m.own, ownDispatch}, {m.own, siblingDispatch}, {m.foreign, foreignDispatch}} {
		for _, dir := range []string{e.l.OutDir(e.d), e.l.BoxDir(e.d)} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.WriteFile(filepath.Join(e.l.BoxDir(e.d), "mailbox.json"), []byte("mailbox of "+e.d), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	chRoot := paths.EventChannelsRoot(filepath.Join(h.root, ".evolve"))
	for _, name := range channelstore.Channels(7) {
		if err := os.MkdirAll(filepath.Join(chRoot, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	m.store = filepath.Join(chRoot, channelstore.Channels(7)[0])
	if err := os.WriteFile(filepath.Join(m.store, "000001.log"), []byte("every message of the cycle"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.prefix = builderPrefix(t, requireNativeSandbox(t), m.contractHost, "enforce")
	return m
}

func (m messagingHost) rows() []contractRow {
	box := func(l outbox.Layout, d string) string { return filepath.Join(l.BoxDir(d), "mailbox.json") }
	write := func(path string) func(contractHost) string {
		return func(contractHost) string { return "echo x > " + path }
	}
	read := func(path string) func(contractHost) string {
		return func(contractHost) string { return "cat " + path }
	}
	return []contractRow{
		{action: "write a draft in its own outbox", want: contractAllow,
			script: func(contractHost) string {
				f := filepath.Join(m.own.OutDir(ownDispatch), "d.json")
				return "echo ok > " + f + ".tmp && mv " + f + ".tmp " + f + " && cat " + f
			}},
		{action: "read its own mailbox", want: contractAllow, script: read(box(m.own, ownDispatch))},
		{action: "write its own mailbox", want: contractDeny, script: write(filepath.Join(m.own.BoxDir(ownDispatch), "forged"))},
		{action: "write the outbox of another dispatch of its cycle", want: contractDeny, script: write(filepath.Join(m.own.OutDir(siblingDispatch), "forged.json"))},
		{action: "write the outbox of a dispatch of another cycle", want: contractDeny, script: write(filepath.Join(m.foreign.OutDir(foreignDispatch), "forged.json"))},
		{action: "create a dispatch directory in the tree", want: contractDeny, script: func(contractHost) string { return "mkdir " + filepath.Join(m.own.TreeDir(), "7", "forged") }},
		{action: "write the message store", want: contractDeny, script: write(filepath.Join(m.store, "000002.log"))},
		{action: "append to the message store", want: contractDeny, script: func(contractHost) string { return "echo x >> " + filepath.Join(m.store, "000001.log") }},
		{action: "read the message store", want: contractDeny, script: read(filepath.Join(m.store, "000001.log"))},
		{action: "read the mailbox of another dispatch of its cycle", want: contractDeny, script: read(box(m.own, siblingDispatch))},
		{action: "read the mailbox of a dispatch of another cycle", want: contractDeny, script: read(box(m.foreign, foreignDispatch))},
		{action: "list the mailbox directory of another dispatch", want: contractDeny, script: func(contractHost) string { return "ls " + m.own.BoxDir(siblingDispatch) }},
		{action: "read another mailbox through ..", want: contractDeny,
			script: read(filepath.Join(m.own.BoxDir(ownDispatch), "..", "..", filepath.Base(m.own.DispatchDir(siblingDispatch)), "box", "mailbox.json"))},
		{action: "read another mailbox through a symlink made in its own outbox", want: contractDeny,
			script: func(contractHost) string {
				link := filepath.Join(m.own.OutDir(ownDispatch), "link")
				return "ln -s " + m.own.BoxDir(siblingDispatch) + " " + link + " && cat " + link + "/mailbox.json"
			}},
		{action: "read another mailbox through a symlink to the tree", want: contractDeny,
			script: func(h contractHost) string {
				link := filepath.Join(h.scratch, "treelink")
				return "ln -s " + m.own.TreeDir() + " " + link + " && cat " + link + "/7/" + filepath.Base(m.own.DispatchDir(siblingDispatch)) + "/box/mailbox.json"
			}},
		{action: "read another mailbox with the path in upper case", want: contractDeny, script: read(strings.ToUpper(box(m.own, siblingDispatch)))},
		{action: "read another mailbox with doubled slashes and a dot segment", want: contractDeny,
			script: read(strings.Replace(box(m.own, siblingDispatch), "/box/", "//./box//", 1))},
		{action: "list the tree with a trailing slash", want: contractDeny, script: func(contractHost) string { return "ls " + m.own.TreeDir() + "/" }},
		{action: "hard-link another mailbox out of the tree", want: contractDeny,
			script: func(h contractHost) string {
				return "ln " + box(m.own, siblingDispatch) + " " + filepath.Join(h.scratch, "hard")
			}},
		{action: "write in its own worktree", want: contractAllow, script: func(h contractHost) string { return "echo ok > " + h.worktree + "/own && cat " + h.worktree + "/own" }},
	}
}

func runMessagingContract(t *testing.T, h contractHost) {
	t.Helper()
	m := newMessagingHost(t, h)
	t.Run("the rows of the messaging identity", func(t *testing.T) {
		for _, row := range m.rows() {
			t.Run(row.action, func(t *testing.T) {
				out, err := m.agent(t, row)
				if failure := contractFailure(row.want, out, err); failure != nil {
					t.Fatal(failure)
				}
				runServerSurvives(t, m.contractHost)
			})
		}
		if _, err := os.Stat(filepath.Join(m.scratch, "hard")); !os.IsNotExist(err) {
			t.Fatalf("the denied hard link left a file on the host: %v", err)
		}
	})
	t.Run("TestSandbox_ARealProcessCannotWriteAnotherOutboxOfItsCycleOrOfAnotherCycle", func(t *testing.T) {
		for name, outDir := range map[string]string{
			"another outbox of its cycle": m.own.OutDir(siblingDispatch),
			"an outbox of another cycle":  m.foreign.OutDir(foreignDispatch),
		} {
			forged := filepath.Join(outDir, "forged.json")
			out, err := m.agent(t, contractRow{script: func(contractHost) string { return "echo x > " + forged }})
			if failure := contractFailure(contractDeny, out, err); failure != nil {
				t.Fatalf("%s: %v", name, failure)
			}
			if _, err := os.Stat(forged); !os.IsNotExist(err) {
				t.Fatalf("%s: the denied write left a file on the host: %v", name, err)
			}
		}
		own := filepath.Join(m.own.OutDir(ownDispatch), "own.json")
		if out, err := m.agent(t, contractRow{script: func(contractHost) string { return "echo ok > " + own }}); err != nil {
			t.Fatalf("the own outbox must stay writable: %v %q", err, out)
		}
	})
	t.Run("without an identity the tree is readable as today", func(t *testing.T) {
		mailbox := filepath.Join(m.own.BoxDir(siblingDispatch), "mailbox.json")
		out, err := h.agent(t, contractRow{script: func(contractHost) string { return "cat " + mailbox }})
		if err != nil || !strings.Contains(out, "mailbox of "+siblingDispatch) {
			t.Fatalf("a dispatch with no identity keeps today's profile, which reads the repo: %v %q", err, out)
		}
	})
}
