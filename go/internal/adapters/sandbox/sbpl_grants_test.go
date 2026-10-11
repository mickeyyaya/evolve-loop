package sandbox

import (
	"strings"
	"testing"
)

func TestGenerateSBPL_TerminalGrantNamesOnlyThePaneDevice(t *testing.T) {
	cfg := canonicalConfig()
	cfg.TerminalPath = "/dev/ttys042"
	out := GenerateSBPL(cfg)
	mustContain(t, out,
		`(allow file-read* file-write-data (literal "/dev/tty") (literal "/dev/ttys042"))`,
		`(allow file-ioctl (require-all (require-any (literal "/dev/tty") (literal "/dev/ttys042")) (require-any (ioctl-command TIOCGETA) (ioctl-command TIOCSETA) (ioctl-command TIOCSETAW) (ioctl-command TIOCSETAF) (ioctl-command TIOCGWINSZ))))`,
	)
	if n := strings.Count(out, "/dev/ttys042"); n != 2 {
		t.Fatalf("the terminal grant must name the pane device exactly in its two rules, got %d:\n%s", n, out)
	}
}

func TestGenerateSBPL_ScratchGrantAboveTheRepoPrecedesTheReadOnlyDeny(t *testing.T) {
	cfg := canonicalConfig()
	cfg.RepoRoot = "/tmp/lane/repo"
	cfg.ReadOnlyRepo = true
	cfg.WritePaths = []string{"/tmp/lane"}
	out := GenerateSBPL(cfg)
	grant := `(allow file-write* (subpath "/tmp/lane"))`
	deny := `(deny file-write* (subpath "/tmp/lane/repo"))`
	if n := strings.Count(out, grant); n != 1 {
		t.Fatalf("the parent grant must be emitted once, before the deny, never again after it; got %d:\n%s", n, out)
	}
	if g, d := strings.Index(out, grant), strings.Index(out, deny); d < 0 || g > d {
		t.Fatalf("a grant above the repo must precede the read-only deny, or it reopens the repo (grant=%d deny=%d):\n%s", g, d, out)
	}
}

const ownPtySlaveGrant = `(allow file-read* file-write-data file-ioctl (require-all (regex #"^/dev/ttys[0-9]+$") (extension "com.apple.sandbox.pty")))`

func TestGenerateSBPL_GrantsOnlyThePseudoTerminalsTheAgentOpens(t *testing.T) {
	for _, tc := range []struct {
		name, pane string
		ttyNames   int
	}{
		{"no pane", "", 1},
		{"pane", "/dev/ttys042", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := canonicalConfig()
			cfg.TerminalPath = tc.pane
			out := GenerateSBPL(cfg)
			for _, rule := range []string{"(allow pseudo-tty)", `(allow file-read* file-write* file-ioctl (literal "/dev/ptmx"))`, ownPtySlaveGrant} {
				if n := strings.Count(out, rule+"\n"); n != 1 {
					t.Fatalf("tmux needs %s exactly once to forkpty a window; got %d:\n%s", rule, n, out)
				}
			}
			if n := strings.Count(out, "(regex") - strings.Count(out, "(require-not (regex"); n != 1 {
				t.Fatalf("the only pattern grant must be the slave grant that the pty extension scopes; got %d patterns:\n%s", n, out)
			}
			if n := strings.Count(out, "/dev/tty"); n != tc.ttyNames {
				t.Fatalf("only the scoped slave grant and the pane's own terminal rules may name a tty, want %d names, got %d:\n%s", tc.ttyNames, n, out)
			}
		})
	}
}

const wantDevReadGrant = `(allow file-read* (require-all (subpath "/dev") (require-not (regex #"^/dev/(tty|pty)[^/]+$"))))`

func TestGenerateSBPL_DevReadGrantLeavesOutEveryTerminalButTheAgentsOwn(t *testing.T) {
	for _, pane := range []string{"", "/dev/ttys042"} {
		cfg := canonicalConfig()
		cfg.TerminalPath = pane
		out := GenerateSBPL(cfg)
		if n := strings.Count(out, wantDevReadGrant+"\n"); n != 1 {
			t.Fatalf("pane %q: the /dev read grant must leave out the tty and pty devices, want it once, got %d:\n%s", pane, n, out)
		}
		if strings.Contains(out, `(subpath "/dev"))`) {
			t.Fatalf("pane %q: a bare /dev subpath grant lets the agent read another pane's tty:\n%s", pane, out)
		}
	}
}

func TestGenerateSBPL_ReadDenialsFollowEveryReadGrantAndSkipEmptyEntries(t *testing.T) {
	cfg := canonicalConfig()
	cfg.DenyReadPaths = []string{"", "/repo/private"}
	out := GenerateSBPL(cfg)
	deny := `(deny file-read* (subpath "/repo/private"))`
	if n := strings.Count(out, deny); n != 1 {
		t.Fatalf("want the read denial once, got %d:\n%s", n, out)
	}
	if strings.Contains(out, `(deny file-read* (subpath ""))`) {
		t.Fatalf("an empty read denial must emit no rule:\n%s", out)
	}
	if d, g := strings.Index(out, deny), strings.LastIndex(out, "(allow file-read*"); d < g {
		t.Fatalf("the read denial must follow every read grant, or a later grant reopens it (deny=%d last grant=%d)", d, g)
	}
}
