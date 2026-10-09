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
	if n := strings.Count(out, "/dev/ttys"); n != 2 {
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
