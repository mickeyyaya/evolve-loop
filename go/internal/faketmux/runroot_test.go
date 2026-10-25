package faketmux

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func startPane(t *testing.T, root, name string) int {
	t.Helper()
	var errOut bytes.Buffer
	if code := Run(root, []string{"-L", "sock", "new-session", "-d", "-s", name, "sleep 30"}, &bytes.Buffer{}, &errOut); code != 0 {
		t.Fatalf("new-session in %s: %s", root, errOut.String())
	}
	sess := session{name: name, dir: filepath.Join(root, "sock", "sessions", name)}
	t.Cleanup(func() { _ = sess.kill() })
	rec, err := sess.pane()
	if err != nil {
		t.Fatal(err)
	}
	return rec.pid
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func newBase(t *testing.T) (base, token string) {
	t.Helper()
	base = filepath.Join(t.TempDir(), "base")
	token, err := baseToken(base)
	if err != nil {
		t.Fatal(err)
	}
	return base, token
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("%s must still exist: %v", path, err)
	}
}

func TestClaimIn_ReapsOnlyTheMarkedRunsWhoseOwnerIsGone(t *testing.T) {
	t.Parallel()
	base, token := newBase(t)
	stale := filepath.Join(base, runPrefix+"dead")
	writeFile(t, filepath.Join(stale, ownerFile), "2 0.000000 "+token)
	stalePane := startPane(t, stale, "leak")
	foreign := filepath.Join(base, runPrefix+"foreign", "data")
	writeFile(t, foreign, "keep")
	forged := filepath.Join(base, runPrefix+"forged")
	writeFile(t, filepath.Join(forged, ownerFile), "2 0.000000 "+strings.Repeat("0", 32))
	claiming := filepath.Join(base, claimTemp+"busy")
	writeFile(t, filepath.Join(claiming, "x"), "")
	first, err := claimIn(base)
	if err != nil {
		t.Fatal(err)
	}
	startPane(t, first, "live")

	second, err := claimIn(base)

	if err != nil || second == first || filepath.Dir(second) != base {
		t.Fatalf("claimIn = %q, %v; want a fresh run root beside %q", second, err, first)
	}
	awaitGone(t, stalePane)
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the stale marked run must be removed: %v", err)
	}
	for _, kept := range []string{foreign, filepath.Join(forged, ownerFile), claiming} {
		mustExist(t, kept)
	}
	if !(session{dir: filepath.Join(first, "sock", "sessions", "live")}).alive() {
		t.Error("the pane of a live owner's run must survive the sweep")
	}
	body, err := os.ReadFile(filepath.Join(second, ownerFile))
	if err != nil || !strings.HasPrefix(string(body), strconv.Itoa(os.Getpid())+" ") || !strings.HasSuffix(string(body), " "+token) {
		t.Errorf("the owner record = %q, %v; want this process and the base token", body, err)
	}
}

func TestReapIn_RefusesEveryRootThatIsNotAMarkedRunOfTheBase(t *testing.T) {
	t.Parallel()
	base, token := newBase(t)
	home, _ := os.UserHomeDir()
	foreign := filepath.Join(base, runPrefix+"foreign")
	writeFile(t, filepath.Join(foreign, "precious"), "keep")
	forged := filepath.Join(base, runPrefix+"forged")
	writeFile(t, filepath.Join(forged, ownerFile), "2 0.000000 "+strings.Repeat("f", 32))
	writeFile(t, filepath.Join(forged, "precious"), "keep")
	cluttered := filepath.Join(base, runPrefix+"cluttered")
	writeFile(t, filepath.Join(cluttered, ownerFile), "2 0.000000 "+token)
	writeFile(t, filepath.Join(cluttered, "precious"), "keep")
	for root, want := range map[string]string{
		"":                           "empty root",
		".":                          "not a clean absolute path",
		"run-x":                      "not a clean absolute path",
		base + "/run-a/../run-b":     "not a clean absolute path",
		"/":                          "not a run directory",
		home:                         "not a run directory",
		filepath.Clean(os.TempDir()): "not a run directory",
		base:                         "not a run directory",
		filepath.Join(base, "x"):     "not a run directory",
		foreign:                      "does not carry the marker",
		forged:                       "does not carry the marker",
		cluttered:                    "precious is not a fake tmux server",
	} {
		wantErr(t, reapIn(base, root), want)
	}
	for _, kept := range []string{"foreign", "forged", "cluttered"} {
		mustExist(t, filepath.Join(base, runPrefix+kept, "precious"))
	}
	if err := reapIn(base, filepath.Join(base, runPrefix+"gone")); err != nil {
		t.Errorf("a run that is already gone = %v, want nil", err)
	}
}

func TestReapIn_RefusesARunWhenTheBaseLostItsMarker(t *testing.T) {
	t.Parallel()
	base, token := newBase(t)
	run := filepath.Join(base, runPrefix+"r")
	writeFile(t, filepath.Join(run, ownerFile), "2 0.000000 "+token)
	if err := os.Remove(filepath.Join(base, markerFile)); err != nil {
		t.Fatal(err)
	}

	wantErr(t, reapIn(base, run), "does not carry the marker")
	mustExist(t, filepath.Join(run, ownerFile))
}

func TestReapIn_AServerDirectoryWithForeignContentIsNotRemoved(t *testing.T) {
	t.Parallel()
	base, token := newBase(t)
	run := filepath.Join(base, runPrefix+"r")
	writeFile(t, filepath.Join(run, ownerFile), "2 0.000000 "+token)
	writeFile(t, filepath.Join(run, "sock", "sessions", "x"), "")
	writeFile(t, filepath.Join(run, "sock", "precious"), "keep")

	wantErr(t, reapIn(base, run), "not empty")
	mustExist(t, filepath.Join(run, "sock", "precious"))
}

func TestReapIn_AnUnreadableRunIsAnError(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads every directory")
	}
	base, token := newBase(t)
	run := filepath.Join(base, runPrefix+"r")
	writeFile(t, filepath.Join(run, ownerFile), "2 0.000000 "+token)
	if err := os.Chmod(run, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(run, 0o700) })

	wantErr(t, reapIn(base, run), "permission denied")
}

func TestClaim_TheExportedPairClaimsAndReapsARunOfTheTempBase(t *testing.T) {
	t.Parallel()
	root, err := Claim()
	if err != nil {
		t.Fatal(err)
	}
	pid := startPane(t, root, "p")

	if err := Reap(root); err != nil {
		t.Fatal(err)
	}

	awaitGone(t, pid)
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("Reap left %s: %v", root, err)
	}
	wantErr(t, Reap("/"), "not a run directory")
	wantErr(t, Reap(""), "empty root")
}

func TestPrivateDir_ABaseOthersCanUseOrASymlinkIsRefused(t *testing.T) {
	t.Parallel()
	shared := t.TempDir()
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := claimIn(shared)
	wantErr(t, err, "only this user can use")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	_, err = claimIn(link)
	wantErr(t, err, "only this user can use")
	file := filepath.Join(t.TempDir(), "file")
	writeFile(t, file, "")
	_, err = claimIn(file)
	wantErr(t, err, "not a directory")
	wantErr(t, sweep(file, "t"), "not a directory")
}

func TestBaseToken_AMalformedMarkerIsRefused(t *testing.T) {
	t.Parallel()
	base := filepath.Join(t.TempDir(), "base")
	writeFile(t, filepath.Join(base, markerFile), "short")
	if err := os.Chmod(base, 0o700); err != nil {
		t.Fatal(err)
	}

	_, err := baseToken(base)

	wantErr(t, err, "has no valid marker")
}

func TestSession_APidThatStartedAgainIsNotTheRecordedPaneAndIsNeverSignalled(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pid := startPane(t, root, "reused")
	sess := session{name: "reused", dir: filepath.Join(root, "sock", "sessions", "reused")}
	writeFile(t, sess.path("pane"), strconv.Itoa(pid)+" 24 0.000000")

	if sess.alive() {
		t.Fatal("a pid whose start time differs from the record is not the pane")
	}
	if err := sess.kill(); err != nil {
		t.Fatal(err)
	}
	if syscall.Kill(pid, 0) != nil {
		t.Errorf("kill signalled pid %d, whose start time does not match the record", pid)
	}
	if err := signalGroup(pid); err != nil {
		t.Fatal(err)
	}
}

type recordingSender struct{ pids []int }

func (r *recordingSender) send(pid int) error {
	r.pids = append(r.pids, pid)
	return nil
}

func TestSeams_TheFakeNeverSignalsItselfItsParentOrANonShellLeader(t *testing.T) {
	restore := setSeams(t)
	sender := &recordingSender{}
	signalGroup = sender.send
	dir := t.TempDir()
	for _, pid := range []int{os.Getpid(), os.Getppid()} {
		begun, err := processStart(pid)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(dir, "pane"), strconv.Itoa(pid)+" 24 "+begun)
		_, err = session{dir: dir}.pane()
		wantErr(t, err, "names this process or its parent")
		if err := (session{dir: dir}).kill(); err != nil {
			t.Fatal(err)
		}
	}
	root := t.TempDir()
	pid := startPane(t, root, "leader")
	sess := session{name: "leader", dir: filepath.Join(root, "sock", "sessions", "leader")}
	for _, cmd := range []struct {
		comm string
		err  error
	}{{"sleep", nil}, {"/bin/sh", errInjected}} {
		commandOf = func(int) (string, error) { return cmd.comm, cmd.err }
		if err := sess.kill(); err != nil {
			t.Fatal(err)
		}
		writeFile(t, sess.path("pane"), recordOf(t, pid))
	}
	if len(sender.pids) != 0 {
		t.Errorf("the fake signalled %v; want no signal to itself, its parent or a leader that is not a shell", sender.pids)
	}
	restore()
	if !isPaneLeader(pid) {
		t.Errorf("the pane leader %d must be the shell wrapper", pid)
	}
}

func recordOf(t *testing.T, pid int) string {
	t.Helper()
	begun, err := processStart(pid)
	if err != nil {
		t.Fatal(err)
	}
	return strconv.Itoa(pid) + " 24 " + begun
}

func TestSeams_ClaimAndStartFailLoudlyWhenAStepFails(t *testing.T) {
	restore := setSeams(t)
	base, token := newBase(t)
	stale := filepath.Join(base, runPrefix+"dead")
	writeFile(t, filepath.Join(stale, ownerFile), "2 0.000000 "+token)
	startPane(t, stale, "held")

	signalGroup = func(int) error { return errInjected }
	_, err := claimIn(base)
	wantErr(t, err, "injected")
	restore()

	for _, step := range []func(){
		func() { mkdirTemp = func(string, string) (string, error) { return "", errInjected } },
		func() { newToken = func() (string, error) { return "", errInjected } },
		func() { newToken = func() (string, error) { return "no/such/dir", nil } },
	} {
		step()
		_, err = claimIn(base)
		if err == nil {
			t.Error("a failed claim step must fail the claim")
		}
		restore()
	}

	processStart = func(int) (string, error) { return "", errInjected }
	_, err = claimIn(base)
	wantErr(t, err, "read the start of the owner: injected")
	wantErr(t, session{name: "p", dir: t.TempDir()}.start([]string{"sleep", "30"}, "", 24), "record the pane: injected")
	restore()

	paneShell = filepath.Join(t.TempDir(), "missing")
	wantErr(t, session{name: "m", dir: t.TempDir()}.start([]string{"sleep", "30"}, "", 24), "start the pane")
	restore()
}

func TestReapIn_ASymlinkedRunOrServerIsRefusedAndItsTargetKept(t *testing.T) {
	t.Parallel()
	base, token := newBase(t)
	elsewhere := t.TempDir()
	writeFile(t, filepath.Join(elsewhere, ownerFile), "2 0.000000 "+token)
	writeFile(t, filepath.Join(elsewhere, "sessions", "precious"), "keep")
	linkedRun := filepath.Join(base, runPrefix+"linked")
	if err := os.Symlink(elsewhere, linkedRun); err != nil {
		t.Fatal(err)
	}
	run := filepath.Join(base, runPrefix+"r")
	writeFile(t, filepath.Join(run, ownerFile), "2 0.000000 "+token)
	if err := os.Symlink(elsewhere, filepath.Join(run, "srv")); err != nil {
		t.Fatal(err)
	}

	wantErr(t, reapIn(base, linkedRun), "not a real directory")
	wantErr(t, reapIn(base, run), "srv is not a fake tmux server")

	mustExist(t, filepath.Join(elsewhere, "sessions", "precious"))
}

func TestClaimIn_AStaleRunTheReaperRefusesIsLeftAndDoesNotFailTheClaim(t *testing.T) {
	t.Parallel()
	base, token := newBase(t)
	stale := filepath.Join(base, runPrefix+"stale")
	writeFile(t, filepath.Join(stale, ownerFile), "2 0.000000 "+token)
	writeFile(t, filepath.Join(stale, "precious"), "keep")

	if _, err := claimIn(base); err != nil {
		t.Fatalf("a refused stale run must not fail the claim: %v", err)
	}

	mustExist(t, filepath.Join(stale, "precious"))
}
