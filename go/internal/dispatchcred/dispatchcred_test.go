package dispatchcred

import (
	"errors"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func newStore(t *testing.T) Store {
	t.Helper()
	return Store{Dir: filepath.Join(t.TempDir(), ".evolve", "dispatch")}
}

func TestStore_EnsureWritesAPrivateCredentialAndTheStageThatHoldPasses(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.Ensure("enforce"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(filepath.Join(s.HeldDir(), credentialFile))
	if err != nil || info.Mode().Perm() != 0o600 || !info.Mode().IsRegular() {
		t.Fatalf("the credential must be a 0600 regular file: %v %v", info, err)
	}
	if dir, err := os.Stat(s.HeldDir()); err != nil || dir.Mode().Perm() != 0o700 {
		t.Fatalf("the held directory must be 0700: %v %v", dir, err)
	}
	if err := s.Hold(); err != nil {
		t.Fatalf("Hold after Ensure: %v", err)
	}
	if got := s.Stage(); got != "enforce" {
		t.Fatalf("Stage = %q, want enforce", got)
	}
}

func TestStore_EnsureKeepsTheCredentialAndReplacesTheStage(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.Ensure("shadow"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(s.HeldDir(), credentialFile)
	first, _ := os.ReadFile(path)
	if err := s.Ensure("enforce"); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) || len(first) == 0 {
		t.Fatalf("a second Ensure must keep the credential: %q then %q", first, second)
	}
	if got := s.Stage(); got != "enforce" {
		t.Fatalf("Stage = %q, want the last stage enforce", got)
	}
}

func TestStore_HoldCreatesAMissingCredentialForTheOperator(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.Hold(); err != nil {
		t.Fatalf("an operator shell with no credential yet must get one: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.HeldDir(), credentialFile)); err != nil {
		t.Fatal(err)
	}
	if got := s.Stage(); got != "shadow" {
		t.Fatalf("Stage with no stage file = %q, want shadow", got)
	}
}

func TestStore_HoldRefusesACredentialThatItCannotCreate(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	blocker := filepath.Join(parent, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (Store{Dir: blocker}).Hold(); err == nil {
		t.Fatal("Hold must fail when the held directory cannot be made")
	}
	if os.Getuid() == 0 {
		t.Skip("root writes in a 0500 directory")
	}
	s := newStore(t)
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(s.HeldDir(), 0o500); err != nil {
		t.Fatal(err)
	}
	if err := s.Hold(); err == nil || !strings.Contains(err.Error(), "create") {
		t.Fatalf("Hold must fail when the credential cannot be written: %v", err)
	}
}

func TestStore_HoldRefusesAForgedOrMaskedCredential(t *testing.T) {
	t.Parallel()
	for name, plant := range map[string]func(path string) error{
		"a link to /dev/null, as a bwrap mask": func(p string) error { return os.Symlink(os.DevNull, p) },
		"a group-readable file":                func(p string) error { return os.WriteFile(p, []byte(strings.Repeat("a", tokenHexLen)), 0o640) },
		"an empty file":                        func(p string) error { return os.WriteFile(p, nil, 0o600) },
		"a short token":                        func(p string) error { return os.WriteFile(p, []byte("abc"), 0o600) },
		"a token that is not hex":              func(p string) error { return os.WriteFile(p, []byte(strings.Repeat("z", tokenHexLen)), 0o600) },
		"a directory":                          func(p string) error { return os.Mkdir(p, 0o700) },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := newStore(t)
			if err := os.MkdirAll(s.HeldDir(), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := plant(filepath.Join(s.HeldDir(), credentialFile)); err != nil {
				t.Fatal(err)
			}
			if err := s.Hold(); err == nil {
				t.Fatal("Hold accepted a credential that the pipeline did not write")
			}
		})
	}
}

func TestStore_HoldRefusesAnUnreadableCredential(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := s.Ensure("enforce"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(s.HeldDir(), credentialFile), 0o000); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() == 0 {
		t.Skip("root reads a 0000 file")
	}
	err := s.Hold()
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Hold must refuse an unreadable credential without making a new one: %v", err)
	}
}

func TestStore_EnsureFailsLoudly(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	blocker := filepath.Join(parent, "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := (Store{Dir: blocker}).Ensure("enforce"); err == nil {
		t.Fatal("Ensure must fail when the store directory cannot be made")
	}
	s := newStore(t)
	if err := os.MkdirAll(s.HeldDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(s.Dir, stageFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := s.Ensure("enforce"); err == nil {
		t.Fatal("Ensure must fail when the stage cannot be replaced")
	}
	if os.Getuid() == 0 {
		t.Skip("root writes in a 0500 directory")
	}
	ro := newStore(t)
	if err := os.MkdirAll(ro.HeldDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ro.Dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ro.Dir, 0o700) })
	if err := ro.Ensure("enforce"); err == nil || !strings.Contains(err.Error(), "stage") {
		t.Fatalf("Ensure must fail when the stage file cannot be made: %v", err)
	}
}

func TestStore_ARandomSourceFailureLeavesNoCredential(t *testing.T) {
	s := newStore(t)
	saved := readRandom
	readRandom = func([]byte) (int, error) { return 0, errors.New("no entropy") }
	t.Cleanup(func() { readRandom = saved })
	if err := s.Hold(); err == nil || !strings.Contains(err.Error(), "no entropy") {
		t.Fatalf("Hold must report the random source failure: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(s.HeldDir(), credentialFile)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a failed write must leave no credential: %v", err)
	}
}

func TestStore_StageReadsTheWordThatThePipelineWrote(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := s.Stage(); got != "shadow" {
		t.Errorf("Stage with no stage file = %q, want shadow", got)
	}
	for raw, want := range map[string]string{"enforce\n": "enforce", "off": "off", "shadow": "shadow", "": "enforce", "0": "enforce", "Off": "enforce", "x": "enforce"} {
		if err := os.WriteFile(filepath.Join(s.Dir, stageFile), []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := s.Stage(); got != want {
			t.Errorf("Stage(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestDefault_IsUnderThePasswdHomeAndIgnoresHOME(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	u, err := user.LookupId(strconv.Itoa(os.Getuid()))
	if err != nil {
		t.Skipf("no passwd entry for this uid: %v", err)
	}
	s, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(u.HomeDir, ".evolve", "dispatch"); s.Dir != want {
		t.Fatalf("Default().Dir = %q, want %q: a cell must not move the credential with HOME", s.Dir, want)
	}
}

func TestDefault_TheBuildRootWinsAndALookupFailureIsAnError(t *testing.T) {
	savedRoot, savedLookup := root, lookupID
	t.Cleanup(func() { root, lookupID = savedRoot, savedLookup })
	root = "/build/root"
	if s, err := Default(); err != nil || s.Dir != "/build/root" {
		t.Fatalf("Default with a build root = %+v, %v", s, err)
	}
	root = ""
	lookupID = func(string) (*user.User, error) { return nil, errors.New("no passwd") }
	if _, err := Default(); err == nil || !strings.Contains(err.Error(), "no passwd") {
		t.Fatalf("Default must report the lookup failure: %v", err)
	}
	lookupID = func(string) (*user.User, error) { return &user.User{}, nil }
	if _, err := Default(); err == nil {
		t.Fatal("Default must refuse an empty home")
	}
}

func TestAdmit_HeldPassesAndAMissingCredentialCarriesTheStage(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if v := Admit(func() (Store, error) { return s, nil }); !v.Held || v.Violation() || v.Refused() {
		t.Fatalf("a host that can hold the credential must pass: %+v", v)
	}
	if err := s.Ensure("enforce"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(s.HeldDir(), credentialFile), 0o644); err != nil {
		t.Fatal(err)
	}
	v := Admit(func() (Store, error) { return s, nil })
	if v.Held || !v.Refused() || !v.Violation() || v.Stage != "enforce" || v.Cause == nil {
		t.Fatalf("an unheld credential at enforce must refuse: %+v", v)
	}
	v = Admit(func() (Store, error) { return Store{}, errors.New("no home") })
	if v.Held || v.Refused() || !v.Violation() || v.Stage != "shadow" {
		t.Fatalf("an unknown store is shadow: it reports and does not refuse: %+v", v)
	}
	if v := (Verdict{Stage: "off", Cause: errors.New("x")}); v.Violation() || v.Refused() {
		t.Fatalf("off neither reports nor refuses: %+v", v)
	}
}

func TestStore_AStageFileThatCannotBeReadIsEnforce(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	if err := os.MkdirAll(filepath.Join(s.Dir, stageFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := s.Stage(); got != "enforce" {
		t.Fatalf("an unreadable stage = %q, want enforce: a cell must not turn the gate down by a stage it cannot read", got)
	}
}

func TestStore_ConcurrentHoldsAllSeeOneWholeCredential(t *testing.T) {
	t.Parallel()
	s := newStore(t)
	errs := make(chan error, 16)
	for range 16 {
		go func() { errs <- s.Hold() }()
	}
	for range 16 {
		if err := <-errs; err != nil {
			t.Fatalf("a concurrent Hold saw a partial credential: %v", err)
		}
	}
	entries, err := os.ReadDir(s.HeldDir())
	if err != nil || len(entries) != 1 {
		t.Fatalf("held/ must keep the credential only, no temporary file: %v %v", entries, err)
	}
}

func TestAdmit_AMaskedCredentialWithNoStageFileIsEnforce(t *testing.T) {
	t.Parallel()
	if os.Getuid() == 0 {
		t.Skip("root reads a 0000 directory")
	}
	s := newStore(t)
	if err := os.MkdirAll(s.HeldDir(), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(s.HeldDir(), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.HeldDir(), 0o700) })
	v := Admit(func() (Store, error) { return s, nil })
	if v.Held || v.Stage != "enforce" || !v.Refused() {
		t.Fatalf("a cell that removed the stage and cannot read the credential must be refused: %+v", v)
	}
	fresh := newStore(t)
	if err := os.MkdirAll(fresh.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fresh.Dir, heldDir), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if v := Admit(func() (Store, error) { return fresh, nil }); v.Held || v.Stage != "shadow" {
		t.Fatalf("a credential that cannot be made, with no stage file, stays shadow: %+v", v)
	}
}
