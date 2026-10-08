package proctree

import "testing"

func TestSharedHelper_MatchesTheClassOfHelpersOthersUse(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		p      Process
		shared bool
	}{
		{"a tmux server", Process{Comm: "/opt/homebrew/bin/tmux", Args: []string{"tmux", "-L", "evolve-bridge", "new-session"}}, true},
		{"the Claude Code daemon", Process{Comm: "/Users/x/.local/bin/claude", Args: []string{"claude", "daemon", "run"}}, true},
		{"a Claude Code pty host", Process{Comm: "claude", Args: []string{"claude", "bg-pty-host"}}, true},
		{"a Claude Code spare", Process{Comm: "claude", Args: []string{"claude", "bg-spare"}}, true},
		{"Google Chrome", Process{Comm: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome", Args: []string{"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"}}, true},
		{"a Chrome helper", Process{Comm: "Google Chrome Helper (Renderer)"}, true},
		{"Chromium for a browser MCP", Process{Comm: "/x/chromium", Args: []string{"/x/chromium", "--headless"}}, true},
		{"an MCP server", Process{Comm: "node", Args: []string{"node", "server.js"}}, false},
		{"a claude session that runs a daemon word as a prompt", Process{Comm: "claude", Args: []string{"claude", "-p", "daemon"}}, false},
		{"agy", Process{Comm: "/opt/homebrew/bin/agy", Args: []string{"agy"}}, false},
	}
	for _, tc := range cases {
		if got := SharedHelper(tc.p); got != tc.shared {
			t.Errorf("%s: shared=%v, want %v", tc.name, got, tc.shared)
		}
	}
}

func TestOwnedByDispatch_ATaggedSharedHelperNeedsTheRecordedTree(t *testing.T) {
	t.Parallel()
	id := "01R/1835/build/p4242n1"
	tag := map[string]string{"EVOLVE_DISPATCH_ID": id}
	daemon := Process{Pid: 50, Ppid: 1, Started: t0, Comm: "claude", Args: []string{"claude", "daemon", "run"}, Env: tag}
	chrome := Process{Pid: 51, Ppid: 1, Started: t0, Comm: "Google Chrome", Env: tag}
	server := Process{Pid: 52, Ppid: 1, Started: t0, Comm: "tmux", Args: []string{"tmux", "new-session"}, Env: tag}
	mcp := Process{Pid: 53, Ppid: 1, Started: t0, Comm: "node", Args: []string{"node", "mcp.js"}, Env: tag}
	zsh := Process{Pid: 54, Ppid: 1, Started: t0, Comm: "/bin/zsh"}
	table := []Process{daemon, chrome, server, mcp, zsh}

	tree := []Identity{daemon.Identity(), zsh.Identity()}
	none := Select(table, OwnedByDispatch(id, nil, nil))
	inTree := Select(table, OwnedByDispatch(id, tree, tree))

	if got := pids(none); len(got) != 1 || got[0] != 53 {
		t.Errorf("no tree: selected %v, want only the MCP server 53", got)
	}
	if got := pids(inTree); len(got) != 3 || got[0] != 50 || got[1] != 53 || got[2] != 54 {
		t.Errorf("tree {50, 54}: selected %v, want [50 53 54]", got)
	}
}

func TestOwnedByDispatch_ARecordedSharedHelperMustStillBeALiveDescendantOfThePane(t *testing.T) {
	t.Parallel()
	id := "01R/1835/build/p4242n1"
	daemon := Process{Pid: 50, Ppid: 1, Started: t0, Comm: "claude", Args: []string{"claude", "daemon", "run"}}
	mcp := Process{Pid: 53, Ppid: 1, Started: t0, Comm: "node"}
	tree := []Identity{daemon.Identity(), mcp.Identity()}

	detached := Select([]Process{daemon, mcp}, OwnedByDispatch(id, tree, nil))
	attached := Select([]Process{daemon, mcp}, OwnedByDispatch(id, tree, []Identity{daemon.Identity()}))

	if got := pids(detached); len(got) != 1 || got[0] != 53 {
		t.Errorf("detached daemon: selected %v, want only 53: a shared helper that left the pane serves other sessions", got)
	}
	if got := pids(attached); len(got) != 2 {
		t.Errorf("attached daemon: selected %v, want [50 53]", got)
	}
}
