package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func e2LandArchive(t *testing.T, headers []tar.Header, bodies []string) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i := range headers {
		h := headers[i]
		if h.Typeflag == tar.TypeReg {
			h.Size = int64(len(bodies[i]))
		}
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeReg {
			if _, err := tw.Write([]byte(bodies[i])); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "untracked.tgz")
	writeE2File(t, path, buf.String())
	return path
}

func TestParseLandArgs_AcceptsExactlyOneInput(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		want    landArgs
		wantErr string
	}{
		{"a patch", []string{"--branch", "b", "--patch", "p"}, landArgs{branch: "b", patch: "p"}, ""},
		{"a salvage leaf with a root", []string{"--salvage", "leaf", "--branch", "fix/x", "--project-root", "/r"}, landArgs{branch: "fix/x", salvage: "leaf", projectRoot: "/r"}, ""},
		{"an operand", []string{"--branch", "b", "--patch", "p", "extra"}, landArgs{}, `unexpected argument "extra"`},
		{"no branch", []string{"--patch", "p"}, landArgs{}, "--branch is required"},
		{"neither input", []string{"--branch", "b"}, landArgs{}, "name exactly one of --patch or --salvage"},
		{"both inputs", []string{"--branch", "b", "--patch", "p", "--salvage", "s"}, landArgs{}, "name exactly one of --patch or --salvage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLandArgs(tc.args)

			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Fatalf("err=%v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %+v err=%v, want %+v", got, err, tc.want)
			}
		})
	}
}

func TestParseLandArgs_AFlagParseErrorIsReturned(t *testing.T) {
	_, err := parseLandArgs([]string{"--branch"})

	if err == nil {
		t.Fatal("a flag without its value must be refused")
	}
}

func TestRegularFile_ResolvesOnlyRegularFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "p.patch")
	writeE2File(t, file, "x")
	chdirForTest(t, dir)

	got, err := regularFile("p.patch")
	if err != nil || !filepath.IsAbs(got) || filepath.Base(got) != "p.patch" {
		t.Fatalf("a relative regular file: got %q err=%v", got, err)
	}
	if _, err := regularFile(dir); err == nil || err.Error() != "not a regular file" {
		t.Fatalf("a directory: err=%v", err)
	}
	if _, err := regularFile(filepath.Join(dir, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing path: err=%v", err)
	}
}

func TestSalvageLeafDir_LooksAtThePathThenTheOperatorSalvageDir(t *testing.T) {
	root := t.TempDir()
	named := filepath.Join(root, ".evolve", "operator-salvage", "leaf-1")
	if err := os.MkdirAll(named, 0o755); err != nil {
		t.Fatal(err)
	}
	direct := filepath.Join(t.TempDir(), "direct")
	if err := os.MkdirAll(direct, 0o755); err != nil {
		t.Fatal(err)
	}
	asFile := filepath.Join(root, ".evolve", "operator-salvage", "a-file")
	writeE2File(t, asFile, "x")

	if got, err := salvageLeafDir("leaf-1", root); err != nil || got != named {
		t.Fatalf("a bare leaf name: got %q err=%v, want %q", got, err, named)
	}
	if got, err := salvageLeafDir(direct, root); err != nil || got != direct {
		t.Fatalf("a path: got %q err=%v", got, err)
	}
	_, err := salvageLeafDir("a-file", root)
	if err == nil || !strings.Contains(err.Error(), "salvage leaf a-file: no such directory (looked in a-file, "+asFile+")") {
		t.Fatalf("a file is not a leaf: err=%v", err)
	}
	_, err = salvageLeafDir("x/missing", root)
	if err == nil || err.Error() != "salvage leaf x/missing: no such directory (looked in x/missing)" {
		t.Fatalf("a path never falls back to the salvage dir: err=%v", err)
	}
}

func TestResolveLandInput_Table(t *testing.T) {
	root := t.TempDir()
	salvage := filepath.Join(root, ".evolve", "operator-salvage")
	leaf := func(name string, files map[string]bool) string {
		dir := filepath.Join(salvage, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f, isDir := range files {
			if isDir {
				if err := os.MkdirAll(filepath.Join(dir, f), 0o755); err != nil {
					t.Fatal(err)
				}
				continue
			}
			writeE2File(t, filepath.Join(dir, f), "x")
		}
		return dir
	}
	both := leaf("both", map[string]bool{"uncommitted.patch": false, "untracked.tgz": false})
	onlyTGZ := leaf("only-tgz", map[string]bool{"untracked.tgz": false})
	leaf("empty", nil)
	leaf("patch-dir", map[string]bool{"uncommitted.patch": true})
	leaf("tgz-dir", map[string]bool{"untracked.tgz": true})
	patch := filepath.Join(root, "x.patch")
	writeE2File(t, patch, "p")
	cases := []struct {
		name    string
		a       landArgs
		want    landInput
		wantErr string
	}{
		{"a patch", landArgs{patch: patch}, landInput{patch: patch}, ""},
		{"a missing patch", landArgs{patch: patch + ".absent"}, landInput{}, "patch " + patch + ".absent: stat "},
		{"a directory patch", landArgs{patch: root}, landInput{}, "patch " + root + ": not a regular file"},
		{"both salvage files", landArgs{salvage: "both"}, landInput{patch: filepath.Join(both, "uncommitted.patch"), untracked: filepath.Join(both, "untracked.tgz")}, ""},
		{"only the archive", landArgs{salvage: "only-tgz"}, landInput{untracked: filepath.Join(onlyTGZ, "untracked.tgz")}, ""},
		{"an empty leaf", landArgs{salvage: "empty"}, landInput{}, "salvage leaf empty holds neither uncommitted.patch nor untracked.tgz"},
		{"a missing leaf", landArgs{salvage: "absent"}, landInput{}, "salvage leaf absent: no such directory"},
		{"a directory patch in the leaf", landArgs{salvage: "patch-dir"}, landInput{}, "salvage leaf patch-dir: not a regular file"},
		{"a directory archive in the leaf", landArgs{salvage: "tgz-dir"}, landInput{}, "salvage leaf tgz-dir: not a regular file"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveLandInput(tc.a, root)

			if tc.wantErr != "" {
				if err == nil || !strings.HasPrefix(err.Error(), tc.wantErr) || got != (landInput{}) {
					t.Fatalf("got %+v err=%v, want %q", got, err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %+v err=%v, want %+v", got, err, tc.want)
			}
		})
	}
}

func TestExtractUntracked_RestoresDirsAndFilesWithTheirModes(t *testing.T) {
	archive := e2LandArchive(t, []tar.Header{
		{Name: "pkg/", Typeflag: tar.TypeDir, Mode: 0o755},
		{Name: "pkg/run.sh", Typeflag: tar.TypeReg, Mode: 0o755},
		{Name: "deep/nested/note.txt", Typeflag: tar.TypeReg, Mode: 0o600},
		{Name: "empty.txt", Typeflag: tar.TypeReg, Mode: 0o644},
	}, []string{"", "#!/bin/sh\n", "ünïcode\n", ""})
	dir := t.TempDir()

	if err := extractUntracked(archive, dir); err != nil {
		t.Fatal(err)
	}

	for rel, want := range map[string]string{"pkg/run.sh": "#!/bin/sh\n", "deep/nested/note.txt": "ünïcode\n", "empty.txt": ""} {
		got, err := os.ReadFile(filepath.Join(dir, rel))
		if err != nil || string(got) != want {
			t.Fatalf("%s = %q err=%v, want %q", rel, got, err, want)
		}
	}
	info, err := os.Stat(filepath.Join(dir, "deep/nested/note.txt"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v err=%v, want 0600", info.Mode().Perm(), err)
	}
}

func TestExtractUntracked_FailsOnEachBrokenLayer(t *testing.T) {
	dir := t.TempDir()
	notGzip := filepath.Join(dir, "plain.tgz")
	writeE2File(t, notGzip, "plain text")
	var gzGarbage bytes.Buffer
	gz := gzip.NewWriter(&gzGarbage)
	if _, err := gz.Write(bytes.Repeat([]byte{0xff}, 1024)); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	notTar := filepath.Join(dir, "garbage.tgz")
	writeE2File(t, notTar, gzGarbage.String())
	link := e2LandArchive(t, []tar.Header{{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}}, []string{""})
	cases := []struct {
		name    string
		archive string
		wantErr string
	}{
		{"a missing archive", filepath.Join(dir, "absent.tgz"), "no such file or directory"},
		{"not gzip", notGzip, "gzip: invalid header"},
		{"not tar", notTar, "archive/tar: invalid tar header"},
		{"a symlink entry", link, `link: unsupported entry type '2'; restore it by hand`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := t.TempDir()

			err := extractUntracked(tc.archive, out)

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want %q", err, tc.wantErr)
			}
			if _, lerr := os.Lstat(filepath.Join(out, "link")); lerr == nil {
				t.Fatal("an unsupported entry was restored")
			}
		})
	}
}

func TestRestoreEntry_RefusesWhatItCannotPlace(t *testing.T) {
	dir := t.TempDir()
	writeE2File(t, filepath.Join(dir, "blocker"), "a file where a dir must go")
	cases := []struct {
		name    string
		hdr     tar.Header
		wantErr string
	}{
		{"a parent that is a file", tar.Header{Name: "blocker/child.txt", Typeflag: tar.TypeReg, Mode: 0o644}, "not a directory"},
		{"a dir under a file", tar.Header{Name: "blocker/sub/", Typeflag: tar.TypeDir, Mode: 0o755}, "not a directory"},
		{"a hard link", tar.Header{Name: "hard", Typeflag: tar.TypeLink}, "unsupported entry type '1'"},
		{"a dot-dot escape", tar.Header{Name: "a/../../x", Typeflag: tar.TypeReg}, "entry escapes the worktree"},
		{"an empty name", tar.Header{Name: "", Typeflag: tar.TypeReg}, "entry escapes the worktree"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hdr := tc.hdr

			err := restoreEntry(strings.NewReader(""), &hdr, dir)

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err=%v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestRestoreEntry_ADirEntryIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	hdr := tar.Header{Name: "a/b/", Typeflag: tar.TypeDir, Mode: 0o755}

	for i := 0; i < 2; i++ {
		if err := restoreEntry(strings.NewReader(""), &hdr, dir); err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
	}
	if info, err := os.Stat(filepath.Join(dir, "a", "b")); err != nil || !info.IsDir() {
		t.Fatalf("a/b not restored as a dir: %v", err)
	}
}

func TestRestoreEntry_ACopyFailureIsReturnedAndTheFileIsClosed(t *testing.T) {
	dir := t.TempDir()
	hdr := tar.Header{Name: "f.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 4}

	err := restoreEntry(errReaderE2{}, &hdr, dir)

	if err == nil || !strings.Contains(err.Error(), "stdin torn") {
		t.Fatalf("err=%v, want the read failure", err)
	}
}

func TestLandInto_AFailedArchiveIsAnIOErrorEvenAfterACleanPatch(t *testing.T) {
	dir := t.TempDir()
	emptyPatch := filepath.Join(t.TempDir(), "empty.patch")
	writeE2File(t, emptyPatch, "")
	var stdout, stderr bytes.Buffer

	code := landInto(dir, landInput{patch: emptyPatch, untracked: filepath.Join(dir, "absent.tgz")}, &stdout, &stderr)

	if code != exitIO || !strings.HasPrefix(stderr.String(), landPrefix+"restore untracked files into "+dir+": open ") || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestLandInto_AnArchiveAloneLandsAndPointsAtTheCommitGate(t *testing.T) {
	dir := t.TempDir()
	archive := e2LandArchive(t, []tar.Header{{Name: "new.txt", Typeflag: tar.TypeReg, Mode: 0o644}}, []string{"new\n"})
	var stdout, stderr bytes.Buffer

	code := landInto(dir, landInput{untracked: archive}, &stdout, &stderr)

	want := "land: landed uncommitted on origin/main in " + dir + "; nothing was committed or pushed\n" +
		"land: next: cd " + dir + ", review the diff, commit through the commit gate (/commit), then evolve ship --class manual\n"
	if code != 0 || stdout.String() != want {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if got, err := os.ReadFile(filepath.Join(dir, "new.txt")); err != nil || string(got) != "new\n" {
		t.Fatalf("new.txt = %q err=%v", got, err)
	}
}

func TestApplyLandPatch_AnEmptyPatchIsANoOpEvenOutsideARepo(t *testing.T) {
	patch := filepath.Join(t.TempDir(), "empty.patch")
	writeE2File(t, patch, "")
	var stderr bytes.Buffer

	if code := applyLandPatch(filepath.Join(t.TempDir(), "absent"), patch, &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunLand_AnUnresolvableRootIsAnIOError(t *testing.T) {
	t.Setenv("EVOLVE_PROJECT_ROOT", "")
	e2StubProcessGetwd(t, errors.New("getwd: gone"))
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "b", "--patch", "p"}, nil, &stdout, &stderr)

	if code != exitIO || stderr.String() != landPrefix+"getwd: gone\n" {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}

func TestRunLand_AMissingPatchIsAnIOErrorBeforeAnyWorktree(t *testing.T) {
	root := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "b", "--patch", filepath.Join(root, "absent.patch"), "--project-root", root}, nil, &stdout, &stderr)

	if code != exitIO || !strings.HasPrefix(stderr.String(), landPrefix+"patch "+filepath.Join(root, "absent.patch")+": stat ") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "dev")); err == nil {
		t.Fatal("a refused land created a dev dir")
	}
}

func TestRunLand_ARootOutsideAHubIsRefusedByTheWorktreeStep(t *testing.T) {
	root := t.TempDir()
	patch := filepath.Join(root, "x.patch")
	writeE2File(t, patch, "")
	var stdout, stderr bytes.Buffer

	code := runLand([]string{"--branch", "b", "--patch", patch, "--project-root", root}, nil, &stdout, &stderr)

	if code != 1 || !strings.Contains(stderr.String(), "evolve worktree create --dev: refused:") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
