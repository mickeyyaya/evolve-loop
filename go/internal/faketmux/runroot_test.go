package faketmux

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func startPane(t *testing.T, root, name string) int {
	t.Helper()
	var errOut bytes.Buffer
	if code := Run(root, []string{"-L", "sock", "new-session", "-d", "-s", name, "sleep 30"}, &bytes.Buffer{}, &errOut); code != 0 {
		t.Fatalf("new-session in %s: %s", root, errOut.String())
	}
	t.Cleanup(func() { _ = Reap(root) })
	rec, err := session{dir: filepath.Join(root, "sock", "sessions", name)}.pane()
	if err != nil {
		t.Fatal(err)
	}
	return rec.pid
}

func writeOwner(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ownerFile), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestClaim_ReapsThePanesOfARunWhoseOwnerIsGoneAndKeepsALiveOwnersRun(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "base")
	stale := filepath.Join(base, runPrefix+"dead")
	writeOwner(t, stale, "2 0.000000")
	stalePane := startPane(t, stale, "leak")
	ownerless := filepath.Join(base, runPrefix+"noowner")
	if err := os.MkdirAll(ownerless, 0o700); err != nil {
		t.Fatal(err)
	}
	claiming := filepath.Join(base, ".claim-busy")
	if err := os.MkdirAll(claiming, 0o700); err != nil {
		t.Fatal(err)
	}
	first, err := Claim(base)
	if err != nil {
		t.Fatal(err)
	}
	livePane := startPane(t, first, "live")

	second, err := Claim(base)

	if err != nil || second == first || !strings.HasPrefix(filepath.Base(second), runPrefix) {
		t.Fatalf("Claim = %q, %v; want a fresh run root beside %q", second, err, first)
	}
	awaitGone(t, stalePane)
	for _, gone := range []string{stale, ownerless} {
		if _, err := os.Stat(gone); !os.IsNotExist(err) {
			t.Errorf("the stale run %s must be removed: %v", gone, err)
		}
	}
	if !(session{dir: filepath.Join(first, "sock", "sessions", "live")}).alive() {
		t.Errorf("the pane %d of a live owner's run must survive the sweep", livePane)
	}
	if _, err := os.Stat(claiming); err != nil {
		t.Errorf("a claim in progress must be left alone: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(second, ownerFile))
	if err != nil || !strings.HasPrefix(string(body), strconv.Itoa(os.Getpid())+" ") {
		t.Errorf("the owner record = %q, %v; want this process", body, err)
	}
}

func TestSession_APidThatStartedAgainIsNotTheRecordedPaneAndIsNeverSignalled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pid := startPane(t, root, "reused")
	sess := session{name: "reused", dir: filepath.Join(root, "sock", "sessions", "reused")}
	if err := os.WriteFile(sess.path("pane"), []byte(strconv.Itoa(pid)+" 24 0.000000"), 0o600); err != nil {
		t.Fatal(err)
	}

	if sess.alive() {
		t.Fatal("a pid whose start time differs from the record is not the pane")
	}
	if err := sess.kill(); err != nil {
		t.Fatal(err)
	}
	if !startedAsNow(pid) {
		t.Errorf("kill signalled pid %d, whose start time does not match the record", pid)
	}
	if err := signalGroup(pid); err != nil {
		t.Fatal(err)
	}
}

func startedAsNow(pid int) bool {
	begun, err := processStart(pid)
	return err == nil && startedAs(pid, begun)
}

func TestReap_AMissingRootIsReapedAndAFileRootIsRefused(t *testing.T) {
	t.Parallel()
	if err := Reap(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Errorf("Reap of a missing root = %v, want nil", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wantErr(t, Reap(file), "not a directory")
	wantErr(t, sweep(file), "not a directory")
	fresh := filepath.Join(t.TempDir(), "fresh", "base")
	if root, err := Claim(fresh); err != nil || filepath.Dir(root) != fresh {
		t.Fatalf("Claim of a new base = %q, %v; want a run root inside it", root, err)
	}
	shared := t.TempDir()
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Claim(shared)
	wantErr(t, err, "only this user can use")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	_, err = Claim(link)
	wantErr(t, err, "only this user can use")
	_, err = Claim(file)
	wantErr(t, err, "not a directory")
}

func TestSeams_ClaimAndStartFailLoudlyWhenTheStartTimeOrTheClaimFails(t *testing.T) {
	restore := setSeams(t)
	base := filepath.Join(t.TempDir(), "base")
	stale := filepath.Join(base, runPrefix+"dead")
	writeOwner(t, stale, "2 0.000000")
	startPane(t, stale, "held")

	signalGroup = func(int) error { return errInjected }
	_, err := Claim(base)
	wantErr(t, err, "injected")
	restore()

	mkdirTemp = func(string, string) (string, error) { return "", errInjected }
	_, err = Claim(base)
	wantErr(t, err, "injected")
	restore()

	processStart = func(int) (string, error) { return "", errInjected }
	_, err = Claim(base)
	wantErr(t, err, "read the start of the owner: injected")
	wantErr(t, session{name: "p", dir: t.TempDir()}.start([]string{"sleep", "30"}, "", 24), "record the pane: injected")
	restore()
}
