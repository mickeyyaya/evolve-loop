package cliroutetest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

type fatalSignalF3 struct{}

type fatalTBF3 struct {
	testing.TB
	dir     string
	message string
	fatal   bool
}

func (f *fatalTBF3) Helper()         {}
func (f *fatalTBF3) TempDir() string { return f.dir }
func (f *fatalTBF3) Fatalf(format string, args ...any) {
	f.message, f.fatal = fmt.Sprintf(format, args...), true
	panic(fatalSignalF3{})
}
func (f *fatalTBF3) Fatal(args ...any) {
	f.message, f.fatal = fmt.Sprint(args...), true
	panic(fatalSignalF3{})
}

func runUntilFatalF3(dir string, body func(tb testing.TB)) (message string, fatal bool) {
	tb := &fatalTBF3{dir: dir}
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(fatalSignalF3); !ok {
				panic(r)
			}
		}
		message, fatal = tb.message, tb.fatal
	}()
	body(tb)
	return "", false
}

func requireFatalF3(t *testing.T, want string, dir string, body func(tb testing.TB)) {
	t.Helper()
	message, fatal := runUntilFatalF3(dir, body)
	if !fatal || !strings.Contains(message, want) {
		t.Fatalf("want a fatal containing %q, got fatal=%v message=%q", want, fatal, message)
	}
}

func requireCleanF3(t *testing.T, dir string, body func(tb testing.TB)) {
	t.Helper()
	if message, fatal := runUntilFatalF3(dir, body); fatal {
		t.Fatalf("unexpected fatal: %s", message)
	}
}

func fileAtF3(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestVariantPathDir_FatalWhenAStubCannotBeWritten(t *testing.T) {
	v := Variant{Name: "bad_stub", Installed: []string{"claude", filepath.Join("missing-subdir", "codex")}}

	requireFatalF3(t, "stub missing-subdir", t.TempDir(), func(tb testing.TB) { v.PathDir(tb) })
}

func TestVariantPathDir_NoInstalledBinariesYieldsAnEmptyDir(t *testing.T) {
	dir := t.TempDir()
	var got string

	requireCleanF3(t, dir, func(tb testing.TB) { got = Variant{Name: "bare"}.PathDir(tb) })

	entries, err := os.ReadDir(got)
	if err != nil || len(entries) != 0 {
		t.Fatalf("an empty Installed set writes no stub: %v %v", entries, err)
	}
}

func TestVariantProjectRoot_FatalWhenTheBenchStoreCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	fileAtF3(t, filepath.Join(root, ".evolve"), "not a directory")
	v := Variant{Name: "benched", BenchedFamily: "codex"}

	requireFatalF3(t, "bench codex", root, func(tb testing.TB) { v.ProjectRoot(tb, nil) })
}

func TestVariantProjectRoot_FatalWhenThePolicyCannotBeWritten(t *testing.T) {
	root := t.TempDir()
	fileAtF3(t, filepath.Join(root, ".evolve"), "not a directory")
	v := Variant{Name: "pin", Pin: &policy.Pin{CLI: "agy", Model: "deep"}}

	requireFatalF3(t, "mkdir", root, func(tb testing.TB) { v.ProjectRoot(tb, []string{"build"}) })
}

func TestVariantPolicy_FatalNamesTheVariantWhenThePolicyDoesNotLoad(t *testing.T) {
	root := t.TempDir()
	fileAtF3(t, filepath.Join(root, ".evolve", "policy.json"), "{not json")
	v := Variant{Name: "broken_policy"}

	requireFatalF3(t, "load variant broken_policy policy", root, func(tb testing.TB) { v.Policy(tb, root) })
}

func TestWritePolicy_EachFailingStepIsFatalAndNamed(t *testing.T) {
	cases := []struct {
		name  string
		want  string
		setup func(t *testing.T, root string)
		doc   map[string]any
	}{
		{name: "marshal", want: "marshal policy", setup: func(*testing.T, string) {}, doc: map[string]any{"bad": make(chan int)}},
		{name: "mkdir", want: "mkdir", setup: func(t *testing.T, root string) { fileAtF3(t, filepath.Join(root, ".evolve"), "x") }, doc: map[string]any{}},
		{name: "write", want: "write policy", setup: func(t *testing.T, root string) {
			if err := os.MkdirAll(filepath.Join(root, ".evolve", "policy.json"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, doc: map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)

			requireFatalF3(t, tc.want, root, func(tb testing.TB) { writePolicy(tb, root, tc.doc) })
		})
	}
}

func TestWritePolicy_WritesTheDocumentTheLoaderReadsBack(t *testing.T) {
	root := t.TempDir()

	requireCleanF3(t, root, func(tb testing.TB) {
		writePolicy(tb, root, map[string]any{"pins": map[string]policy.Pin{"build": {CLI: "agy", Model: "deep"}}})
	})

	p, err := policy.Load(filepath.Join(root, ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	if pin, ok := p.PinFor("build"); !ok || pin.CLI != "agy" || pin.Model != "deep" {
		t.Fatalf("pin = %+v %v", pin, ok)
	}
}

func TestSelfFile_FatalWhenTheCallerCannotBeResolved(t *testing.T) {
	failing := func(int) (uintptr, string, int, bool) { return 0, "", 0, false }

	requireFatalF3(t, "runtime.Caller failed", t.TempDir(), func(tb testing.TB) { selfFile(tb, failing) })
}

func TestRepoRootAndTestdataPath_ResolveFromThisSourceFile(t *testing.T) {
	root := RepoRoot(t)
	if _, err := os.Stat(filepath.Join(root, "go", "go.mod")); err != nil {
		t.Fatalf("RepoRoot %s: %v", root, err)
	}
	path := testdataPath(t, "x.jsonl")
	if filepath.Base(path) != "x.jsonl" || filepath.Base(filepath.Dir(path)) != "testdata" {
		t.Fatalf("testdataPath = %s", path)
	}
}

func TestEncodeWith_ReturnsTheMarshalErrorAndNoBytes(t *testing.T) {
	boom := errors.New("marshal boom")
	calls := 0
	marshal := func(any) ([]byte, error) {
		calls++
		if calls == 2 {
			return nil, boom
		}
		return []byte("{}"), nil
	}

	raw, err := encodeWith([]Record{{}, {}, {}}, marshal)

	if !errors.Is(err, boom) || raw != nil || calls != 2 {
		t.Fatalf("raw=%q err=%v calls=%d", raw, err, calls)
	}
}

func TestEncode_EmptyAndOrderedRecords(t *testing.T) {
	raw, err := Encode(nil)
	if err != nil || len(raw) != 0 {
		t.Fatalf("no records encode to no bytes: %q %v", raw, err)
	}
	raw, err = Encode([]Record{{Agent: "b"}, {Agent: "a"}})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"agent":"b"`) || !strings.Contains(lines[1], `"agent":"a"`) {
		t.Fatalf("records keep their order, one per line: %q", raw)
	}
}

func TestReadGoldenAt_FatalOnAMissingFileAndOnABadLine(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.jsonl")
	fileAtF3(t, bad, "{\"agent\":\"a\"}\n{oops\n")

	requireFatalF3(t, "read golden", dir, func(tb testing.TB) { readGoldenAt(tb, filepath.Join(dir, "absent.jsonl")) })
	requireFatalF3(t, "golden line", dir, func(tb testing.TB) { readGoldenAt(tb, bad) })
}

func TestReadGoldenAt_DecodesEveryLineInOrder(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.jsonl")
	fileAtF3(t, path, "{\"agent\":\"a\",\"model\":\"deep\"}\n{\"agent\":\"b\"}\n")
	var got []Record

	requireCleanF3(t, dir, func(tb testing.TB) { got = readGoldenAt(tb, path) })

	if len(got) != 2 || got[0].Agent != "a" || got[0].Model != "deep" || got[1].Agent != "b" {
		t.Fatalf("records = %+v", got)
	}
}

func TestAssertGoldenWith_EachBranch(t *testing.T) {
	records := []Record{{Agent: "a"}, {Agent: "b"}}
	good, err := Encode(records)
	if err != nil {
		t.Fatal(err)
	}
	boom := func([]Record) ([]byte, error) { return nil, errors.New("encode boom") }

	t.Run("encode error is fatal", func(t *testing.T) {
		requireFatalF3(t, "encode records", t.TempDir(), func(tb testing.TB) {
			assertGoldenWith(tb, filepath.Join(t.TempDir(), "g"), records, false, boom)
		})
	})
	t.Run("update writes the encoded bytes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "g.jsonl")

		requireCleanF3(t, t.TempDir(), func(tb testing.TB) { AssertGoldenAt(tb, path, records, true) })

		if raw, err := os.ReadFile(path); err != nil || string(raw) != string(good) {
			t.Fatalf("golden = %q %v", raw, err)
		}
	})
	t.Run("update into a missing directory is fatal", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "nodir", "g.jsonl")

		requireFatalF3(t, "write golden", t.TempDir(), func(tb testing.TB) { AssertGoldenAt(tb, path, records, true) })
	})
	t.Run("a missing golden is fatal", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "absent.jsonl")

		requireFatalF3(t, "regenerate with -update", t.TempDir(), func(tb testing.TB) { AssertGoldenAt(tb, path, records, false) })
	})
	t.Run("a drifted golden names the first differing records", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "g.jsonl")
		fileAtF3(t, path, strings.Replace(string(good), `"agent":"b"`, `"agent":"z"`, 1))

		message, fatal := runUntilFatalF3(t.TempDir(), func(tb testing.TB) { AssertGoldenAt(tb, path, records, false) })

		if !fatal || !strings.Contains(message, `got: {"resolver":"","variant":"","agent":"b"}`) || !strings.Contains(message, `"agent":"z"`) {
			t.Fatalf("message = %q fatal=%v", message, fatal)
		}
	})
	t.Run("a matching golden passes", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "g.jsonl")
		fileAtF3(t, path, string(good))

		requireCleanF3(t, t.TempDir(), func(tb testing.TB) { AssertGoldenAt(tb, path, records, false) })
	})
}
