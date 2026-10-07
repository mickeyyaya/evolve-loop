package atomicwrite

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type fakeDir struct {
	syncErr, closeErr error
	closed            bool
}

func (d *fakeDir) Sync() error  { return d.syncErr }
func (d *fakeDir) Close() error { d.closed = true; return d.closeErr }

func TestDurable_SyncError_CleansUpTemp(t *testing.T) {
	sentinel := errors.New("sync boom")
	ft := &fakeTemp{name: "t.tmp", syncErr: sentinel}
	var removed string
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return ft, nil }
		removeFile = func(n string) error { removed = n; return nil }
	})

	err := Durable(filepath.Join(t.TempDir(), "out"), []byte("x"), 0o600)

	if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), "sync") {
		t.Fatalf("want wrapped sync error, got %v", err)
	}
	if !ft.closed || removed != ft.name {
		t.Errorf("expected temp closed (%v) and removed (%q==%q)", ft.closed, removed, ft.name)
	}
}

func TestBytes_NeverSyncs(t *testing.T) {
	ft := &fakeTemp{name: "t.tmp", syncErr: errors.New("Bytes must not sync")}
	withSeams(t, func() {
		createTemp = func(string, string) (tempFile, error) { return ft, nil }
		renameFile = func(string, string) error { return nil }
		openDir = func(string) (syncCloser, error) { return nil, errors.New("Bytes must not open the directory") }
	})

	if err := Bytes(filepath.Join(t.TempDir(), "out"), []byte("x")); err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if ft.synced {
		t.Error("Bytes fsynced its temp file; only Durable may")
	}
}

func TestDurable_DirectorySyncFaults(t *testing.T) {
	sentinel := errors.New("dir boom")
	cases := []struct {
		name    string
		openErr error
		dir     *fakeDir
		want    string
	}{
		{"open", sentinel, nil, "open dir"},
		{"sync", nil, &fakeDir{syncErr: sentinel}, "sync dir"},
		{"close", nil, &fakeDir{closeErr: sentinel}, "close dir"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withSeams(t, func() {
				openDir = func(string) (syncCloser, error) {
					if tc.openErr != nil {
						return nil, tc.openErr
					}
					return tc.dir, nil
				}
			})
			path := filepath.Join(t.TempDir(), "out")

			err := Durable(path, []byte("renamed before the directory sync\n"), 0o600)

			if !errors.Is(err, sentinel) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want wrapped %q error, got %v", tc.want, err)
			}
			if tc.dir != nil && !tc.dir.closed {
				t.Error("the directory handle was not closed")
			}
			if got, rerr := os.ReadFile(path); rerr != nil || string(got) != "renamed before the directory sync\n" {
				t.Errorf("the renamed file = %q, %v", got, rerr)
			}
		})
	}
}

func TestTempWriter_ReadsThePidOfTheWriterThatCreatedTheTemp(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), tempPattern("/store/sha256/ab/cdef.object", 4242))
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	pid, ok := TempWriter(filepath.Base(f.Name()))

	if !ok || pid != 4242 {
		t.Fatalf("TempWriter(%q) = (%d, %v), want (4242, true)", filepath.Base(f.Name()), pid, ok)
	}
}

func TestDurable_NamesItsTempAfterTheWritingProcess(t *testing.T) {
	var pattern string
	withSeams(t, func() {
		createTemp = func(dir, p string) (tempFile, error) {
			pattern = p
			return nil, errors.New("stop after naming")
		}
	})

	_ = Durable(filepath.Join(t.TempDir(), "out"), []byte("x"), 0o600)

	if pid, ok := TempWriter(strings.Replace(pattern, "*", "123", 1)); !ok || pid != os.Getpid() {
		t.Fatalf("Durable's temp pattern %q names writer (%d, %v), want this process %d", pattern, pid, ok, os.Getpid())
	}
}

func TestTempWriter_RefusesNamesItsWritersNeverCreate(t *testing.T) {
	for _, name := range []string{
		"object",
		"cdef.4242.123.tmp",
		".4242.tmp",
		".4242.123.tmp",
		".cdef.tmp",
		".cdef.notapid.123.tmp",
		".cdef.0.123.tmp",
		".cdef.-7.123.tmp",
		".put-4242-1",
		".cdef.4242.123.tmp.gz",
	} {
		if pid, ok := TempWriter(name); ok {
			t.Errorf("TempWriter(%q) = (%d, true), want not a temp", name, pid)
		}
	}
}
