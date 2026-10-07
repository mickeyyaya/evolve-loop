package ledgerartifacts

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func TestOpen_RootsTheStoreUnderTheEvolveDir(t *testing.T) {
	evolveDir := t.TempDir()
	body := []byte("diff --git a/x b/x\n")

	digest := sha256Hex(body)
	var store Store = Open(evolveDir)

	got, err := store.Path(digest)

	if want := filepath.Join(evolveDir, DirName, "sha256", digest[:2], digest[2:]); err != nil || got != want {
		t.Fatalf("Path = (%q, %v), want %q", got, err, want)
	}
	if DirName != "ledger-artifacts" {
		t.Fatalf("DirName = %q: gc protects this name and the ledger docs cite it", DirName)
	}
}

func TestPut_NamesTheBytesByTheirSHA256AndGetReturnsThem(t *testing.T) {
	store := Open(t.TempDir())
	body := []byte("the audited change\n")

	digest, err := store.Put(body)
	if err != nil {
		t.Fatal(err)
	}
	if digest != sha256Hex(body) {
		t.Fatalf("Put = %s, want the sha256 of the bytes %s", digest, sha256Hex(body))
	}
	got, err := store.Get(digest)
	if err != nil || string(got) != string(body) {
		t.Fatalf("Get = (%q, %v), want the stored bytes", got, err)
	}
}

func TestPut_IsIdempotentAndNeverRewritesAValidObject(t *testing.T) {
	evolveDir := t.TempDir()
	store := Open(evolveDir)
	body := []byte("write once\n")
	digest := sha256Hex(body)
	path := filepath.Join(evolveDir, DirName, "sha256", digest[:2], digest[2:])
	if _, err := store.Put(body); err != nil {
		t.Fatal(err)
	}
	before, statErr := os.Stat(path)

	again, err := store.Put(body)

	if err != nil || again != digest {
		t.Fatalf("second Put = (%s, %v), want (%s, nil)", again, err, digest)
	}
	after, err := os.Stat(path)
	if statErr != nil || err != nil || !os.SameFile(before, after) {
		t.Fatalf("the object at %s was not kept in place (first stat err=%v, second stat err=%v)", path, statErr, err)
	}
}

func TestGet_RefusesBytesThatNoLongerHashToTheirName(t *testing.T) {
	evolveDir := t.TempDir()
	store := Open(evolveDir)
	digest := sha256Hex([]byte("original\n"))
	path := filepath.Join(evolveDir, DirName, "sha256", digest[:2], digest[2:])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.Get(digest)

	if !errors.Is(err, errCorrupt) {
		t.Fatalf("Get of a tampered object = %v, want errCorrupt", err)
	}
}

func TestPut_ReplacesAnObjectWhoseBytesNoLongerHashToItsName(t *testing.T) {
	evolveDir := t.TempDir()
	store := Open(evolveDir)
	digest := sha256Hex([]byte("original\n"))
	path := filepath.Join(evolveDir, DirName, "sha256", digest[:2], digest[2:])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := store.Put([]byte("original\n"))

	if got, readErr := os.ReadFile(path); err != nil || readErr != nil || string(got) != "original\n" {
		t.Fatalf("after Put the object holds %q (put err=%v, read err=%v), want the bytes its name commits to", got, err, readErr)
	}
}

func TestGet_AMissingObjectIsNotExist(t *testing.T) {
	_, err := Open(t.TempDir()).Get(sha256Hex([]byte("never stored")))

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Get of an absent object = %v, want fs.ErrNotExist", err)
	}
}

func TestPath_RefusesANameThatIsNotASHA256Digest(t *testing.T) {
	store := Open(t.TempDir())
	for _, name := range []string{"", "../../ledger.jsonl", "ABCDEF" + sha256Hex(nil)[6:], sha256Hex(nil)[:63]} {
		if _, err := store.Path(name); !errors.Is(err, errBadDigest) {
			t.Errorf("Path(%q) = %v, want errBadDigest", name, err)
		}
		if _, err := store.Get(name); !errors.Is(err, errBadDigest) {
			t.Errorf("Get(%q) = %v, want errBadDigest", name, err)
		}
	}
}

func TestPut_FailsLoudlyWhenTheStoreCannotBeCreated(t *testing.T) {
	evolveDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(evolveDir, DirName), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(evolveDir).Put([]byte("x")); err == nil {
		t.Fatal("Put into a store that cannot be created must fail")
	}
}

func TestPut_FailsLoudlyWhenTheObjectCannotBeWritten(t *testing.T) {
	evolveDir := t.TempDir()
	store := Open(evolveDir)
	body := []byte("blocked\n")
	digest := sha256Hex(body)
	path := filepath.Join(evolveDir, DirName, "sha256", digest[:2], digest[2:])
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := store.Put(body); err == nil {
		t.Fatal("Put must fail when its object path is not a file it can place")
	}
}

func TestPut_FailsLoudlyWhenTheFanOutDirIsReadOnly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes through a read-only directory, so this run cannot show Put failing on one")
	}
	evolveDir := t.TempDir()
	store := Open(evolveDir)
	body := []byte("read-only\n")
	digest := sha256Hex(body)
	path := filepath.Join(evolveDir, DirName, "sha256", digest[:2], digest[2:])
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(path), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(path), 0o755) })

	if _, err := store.Put(body); err == nil {
		t.Fatal("Put must fail when it cannot stage the object")
	}
}

func TestPut_LeavesTheObjectReadOnly(t *testing.T) {
	store := Open(t.TempDir())
	digest, err := store.Put([]byte("write once\n"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.Path(digest)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)

	if err != nil || info.Mode().Perm()&0o222 != 0 {
		t.Fatalf("object %s mode = %v (stat err=%v), want no write bit: the store is write-once", path, info.Mode(), err)
	}
}

func TestDigest_IsTheNameAnObjectIsStoredUnder(t *testing.T) {
	body := []byte("the audited change\n")
	store := Open(t.TempDir())

	digest, err := store.Put(body)

	if err != nil || Digest(body) != sha256Hex(body) || digest != Digest(body) {
		t.Fatalf("Digest = %s, Put = (%s, %v), want both the sha256 of the bytes %s", Digest(body), digest, err, sha256Hex(body))
	}
}

func exitedPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.ProcessState.Pid()
}

func TestPut_ReapsTheTempFilesOfAWriterThatDied(t *testing.T) {
	evolveDir := t.TempDir()
	store := Open(evolveDir)
	body := []byte("reaped beside\n")
	digest := sha256Hex(body)
	fanOut := filepath.Join(evolveDir, DirName, "sha256", digest[:2])
	if err := os.MkdirAll(fanOut, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(fanOut, fmt.Sprintf(".put-%d-1", exitedPID(t)))
	inFlight := filepath.Join(fanOut, fmt.Sprintf(".put-%d-2", os.Getpid()))
	for _, p := range []string{stale, inFlight} {
		if err := os.WriteFile(p, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := store.Put(body); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("the temp file of a writer that died is still in the store (stat err=%v)", err)
	}
	if _, err := os.Stat(inFlight); err != nil {
		t.Errorf("the temp file of a live writer was reaped: %v", err)
	}
}

func TestPut_LeavesAloneEveryFileThatIsNotTheTempOfAWriterThatDied(t *testing.T) {
	evolveDir := t.TempDir()
	body := []byte("neighbours\n")
	digest := sha256Hex(body)
	fanOut := filepath.Join(evolveDir, DirName, "sha256", digest[:2])
	if err := os.MkdirAll(fanOut, 0o755); err != nil {
		t.Fatal(err)
	}
	dead := exitedPID(t)
	kept := []string{".put-notapid-1", fmt.Sprintf(".put-%d", dead), ".put-0-1", fmt.Sprintf("put-%d-1", dead), "0123456789abcdef"}
	for _, name := range kept {
		if err := os.WriteFile(filepath.Join(fanOut, name), []byte("kept"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Open(evolveDir).Put(body); err != nil {
		t.Fatal(err)
	}

	for _, name := range kept {
		if _, err := os.Stat(filepath.Join(fanOut, name)); err != nil {
			t.Errorf("Put removed %s, which no writer of this store named: %v", name, err)
		}
	}
}

func TestPut_KeepsTheTempOfALiveWriterItCannotSignal(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can signal every process, so this run cannot show a live writer it is refused to signal")
	}
	evolveDir := t.TempDir()
	body := []byte("another user's writer\n")
	digest := sha256Hex(body)
	fanOut := filepath.Join(evolveDir, DirName, "sha256", digest[:2])
	if err := os.MkdirAll(fanOut, 0o755); err != nil {
		t.Fatal(err)
	}
	ofLaunchd := filepath.Join(fanOut, ".put-1-1")
	if err := os.WriteFile(ofLaunchd, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Open(evolveDir).Put(body); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(ofLaunchd); err != nil {
		t.Errorf("Put reaped the temp of pid 1, which is alive and only refuses this user's signal (EPERM): %v", err)
	}
}

func TestPut_TheReaperReadsTheTempNamesItsWriterCreates(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), tempPattern(4242))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	pid, ok := tempWriter(filepath.Base(f.Name()))

	if !ok || pid != 4242 {
		t.Fatalf("tempWriter(%q) = (%d, %v), want (4242, true): the reaper must parse the name its own writer creates", filepath.Base(f.Name()), pid, ok)
	}
}

func TestPut_FailsLoudlyWhenItCannotListItsFanOutDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root lists every directory, so this run cannot show an unlistable fan-out dir")
	}
	evolveDir := t.TempDir()
	body := []byte("unlistable\n")
	digest := sha256Hex(body)
	fanOut := filepath.Join(evolveDir, DirName, "sha256", digest[:2])
	if err := os.MkdirAll(fanOut, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(fanOut, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(fanOut, 0o755) })

	_, err := Open(evolveDir).Put(body)

	if err == nil {
		t.Fatal("Put into a fan-out dir it cannot list = nil, want the failure surfaced")
	}
}
