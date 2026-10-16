//go:build acs

package cycle1883

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	buildOnce sync.Once
	builtBin  string
	buildOut  string
	buildErr  error
)

func evolveBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "evolve-acs-1883-")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "evolve")
		cmd := exec.Command("go", "build", "-o", builtBin, "./cmd/evolve")
		cmd.Dir = filepath.Join(acsassert.RepoRoot(t), "go")
		out, err := cmd.CombinedOutput()
		buildOut, buildErr = string(out), err
	})
	if buildErr != nil {
		t.Fatalf("go build ./cmd/evolve failed: %v\n%s", buildErr, buildOut)
	}
	return builtBin
}

type result struct {
	stdout string
	stderr string
	code   int
}

func runEvolve(t *testing.T, projectEnv string, args ...string) result {
	t.Helper()
	cmd := exec.Command(evolveBinary(t), args...)
	cmd.Dir = t.TempDir()
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "EVOLVE_") {
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	if projectEnv != "" {
		cmd.Env = append(cmd.Env, "EVOLVE_PROJECT_ROOT="+projectEnv)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("evolve did not run: %v", err)
		}
		code = ee.ExitCode()
	}
	return result{stdout.String(), stderr.String(), code}
}

const lessonTemplate = `- id: cycle-%CYCLE%-sample
  pattern: %PATTERN%
  failureContext:
    errorCategory: infrastructure
`

func writeLesson(t *testing.T, root, file, cycle, pattern string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "instincts", "lessons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.NewReplacer("%CYCLE%", cycle, "%PATTERN%", pattern).Replace(lessonTemplate)
	if err := os.WriteFile(filepath.Join(dir, file), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeLesson(t, root, "cycle-1-a.yaml", "1", "recurring-defect-alpha")
	writeLesson(t, root, "cycle-2-a.yaml", "2", "recurring-defect-alpha")
	writeLesson(t, root, "cycle-3-a.yaml", "3", "recurring-defect-alpha")
	writeLesson(t, root, "cycle-4-b.yaml", "4", "rare-defect-beta")
	return root
}

func ledgerPath(root string) string {
	return filepath.Join(root, ".evolve", "recurrence-ledger.json")
}

func TestC1883_001_DryRunPrintsCountsAndWritesNothing(t *testing.T) {
	root := fixtureProject(t)
	res := runEvolve(t, "", "lessons", "backfill", "--project-root", root)
	if res.code != 0 {
		t.Fatalf("RED: backfill dry-run exit=%d stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "3  recurring-defect-alpha") {
		t.Errorf("alpha count 3 missing from output:\n%s", res.stdout)
	}
	if !strings.Contains(res.stdout, "1  rare-defect-beta") {
		t.Errorf("beta count 1 missing from output:\n%s", res.stdout)
	}
	if _, err := os.Stat(ledgerPath(root)); err == nil {
		t.Errorf("dry-run created %s", ledgerPath(root))
	}
}

func TestC1883_002_WriteReplacesLedgerAndRecurrenceRendersSameCounts(t *testing.T) {
	root := fixtureProject(t)
	staleLedger := []byte(`{"entries":{"stale-pattern":{"pattern":"stale-pattern","count":9}}}`)
	if err := os.MkdirAll(filepath.Dir(ledgerPath(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledgerPath(root), staleLedger, 0o644); err != nil {
		t.Fatal(err)
	}
	runEvolve(t, "", "lessons", "backfill", "--project-root", root)
	if got, _ := os.ReadFile(ledgerPath(root)); !bytes.Equal(got, staleLedger) {
		t.Fatalf("dry-run mutated the existing ledger: %s", got)
	}
	write := runEvolve(t, "", "lessons", "backfill", "--write", "--project-root", root)
	if write.code != 0 {
		t.Fatalf("RED: backfill --write exit=%d stderr=%q", write.code, write.stderr)
	}
	rec := runEvolve(t, root, "lessons", "recurrence")
	if rec.code != 0 {
		t.Fatalf("lessons recurrence exit=%d stderr=%q", rec.code, rec.stderr)
	}
	for _, row := range []string{"3  recurring-defect-alpha", "1  rare-defect-beta"} {
		if !strings.Contains(rec.stdout, row) {
			t.Errorf("recurrence report lacks %q after --write:\n%s", row, rec.stdout)
		}
	}
	if strings.Contains(rec.stdout, "stale-pattern") {
		t.Errorf("--write did not replace the stale ledger:\n%s", rec.stdout)
	}
}

func TestC1883_003_SkippedMalformedFilesAreListed(t *testing.T) {
	root := fixtureProject(t)
	dir := filepath.Join(root, ".evolve", "instincts", "lessons")
	if err := os.WriteFile(filepath.Join(dir, "broken-lesson.yaml"), []byte("::: not [valid yaml\n\t- {"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "empty-lesson.yaml"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	res := runEvolve(t, "", "lessons", "backfill", "--project-root", root)
	if res.code != 0 {
		t.Fatalf("RED: backfill exit=%d stderr=%q", res.code, res.stderr)
	}
	for _, name := range []string{"broken-lesson.yaml", "empty-lesson.yaml"} {
		if !strings.Contains(res.stdout, "skipped: "+name) {
			t.Errorf("skipped listing lacks %s:\n%s", name, res.stdout)
		}
	}
	if strings.Contains(res.stdout, "skipped: cycle-1-a.yaml") {
		t.Errorf("a valid lesson was reported as skipped:\n%s", res.stdout)
	}
}

func TestC1883_004_MissingLessonsDirectoryExitsTwo(t *testing.T) {
	emptyProject := t.TempDir()
	res := runEvolve(t, "", "lessons", "backfill", "--write", "--project-root", emptyProject)
	if res.code != 2 {
		t.Errorf("RED: missing lessons dir exit=%d want 2 (stderr=%q)", res.code, res.stderr)
	}
	if res.stderr == "" {
		t.Errorf("exit 2 carried no diagnostic on stderr")
	}
	if _, err := os.Stat(ledgerPath(emptyProject)); err == nil {
		t.Errorf("failed backfill still wrote a ledger")
	}
}

func TestC1883_005_EmptyCorpusExitsZeroWithoutPatternRows(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "instincts", "lessons"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := runEvolve(t, "", "lessons", "backfill", "--project-root", root)
	if res.code != 0 {
		t.Errorf("RED: empty corpus exit=%d stderr=%q", res.code, res.stderr)
	}
	if !strings.Contains(res.stdout, "ledger is empty") {
		t.Errorf("empty corpus not reported:\n%s", res.stdout)
	}
}

func TestC1883_006_UnknownUsageNamesBackfill(t *testing.T) {
	res := runEvolve(t, "", "lessons", "no-such-subcommand")
	if res.code != 10 {
		t.Errorf("unknown subcommand exit=%d want 10", res.code)
	}
	if !strings.Contains(res.stderr, "backfill") {
		t.Errorf("usage text does not list backfill: %q", res.stderr)
	}
}

func TestC1883_007_OperatorCommandDocumented(t *testing.T) {
	// acs-predicate: config-check
	root := acsassert.RepoRoot(t)
	doc := filepath.Join(root, "docs", "operations", "runtime-reference.md")
	if !acsassert.FileContains(t, doc, "evolve lessons backfill") {
		t.Errorf("runtime-reference.md does not document evolve lessons backfill")
	}
}

func TestC1883_008_BackfillHandlerCarriesNoComments(t *testing.T) {
	root := acsassert.RepoRoot(t)
	src := filepath.Join(root, "go", "cmd", "evolve", "cmd_lessons.go")
	n, err := acsassert.CountInGoFunc(src, "runLessonsBackfill", "//", "/*")
	if err != nil {
		t.Fatalf("RED: runLessonsBackfill not found: %v", err)
	}
	if n != 0 {
		t.Errorf("runLessonsBackfill carries %d comment marker(s)", n)
	}
}
