package acssuite

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func captureStderrF4(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stderr
	os.Stderr = w
	done := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	func() {
		defer func() {
			os.Stderr = orig
			w.Close()
		}()
		fn()
	}()
	return <-done
}

func TestResolveOptions_AbsFailureNamesTheOffendingRootAndStopsAtFirst(t *testing.T) {
	orig := resolveOptionsAbs
	t.Cleanup(func() { resolveOptionsAbs = orig })
	var seen []string
	resolveOptionsAbs = func(p string) (string, error) {
		seen = append(seen, p)
		return "", errors.New("no cwd")
	}

	_, err := resolveOptions(Options{Root: "rel", ProjectRoot: "other", Cycle: 1})
	if err == nil || !strings.Contains(err.Error(), `resolve "rel" to an absolute path: no cwd`) {
		t.Fatalf("want an absolute-path resolution error naming the root, got %v", err)
	}
	if len(seen) != 1 {
		t.Errorf("the first failing root must stop the loop; resolved %v", seen)
	}
}

func TestResolveOptions_AbsolutizesEveryNonEmptyRootAndLeavesEmptyOnesEmpty(t *testing.T) {
	root := t.TempDir()
	got, err := resolveOptions(Options{Root: root, Cycle: 2, GoModuleDir: filepath.Join(root, "x", "..", "mod")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Root != root || got.ProjectRoot != "" || got.GoModuleDir != filepath.Join(root, "mod") {
		t.Fatalf("roots = %q %q %q", got.Root, got.ProjectRoot, got.GoModuleDir)
	}
}

func TestWriteRedEvidence_StateDirBlockedByFileWarnsAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".evolve"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	rs := []Result{
		{ACID: "a/One", ResultStr: "red", fullEvidence: "E1"},
		{ACID: "a/Two", ResultStr: "red", fullEvidence: "E2"},
	}
	out := captureStderrF4(t, func() { writeRedEvidence(Options{Root: root, Cycle: 4}, rs) })
	if got := strings.Count(out, "red-evidence dir:"); got != 1 {
		t.Fatalf("want exactly one dir warning (the loop stops at the first failure), got %d in %q", got, out)
	}
}

func TestWriteRedEvidence_OpenFailureWarnsAndSkipsToNextResult(t *testing.T) {
	root := t.TempDir()
	orig := redEvidenceOpenFile
	t.Cleanup(func() { redEvidenceOpenFile = orig })
	attempts := 0
	redEvidenceOpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		attempts++
		if strings.HasSuffix(name, "a__One.txt") {
			return nil, errors.New("disk on fire")
		}
		return orig(name, flag, perm)
	}
	rs := []Result{
		{ACID: "a/One", ResultStr: "red", fullEvidence: "E1"},
		{ACID: "a/Two", ResultStr: "red", fullEvidence: "E2"},
	}
	out := captureStderrF4(t, func() { writeRedEvidence(Options{Root: root, Cycle: 4}, rs) })
	if !strings.Contains(out, "red-evidence write a__One.txt: disk on fire") {
		t.Errorf("open failure must be reported with the name; got %q", out)
	}
	dir := filepath.Join(root, ".evolve", "runs", "cycle-4", "acs-red-evidence")
	if b, err := os.ReadFile(filepath.Join(dir, "a__Two.txt")); err != nil || !strings.Contains(string(b), "E2") {
		t.Errorf("the next result must still be persisted: %v", err)
	}
	if attempts != 2 {
		t.Errorf("a non-collision failure must not retry other suffixes; attempts=%d", attempts)
	}
}

func TestWriteRedEvidence_WriteFailureOnReadOnlyHandleWarns(t *testing.T) {
	root := t.TempDir()
	orig := redEvidenceOpenFile
	t.Cleanup(func() { redEvidenceOpenFile = orig })
	redEvidenceOpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		if err := os.WriteFile(name, nil, perm); err != nil {
			return nil, err
		}
		return os.Open(name)
	}
	out := captureStderrF4(t, func() {
		writeRedEvidence(Options{Root: root, Cycle: 4}, []Result{{ACID: "a/One", ResultStr: "red", fullEvidence: "E1"}})
	})
	if !strings.Contains(out, "red-evidence write a__One.txt:") {
		t.Errorf("a failed write must be reported; got %q", out)
	}
}

func TestWriteRedEvidence_TenCollisionsDropEvidenceWithWarning(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".evolve", "runs", "cycle-4", "acs-red-evidence")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"a__One.txt"}
	for i := 2; i <= 10; i++ {
		names = append(names, fmt.Sprintf("a__One-%d.txt", i))
	}
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := captureStderrF4(t, func() {
		writeRedEvidence(Options{Root: root, Cycle: 4}, []Result{{ACID: "a/One", ResultStr: "red", fullEvidence: "NEW"}})
	})
	if !strings.Contains(out, "10 name collisions for a__One") {
		t.Errorf("collision exhaustion must warn; got %q", out)
	}
	for _, n := range names {
		if b, _ := os.ReadFile(filepath.Join(dir, n)); string(b) != "old" {
			t.Errorf("%s was overwritten: %q", n, b)
		}
	}
}

func TestChangedPackagesForCycle_FallsBackToBuilderHandoffName(t *testing.T) {
	root := t.TempDir()
	runDir := filepath.Join(root, ".evolve", "runs", "cycle-3")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		t.Fatal(err)
	}
	doc := `{"thrusts":[{"files_modified":["go/internal/foo/foo.go"]}]}`
	if err := os.WriteFile(filepath.Join(runDir, "handoff-builder.json"), []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	got := changedPackagesForCycle(root, 3)
	if len(got) != 1 || got[0] != "./internal/foo/..." {
		t.Fatalf("got %v", got)
	}
	if other := changedPackagesForCycle(root, 4); other != nil {
		t.Errorf("another cycle has no handoff: %v", other)
	}
	if empty := changedPackagesForCycle("", 3); empty != nil {
		t.Errorf("empty project root: %v", empty)
	}
}

func TestWriteVerdict_MarshalFailureIsWrappedAndLeavesNoTempFile(t *testing.T) {
	orig := writeVerdictMarshalIndent
	t.Cleanup(func() { writeVerdictMarshalIndent = orig })
	writeVerdictMarshalIndent = func(any, string, string) ([]byte, error) { return nil, errors.New("unmarshalable") }
	evolveDir := t.TempDir()

	path, err := WriteVerdict(evolveDir, Verdict{Cycle: 5})
	if err == nil || !strings.Contains(err.Error(), "acssuite: marshal: unmarshalable") || path != "" {
		t.Fatalf("path=%q err=%v", path, err)
	}
	entries, _ := filepath.Glob(filepath.Join(evolveDir, "runs", "cycle-5", "*"))
	if len(entries) != 0 {
		t.Errorf("nothing may be written when marshal fails: %v", entries)
	}
}

func TestLoadLaneConfig_MalformedPolicyWarnsAndFallsBackToDefaultBudget(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	var forward []string
	out := captureStderrF4(t, func() {
		cfg, refusals := loadLaneConfig(root)
		forward = cfg.PredicateEnv
		if len(refusals) != 0 {
			t.Errorf("no refusals expected: %v", refusals)
		}
	})
	if !strings.Contains(out, "the Go lane runs under the default budget") {
		t.Errorf("want the fallback warning, got %q", out)
	}
	if len(forward) != 0 {
		t.Errorf("a broken policy forwards no operator keys: %v", forward)
	}
}
