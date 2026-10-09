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
		`(allow file-write-data (literal "/dev/tty") (literal "/dev/ttys042"))`,
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

const ownPtySlaveGrant = `(allow file-write-data file-ioctl (require-all (regex #"^/dev/ttys[0-9]+$") (extension "com.apple.sandbox.pty")))`

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
			if n := strings.Count(out, "regex"); n != 1 {
				t.Fatalf("the only pattern grant must be the slave grant that the pty extension scopes; got %d patterns:\n%s", n, out)
			}
			if n := strings.Count(out, "/dev/tty"); n != tc.ttyNames {
				t.Fatalf("only the scoped slave grant and the pane's own terminal rules may name a tty, want %d names, got %d:\n%s", tc.ttyNames, n, out)
			}
		})
	}
}
