package testverdict

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/overlap"
	"github.com/mickeyyaya/evolve-loop/go/internal/testkey"
)

var epoch = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

const (
	packageA  = "example.com/m/internal/a"
	logReadsW = "# test log\ngetenv MODE\ngetenv GONE\nopen /repo/go/internal/a/a.go\n"
)

func digest(c string) string { return strings.Repeat(c, 64) }

func keyInput(codeID string, log testkey.Log, env map[string]string) testkey.Input {
	buildEnv := map[string]string{}
	for _, name := range testkey.BuildEnvNames() {
		buildEnv[name] = "v"
	}
	return testkey.Input{
		Package: packageA,
		Module: overlap.Module{Path: "example.com/m", Packages: []overlap.Package{
			{ImportPath: packageA, Dir: "go/internal/a", Files: []string{"go/internal/a/a.go"}},
		}},
		Toolchain:   testkey.Toolchain{GoVersion: "go1.23", BuildEnv: buildEnv},
		IDs:         map[string]string{"go/go.mod": "m", "go/go.sum": "s", "go/internal/a/a.go": "code-" + codeID},
		TestdataIDs: map[string]string{"go/internal/a": "td"},
		WholeTreeID: "tree-" + codeID,
		Log:         log,
		RepoRoot:    "/repo",
		WorkDir:     "/repo/go/internal/a",
		Env:         env,
	}
}

func recordWith(t *testing.T, codeID, logText string, env map[string]string) Record {
	t.Helper()
	log := testkey.ParseLog(logText)
	key, err := testkey.Compute(keyInput(codeID, log, env))
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return Record{Package: packageA, Outcome: Pass, Key: key, Log: log, Gate: "ship"}
}

func recordFor(t *testing.T, codeID string, env map[string]string) Record {
	t.Helper()
	return recordWith(t, codeID, logReadsW, env)
}

func greenRecord(t *testing.T) Record { return recordFor(t, "1", currentEnv()) }

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "testcache")
	return NewStore(root, func() time.Time { return epoch }), root
}

func mustPut(t *testing.T, s *Store, r Record) {
	t.Helper()
	if err := s.Put(r); err != nil {
		t.Fatalf("Put: %v", err)
	}
}

func currentEnv() map[string]string { return map[string]string{"MODE": "fast"} }

func files(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return out
}

func TestStore_ARedIsNeverStored(t *testing.T) {
	s, root := newStore(t)
	red := greenRecord(t)
	red.Outcome = Fail

	err := s.Put(red)

	if !errors.Is(err, ErrNotGreen) {
		t.Fatalf("Put(red) error = %v, want ErrNotGreen", err)
	}
	if got := files(t, root); len(got) != 0 {
		t.Fatalf("a red left files %v", got)
	}
	if _, ok := s.Lookup(red.Key.File, currentEnv()); ok {
		t.Fatal("a red was found by Lookup")
	}
}

func TestStore_ATimeoutIsNeverStored(t *testing.T) {
	s, root := newStore(t)
	slow := greenRecord(t)
	slow.Outcome = Timeout

	err := s.Put(slow)

	if !errors.Is(err, ErrNotGreen) {
		t.Fatalf("Put(timeout) error = %v, want ErrNotGreen", err)
	}
	if got := files(t, root); len(got) != 0 {
		t.Fatalf("a timeout left files %v", got)
	}
	blank := greenRecord(t)
	blank.Outcome = ""
	if err := s.Put(blank); !errors.Is(err, ErrNotGreen) {
		t.Fatalf("Put(empty outcome) error = %v, want ErrNotGreen", err)
	}
}

func TestStore_AHitNeedsAnEqualKey(t *testing.T) {
	s, _ := newStore(t)
	g := greenRecord(t)
	mustPut(t, s, g)

	hit, ok := s.Lookup(g.Key.File, currentEnv())
	if !ok || hit.Outcome != Pass || hit.Package != g.Package || hit.Key.Full != g.Key.Full {
		t.Fatalf("Lookup = %+v, %v, want the stored green", hit, ok)
	}
	if !hit.StoredAt.Equal(epoch) {
		t.Fatalf("StoredAt = %v, want %v", hit.StoredAt, epoch)
	}
	cases := map[string]struct {
		file string
		env  map[string]string
	}{
		"another file key":      {recordFor(t, "2", currentEnv()).Key.File, currentEnv()},
		"a changed value":       {g.Key.File, map[string]string{"MODE": "slow"}},
		"a missing variable":    {g.Key.File, map[string]string{}},
		"a newly set variable":  {g.Key.File, map[string]string{"MODE": "fast", "GONE": ""}},
		"a malformed file key":  {"../../etc", currentEnv()},
		"an empty file key":     {"", currentEnv()},
		"a short file key":      {"abc", currentEnv()},
		"an upper case hex key": {strings.ToUpper(digest("a")), currentEnv()},
	}
	for name, tc := range cases {
		if got, ok := s.Lookup(tc.file, tc.env); ok {
			t.Errorf("%s: Lookup hit %+v, want a miss", name, got)
		}
	}
}

func TestStore_AHitComesFromTheEntryWhoseRecordedEnvMatches(t *testing.T) {
	s, _ := newStore(t)
	fast := greenRecord(t)
	slow := recordFor(t, "1", map[string]string{"MODE": "slow"})
	mustPut(t, s, fast)
	mustPut(t, s, slow)

	got, ok := s.Lookup(fast.Key.File, map[string]string{"MODE": "slow"})

	if !ok || got.Key.Full != slow.Key.Full {
		t.Fatalf("Lookup = %+v, %v, want the slow entry", got, ok)
	}
}

func TestStore_ATornWriteIsAMiss(t *testing.T) {
	g := greenRecord(t)
	mutations := map[string]func(t *testing.T, path string){
		"truncated": func(t *testing.T, path string) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data[:len(data)/2], 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"empty": func(t *testing.T, path string) {
			if err := os.WriteFile(path, nil, 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"edited payload": func(t *testing.T, path string) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			edited := strings.Replace(string(data), "internal/a", "internal/z", 1)
			if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"a fail with a good sum": func(t *testing.T, path string) {
			bad := g
			bad.Outcome = Fail
			bad.StoredAt = epoch
			data := sealRecord(bad)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"a record moved under another key": func(t *testing.T, path string) {
			moved := recordFor(t, "2", currentEnv())
			moved.StoredAt = epoch
			data := sealRecord(moved)
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			s, root := newStore(t)
			mustPut(t, s, g)
			mutate(t, entryPath(root, g.Key.File, g.Key.Full, g.Gate))

			got, ok := s.Lookup(g.Key.File, currentEnv())

			if ok && got.Key.Full == g.Key.Full {
				t.Fatalf("a damaged entry hit: %+v", got)
			}
		})
	}
}

func TestStore_ATornWriteLeavesTheIntactSiblingAHit(t *testing.T) {
	s, root := newStore(t)
	g := greenRecord(t)
	sibling := recordFor(t, "1", map[string]string{"MODE": "slow"})
	mustPut(t, s, g)
	mustPut(t, s, sibling)
	if err := os.WriteFile(entryPath(root, g.Key.File, g.Key.Full, g.Gate), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := s.Lookup(g.Key.File, map[string]string{"MODE": "slow"})

	if !ok || got.Key.Full != sibling.Key.Full {
		t.Fatalf("Lookup = %+v, %v, want the intact sibling", got, ok)
	}
}

func TestStore_GCRemovesEntriesPastTheirAge(t *testing.T) {
	s, root := newStore(t)
	old, fresh := recordFor(t, "1", currentEnv()), recordFor(t, "2", currentEnv())
	other := recordFor(t, "2", map[string]string{"MODE": "x"})
	other.Package = "example.com/m/internal/b"
	mustPut(t, s, old)
	mustPut(t, s, fresh)
	mustPut(t, s, other)
	past := epoch.Add(-48 * time.Hour)
	for _, p := range []string{entryPath(root, old.Key.File, old.Key.Full, old.Gate), indexPath(root, old.Package)} {
		if err := os.Chtimes(p, past, past); err != nil {
			t.Fatal(err)
		}
	}

	removed, err := s.GC(24 * time.Hour)

	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if removed != 2 {
		t.Fatalf("GC removed %d files, want 2 (the old entry and its index)", removed)
	}
	if _, ok := s.Lookup(old.Key.File, currentEnv()); ok {
		t.Fatal("an entry past its age still hits")
	}
	if _, ok := s.Lookup(fresh.Key.File, currentEnv()); !ok {
		t.Fatal("a fresh entry was removed")
	}
	if _, err := os.Stat(filepath.Join(root, old.Key.File)); !os.IsNotExist(err) {
		t.Fatalf("the emptied key directory stayed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, fresh.Key.File)); err != nil {
		t.Fatalf("a key directory with a fresh entry went: %v", err)
	}
}

func TestStore_GCOfAMissingRootRemovesNothing(t *testing.T) {
	s, _ := newStore(t)

	removed, err := s.GC(time.Hour)

	if err != nil || removed != 0 {
		t.Fatalf("GC = %d, %v, want 0, nil", removed, err)
	}
}

func TestStore_GCReportsWhatItCannotRemove(t *testing.T) {
	s, root := newStore(t)
	g := greenRecord(t)
	mustPut(t, s, g)
	past := epoch.Add(-48 * time.Hour)
	if err := os.Chtimes(entryPath(root, g.Key.File, g.Key.Full, g.Gate), past, past); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, g.Key.File)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := s.GC(time.Hour); err == nil {
		t.Fatal("GC hid a file it could not remove")
	}
}

func TestStore_GCReportsADirectoryItCannotRemove(t *testing.T) {
	s, root := newStore(t)
	g := greenRecord(t)
	mustPut(t, s, g)
	past := epoch.Add(-48 * time.Hour)
	if err := os.Chtimes(entryPath(root, g.Key.File, g.Key.Full, g.Gate), past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(indexPath(root, g.Package), past, past); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(root, digest("9"))
	if err := os.Mkdir(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) })

	if _, err := s.GC(time.Hour); err == nil {
		t.Fatal("GC hid a directory it could not remove")
	}
}

func TestStore_GCReportsADirectoryItCannotRead(t *testing.T) {
	s, root := newStore(t)
	g := greenRecord(t)
	mustPut(t, s, g)
	dir := filepath.Join(root, g.Key.File)
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	if _, err := s.GC(time.Hour); err == nil {
		t.Fatal("GC hid a directory it could not read")
	}
}

func TestStore_GCOfARootThatIsAFileFails(t *testing.T) {
	root := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(root, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewStore(root, func() time.Time { return epoch }).GC(time.Hour); err == nil {
		t.Fatal("GC of a file root returned no error")
	}
}

func TestStore_PutRefusesAKeyThatIsNotADigest(t *testing.T) {
	s, root := newStore(t)
	for name, mutate := range map[string]func(*Record){
		"traversal file": func(r *Record) { r.Key.File = "../escape" },
		"short full":     func(r *Record) { r.Key.Full = "abc" },
		"edited full":    func(r *Record) { r.Key.Full = digest("0") },
		"empty":          func(r *Record) { r.Key = testkey.Key{} },
	} {
		r := greenRecord(t)
		mutate(&r)
		if err := s.Put(r); !errors.Is(err, ErrBadKey) {
			t.Errorf("%s: Put error = %v, want ErrBadKey", name, err)
		}
	}
	if got := files(t, root); len(got) != 0 {
		t.Fatalf("a refused key left files %v", got)
	}
}

func TestStore_PutReportsAStoreItCannotWrite(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := NewStore(file, func() time.Time { return epoch }).Put(greenRecord(t)); err == nil {
		t.Fatal("Put under a file root returned no error")
	}
	s, root := newStore(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "index"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(greenRecord(t)); err == nil {
		t.Fatal("Put with an unwritable index returned no error")
	}
}

func TestStore_LastLogIsTheLogOfTheLatestGreenOfThePackage(t *testing.T) {
	s, root := newStore(t)
	first := greenRecord(t)
	second := recordWith(t, "2", "# test log\ngetenv OTHER\n", currentEnv())
	mustPut(t, s, first)
	mustPut(t, s, second)

	got, ok := s.LastLog(first.Package)

	if !ok || len(got.Getenv) != 1 || got.Getenv[0] != "OTHER" {
		t.Fatalf("LastLog = %+v, %v, want the second log", got, ok)
	}
	if _, ok := s.LastLog("example.com/m/internal/none"); ok {
		t.Fatal("LastLog found a package that never stored")
	}
	if err := os.WriteFile(indexPath(root, first.Package), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.LastLog(first.Package); ok {
		t.Fatal("LastLog trusted a torn index")
	}
	red := greenRecord(t)
	red.Outcome = Fail
	if err := s.Put(red); !errors.Is(err, ErrNotGreen) {
		t.Fatalf("Put(red) = %v", err)
	}
	if _, ok := s.LastLog(first.Package); ok {
		t.Fatal("a red replaced the index")
	}
}

func TestStore_AStaleLogCannotGiveAFalseHit(t *testing.T) {
	logB := "# test log\ngetenv W\nopen /repo/go/internal/a/a.go\n"
	logC := "# test log\nopen /repo/go/internal/a/a.go\n"
	s, root := newStore(t)

	staleKey := recordWith(t, "B", logC, map[string]string{"W": "1"})
	staleKey.Log = testkey.ParseLog(logB)
	if err := s.Put(staleKey); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("Put(key from log C, log B) = %v, want ErrInconsistent", err)
	}
	if got := files(t, root); len(got) != 0 {
		t.Fatalf("an inconsistent record left files %v", got)
	}

	own := recordWith(t, "B", logB, map[string]string{"W": "1"})
	mustPut(t, s, own)
	mustPut(t, s, recordWith(t, "C", logC, map[string]string{}))
	last, ok := s.LastLog(packageA)
	if !ok {
		t.Fatal("no last log")
	}
	again, err := testkey.Compute(keyInput("B", last, map[string]string{"W": "2"}))
	if err != nil {
		t.Fatal(err)
	}
	if hit, ok := s.Lookup(again.File, map[string]string{"W": "2"}); ok {
		t.Fatalf("B returned with W changed and a stale log: false hit %+v", hit)
	}
	if _, ok := s.Lookup(own.Key.File, map[string]string{"W": "1"}); !ok {
		t.Fatal("B with its own log and an equal W did not hit")
	}
	if _, ok := s.Lookup(own.Key.File, map[string]string{"W": "2"}); ok {
		t.Fatal("B with W changed hit under its own key")
	}
}

func TestStore_AStaleLogCannotHideAnOutsideRead(t *testing.T) {
	logB := "# test log\nopen /etc/hosts\nopen /repo/go/internal/a/a.go\n"
	logC := "# test log\nopen /repo/go/internal/a/a.go\n"
	s, root := newStore(t)

	r := recordWith(t, "B", logC, currentEnv())
	r.Log = testkey.ParseLog(logB)
	if err := s.Put(r); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("Put = %v, want ErrInconsistent", err)
	}
	unknown := recordWith(t, "B", logC, currentEnv())
	unknown.Log = testkey.ParseLog("")
	if err := s.Put(unknown); !errors.Is(err, ErrInconsistent) {
		t.Fatalf("Put(unknown log under a closure key) = %v, want ErrInconsistent", err)
	}
	if got := files(t, root); len(got) != 0 {
		t.Fatalf("files left: %v", got)
	}
	own := recordWith(t, "B", logB, currentEnv())
	mustPut(t, s, own)
	if hit, ok := s.Lookup(own.Key.File, currentEnv()); !ok || !hit.Key.Whole {
		t.Fatalf("the whole-tree key of its own log = %+v, %v, want a whole-tree hit", hit, ok)
	}
}

func TestStore_GCIgnoresAFileThatAnotherRemoverTookFirst(t *testing.T) {
	s, root := newStore(t)
	g := greenRecord(t)
	mustPut(t, s, g)
	past := epoch.Add(-48 * time.Hour)
	for _, p := range []string{entryPath(root, g.Key.File, g.Key.Full, g.Gate), indexPath(root, g.Package)} {
		if err := os.Chtimes(p, past, past); err != nil {
			t.Fatal(err)
		}
	}
	original := removePath
	removePath = func(p string) error { return fs.ErrNotExist }
	t.Cleanup(func() { removePath = original })

	removed, err := s.GC(time.Hour)

	if err != nil || removed != 0 {
		t.Fatalf("GC = %d, %v, want 0, nil when the files are already gone", removed, err)
	}
}

func TestStore_AForgedRecordAtItsOwnPathIsAMiss(t *testing.T) {
	forgeries := map[string]func(t *testing.T) Record{
		"a log that reads more than the key holds": func(t *testing.T) Record {
			r := recordWith(t, "1", "# test log\nopen /repo/go/internal/a/a.go\n", currentEnv())
			r.Log = testkey.ParseLog(logReadsW)
			return r
		},
		"a full key that its parts do not give": func(t *testing.T) Record {
			r := greenRecord(t)
			r.Key.Full = digest("0")
			return r
		},
	}
	for name, forge := range forgeries {
		t.Run(name, func(t *testing.T) {
			s, root := newStore(t)
			r := forge(t)
			r.StoredAt = epoch
			path := entryPath(root, r.Key.File, r.Key.Full, r.Gate)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, sealRecord(r), 0o644); err != nil {
				t.Fatal(err)
			}

			if hit, ok := s.Lookup(r.Key.File, currentEnv()); ok {
				t.Fatalf("a forged record hit: %+v", hit)
			}
		})
	}
}
