package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/core/evidence"
)

// git_evidence_test.go — the ADR-0027 git-evidence completion contract:
// completion = a NEW commit in baseline..HEAD whose trailer verifies the phase
// + challenge token. Driven through the gitCmd seam (no real repo).

// fakeGit scripts the three git calls the detector makes: rev-parse HEAD
// (advances once per call, last repeats), rev-list <base>..HEAD (commits in the
// range, keyed by the base SHA, newest-first), and log -1 --format=%B <sha>.
type fakeGit struct {
	heads   []string
	hi      int
	revlist map[string][]string
	msgs    map[string]string
}

func (f *fakeGit) cmd(_ context.Context, args ...string) (string, error) {
	switch args[0] {
	case "rev-parse":
		v := f.heads[f.hi]
		if f.hi < len(f.heads)-1 {
			f.hi++
		}
		return v, nil
	case "rev-list":
		base, _, _ := strings.Cut(args[len(args)-1], "..")
		return strings.Join(f.revlist[base], "\n"), nil
	case "log":
		return f.msgs[args[len(args)-1]], nil
	}
	return "", nil
}

func pollUntil(d *gitEvidenceDetector, maxPolls int) (bool, completionEvidence) {
	for i := 0; i < maxPolls; i++ {
		if ready, ev, _, _ := d.poll(context.Background()); ready {
			return true, ev
		}
	}
	return false, completionEvidence{}
}

func TestGitEvidence_ReadyOnVerifiedCommit(t *testing.T) {
	good := "subject\n" + evidence.Trailer{Phase: "scout", Challenge: "tok", Cycle: 5}.Build()
	g := &fakeGit{
		heads:   []string{"sha0", "sha1"},
		revlist: map[string][]string{"sha0": {"sha1"}},
		msgs:    map[string]string{"sha1": good},
	}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok", gitCmd: g.cmd}
	ready, ev := pollUntil(d, 5)
	if !ready {
		t.Fatal("git-evidence should complete on a verified evidence commit")
	}
	if ev.CommitSHA != "sha1" {
		t.Errorf("evidence SHA = %q, want sha1", ev.CommitSHA)
	}
}

func TestGitEvidence_FindsEvidenceWhenNotTip(t *testing.T) {
	// HIGH-fix regression: two commits land between polls — the evidence commit
	// (sha1) then a stray commit (sha2, now HEAD). Inspecting only HEAD would
	// re-baseline PAST sha1 and hang forever; the range walk must find sha1.
	good := "build done\n" + evidence.Trailer{Phase: "build", Challenge: "tok"}.Build()
	g := &fakeGit{
		heads:   []string{"sha0", "sha2"},
		revlist: map[string][]string{"sha0": {"sha2", "sha1"}}, // newest-first
		msgs:    map[string]string{"sha2": "chore: stray commit\n", "sha1": good},
	}
	d := &gitEvidenceDetector{phase: "build", expectedTok: "tok", gitCmd: g.cmd}
	ready, ev := pollUntil(d, 5)
	if !ready || ev.CommitSHA != "sha1" {
		t.Fatalf("range walk must find the non-tip evidence commit sha1; got ready=%v sha=%q", ready, ev.CommitSHA)
	}
}

func TestGitEvidence_NotReadyWithoutTrailer(t *testing.T) {
	g := &fakeGit{
		heads:   []string{"sha0", "sha1"},
		revlist: map[string][]string{"sha0": {"sha1"}},
		msgs:    map[string]string{"sha1": "just a normal commit\n"},
	}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok", gitCmd: g.cmd}
	if ready, _ := pollUntil(d, 5); ready {
		t.Fatal("a HEAD advance without a verifying trailer must not complete")
	}
}

func TestGitEvidence_NotReadyWrongToken(t *testing.T) {
	msg := "x\n" + evidence.Trailer{Phase: "scout", Challenge: "WRONG"}.Build()
	g := &fakeGit{
		heads:   []string{"sha0", "sha1"},
		revlist: map[string][]string{"sha0": {"sha1"}},
		msgs:    map[string]string{"sha1": msg},
	}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok", gitCmd: g.cmd}
	if ready, _ := pollUntil(d, 5); ready {
		t.Fatal("a commit with the wrong challenge token must not complete")
	}
}

func TestGitEvidence_EmptyTokenNeverVerifies(t *testing.T) {
	// Fail-closed: an empty expected token (challenge-token.txt missing) must
	// never complete, even for a commit carrying the right phase.
	msg := "x\n" + evidence.Trailer{Phase: "scout", Challenge: "anything"}.Build()
	g := &fakeGit{
		heads:   []string{"sha0", "sha1"},
		revlist: map[string][]string{"sha0": {"sha1"}},
		msgs:    map[string]string{"sha1": msg},
	}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "", gitCmd: g.cmd}
	if ready, _ := pollUntil(d, 5); ready {
		t.Fatal("an empty expected token must never verify (fail-closed)")
	}
}

func TestGitEvidence_NotReadyWhenHeadStatic(t *testing.T) {
	g := &fakeGit{heads: []string{"sha0"}, revlist: nil, msgs: nil}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok", gitCmd: g.cmd}
	if ready, _ := pollUntil(d, 5); ready {
		t.Fatal("a static HEAD must not complete")
	}
}

func TestGitEvidence_GitErrorKeepsWaiting(t *testing.T) {
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok",
		gitCmd: func(context.Context, ...string) (string, error) { return "", context.DeadlineExceeded }}
	if ready, _ := pollUntil(d, 3); ready {
		t.Fatal("a git error must not complete")
	}
}

func TestNewGitEvidenceDetector_ReadsToken(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte("tok-xyz\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d := newGitEvidenceDetector(&Config{Workspace: ws, Worktree: ws, Agent: "scout"}, Deps{}.withDefaults())
	if d.expectedTok != "tok-xyz" || d.phase != "scout" {
		t.Fatalf("constructor read tok=%q phase=%q, want tok-xyz/scout", d.expectedTok, d.phase)
	}
}

func TestNewGitEvidenceDetector_MissingTokenWarns(t *testing.T) {
	var stderr bytes.Buffer
	deps := Deps{Stderr: &stderr}.withDefaults()
	deps.Stderr = &stderr // withDefaults keeps a non-nil Stderr; pin our buffer
	d := newGitEvidenceDetector(&Config{Workspace: t.TempDir(), Agent: "scout"}, deps)
	if d.expectedTok != "" {
		t.Errorf("expectedTok = %q, want empty when token file absent", d.expectedTok)
	}
	if !strings.Contains(stderr.String(), "challenge-token.txt missing") {
		t.Errorf("missing-token should warn to stderr; got %q", stderr.String())
	}
}

func TestNewCompletionDetector_GitMode(t *testing.T) {
	cfg := &Config{Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "scout"}
	if _, ok := newCompletionDetector("git", cfg, Deps{}.withDefaults(), tmuxLaunch{}).(*gitEvidenceDetector); !ok {
		t.Error("mode \"git\" should build a *gitEvidenceDetector")
	}
}

func TestShortSHA(t *testing.T) {
	if got := shortSHA("0123456789abcdef"); got != "01234567" {
		t.Errorf("shortSHA = %q, want 01234567", got)
	}
	if got := shortSHA("abc"); got != "abc" {
		t.Errorf("shortSHA short input = %q, want abc", got)
	}
}
