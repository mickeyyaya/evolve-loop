//go:build acs

package cycle1824

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/commentaudit"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const (
	summaryName       = "interaction-summary.json"
	fixedTempName     = summaryName + ".tmp"
	atomicwriteImport = "github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	rollupSource      = "go/internal/interaction/rollup.go"
)

func seedLedgers(t *testing.T, ws string) interaction.Summary {
	t.Helper()
	rec := interaction.NewRecorder(ws)
	rec.Record(interaction.Outcome{
		Event:   interaction.Event{Kind: interaction.KindNudge, Phase: "build", Trigger: "idle_no_artifact", DecisionID: "d-1"},
		Result:  interaction.ResultArtifactAppeared,
		CostUSD: 0.25,
	})
	rec.Record(interaction.Outcome{
		Event:  interaction.Event{Kind: interaction.KindAutoRespond, Phase: "scout", Trigger: "unknown_prompt", Rung: "kernel"},
		Result: interaction.ResultPromptCleared,
	})
	want, ok := interaction.Rollup(ws)
	if !ok {
		t.Fatalf("fixture ledgers in %s produced no rollup", ws)
	}
	return want
}

func readSummary(t *testing.T, ws string) interaction.Summary {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(ws, summaryName))
	if err != nil {
		t.Fatalf("reading %s: %v", summaryName, err)
	}
	var got interaction.Summary
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("%s is not one valid JSON summary (a torn or interleaved write): %v\n%s", summaryName, err, data)
	}
	return got
}

func strayTempEntries(t *testing.T, ws string, allowed ...string) []string {
	t.Helper()
	entries, err := os.ReadDir(ws)
	if err != nil {
		t.Fatalf("listing %s: %v", ws, err)
	}
	var stray []string
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp") && !slices.Contains(allowed, e.Name()) {
			stray = append(stray, e.Name())
		}
	}
	return stray
}

func TestC1824_001_ConcurrentWriteRollupCallersAllSucceedWithOneValidSummary(t *testing.T) {
	const iterations = 100
	const writers = 16
	for i := 0; i < iterations; i++ {
		ws := t.TempDir()
		want := seedLedgers(t, ws)
		start := make(chan struct{})
		errs := make([]error, writers)
		var wg sync.WaitGroup
		wg.Add(writers)
		for w := 0; w < writers; w++ {
			go func(idx int) {
				defer wg.Done()
				<-start
				errs[idx] = interaction.WriteRollup(ws)
			}(w)
		}
		close(start)
		wg.Wait()
		for w, err := range errs {
			if err != nil {
				t.Fatalf("iteration %d writer %d: WriteRollup failed while another writer ran (a shared temp path collided): %v", i, w, err)
			}
		}
		if got := readSummary(t, ws); !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d: summary after concurrent writers = %+v, want %+v", i, got, want)
		}
		if stray := strayTempEntries(t, ws); len(stray) > 0 {
			t.Fatalf("iteration %d: concurrent WriteRollup left temp files behind: %v", i, stray)
		}
	}
}

func TestC1824_002_WriteRollupNeverUsesTheFixedTempPath(t *testing.T) {
	t.Run("another writer's in-flight temp file is left untouched", func(t *testing.T) {
		ws := t.TempDir()
		want := seedLedgers(t, ws)
		inFlight := filepath.Join(ws, fixedTempName)
		otherWritersBytes := []byte(`{"in_flight":"another writer's rollup"}`)
		if err := os.WriteFile(inFlight, otherWritersBytes, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := interaction.WriteRollup(ws); err != nil {
			t.Fatalf("WriteRollup with another writer's temp file present: %v", err)
		}
		if got := readSummary(t, ws); !reflect.DeepEqual(got, want) {
			t.Errorf("summary = %+v, want %+v", got, want)
		}
		after, err := os.ReadFile(inFlight)
		if err != nil {
			t.Fatalf("WriteRollup consumed %s, which belongs to another writer: %v", fixedTempName, err)
		}
		if !bytes.Equal(after, otherWritersBytes) {
			t.Errorf("WriteRollup overwrote another writer's %s: got %q", fixedTempName, after)
		}
		if stray := strayTempEntries(t, ws, fixedTempName); len(stray) > 0 {
			t.Errorf("WriteRollup left its own temp files behind: %v", stray)
		}
	})

	t.Run("a directory squatting the fixed temp name does not block the write", func(t *testing.T) {
		ws := t.TempDir()
		want := seedLedgers(t, ws)
		if err := os.Mkdir(filepath.Join(ws, fixedTempName), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := interaction.WriteRollup(ws); err != nil {
			t.Fatalf("WriteRollup failed because %s was occupied: %v", fixedTempName, err)
		}
		if got := readSummary(t, ws); !reflect.DeepEqual(got, want) {
			t.Errorf("summary = %+v, want %+v", got, want)
		}
	})

	t.Run("rollup.go writes through atomicwrite and carries no bespoke temp-and-rename", func(t *testing.T) {
		path := filepath.Join(acsassert.RepoRoot(t), rollupSource)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", rollupSource, err)
		}
		imported := slices.ContainsFunc(file.Imports, func(spec *ast.ImportSpec) bool {
			p, _ := strconv.Unquote(spec.Path.Value)
			return p == atomicwriteImport
		})
		if !imported {
			t.Errorf("%s does not import %s; WriteRollup must use the single atomic-write path", rollupSource, atomicwriteImport)
		}
		var bespoke []string
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && slices.Contains([]string{"WriteFile", "Rename", "CreateTemp", "OpenFile", "Create"}, sel.Sel.Name) {
				bespoke = append(bespoke, "os."+sel.Sel.Name)
			}
			return true
		})
		if len(bespoke) > 0 {
			t.Errorf("%s still writes files itself via %v instead of atomicwrite", rollupSource, bespoke)
		}
	})
}

func TestC1824_003_AFailedWriteReturnsTheErrorAndLeavesNoTempFile(t *testing.T) {
	ws := t.TempDir()
	seedLedgers(t, ws)
	summaryBlockedByDirectory := filepath.Join(ws, summaryName)
	if err := os.MkdirAll(filepath.Join(summaryBlockedByDirectory, "occupant"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := interaction.WriteRollup(ws); err == nil {
		t.Fatalf("WriteRollup returned nil although %s is a non-empty directory; the rename failure was swallowed", summaryName)
	}
	if stray := strayTempEntries(t, ws); len(stray) > 0 {
		t.Errorf("a failed WriteRollup left temp files behind: %v", stray)
	}
	if _, ok := interaction.Rollup(ws); !ok {
		t.Errorf("the ledgers were lost after a failed WriteRollup")
	}

	emptyWorkspace := t.TempDir()
	if err := interaction.WriteRollup(emptyWorkspace); err != nil {
		t.Fatalf("WriteRollup on a workspace with nothing to summarize: %v", err)
	}
	entries, err := os.ReadDir(emptyWorkspace)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("a workspace with nothing to summarize must get no file, found %v", entries)
	}
}

func TestC1824_004_ConcurrentRecordCallsWriteWholeLedgerLines(t *testing.T) {
	const goroutines = 16
	const perGoroutine = 50
	ws := t.TempDir()
	rec := interaction.NewRecorder(ws)
	longRuleID := strings.Repeat("r", 8192)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer wg.Done()
			<-start
			for s := 0; s < perGoroutine; s++ {
				rec.Record(interaction.Outcome{
					Event: interaction.Event{
						Kind:       interaction.KindNudge,
						Phase:      "build",
						Trigger:    fmt.Sprintf("g%d-s%d", g, s),
						DecisionID: fmt.Sprintf("d-%d-%d", g, s),
						RuleID:     longRuleID,
					},
					Result: interaction.ResultNoEffect,
				})
			}
		}(g)
	}
	close(start)
	wg.Wait()

	f, err := os.Open(filepath.Join(ws, "build-interactions.ndjson"))
	if err != nil {
		t.Fatalf("opening the build ledger: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	seen := map[string]int{}
	lines := 0
	for scanner.Scan() {
		lines++
		var out interaction.Outcome
		if err := json.Unmarshal(scanner.Bytes(), &out); err != nil {
			t.Fatalf("ledger line %d is not one whole Outcome (interleaved append): %v", lines, err)
		}
		if out.RuleID != longRuleID {
			t.Fatalf("ledger line %d carries a torn rule_id of %d bytes", lines, len(out.RuleID))
		}
		seen[out.Trigger]++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning the ledger: %v", err)
	}
	if lines != goroutines*perGoroutine {
		t.Errorf("ledger holds %d lines, want %d", lines, goroutines*perGoroutine)
	}
	for g := 0; g < goroutines; g++ {
		for s := 0; s < perGoroutine; s++ {
			if key := fmt.Sprintf("g%d-s%d", g, s); seen[key] != 1 {
				t.Errorf("record %s appears %d times in the ledger, want exactly once", key, seen[key])
			}
		}
	}
	if got := len(rec.Outcomes()); got != goroutines*perGoroutine {
		t.Errorf("Recorder kept %d outcomes in memory, want %d", got, goroutines*perGoroutine)
	}
}

type rootedGit struct{ root string }

func (g rootedGit) run(args ...string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("git", append([]string{"-C", g.root}, args...)...)
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

func (g rootedGit) ChangedFiles(base string) ([]string, error) {
	tracked, err := g.run("diff", "--name-only", "--no-renames", base)
	if err != nil {
		return nil, err
	}
	untracked, err := g.run("ls-files", "--others", "--exclude-standard", "--full-name", ":/")
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(tracked) + "\n" + string(untracked)), nil
}

func (g rootedGit) Show(base, path string) ([]byte, error) {
	return commentaudit.ReadAtBase(g.run, base)(path)
}

func (g rootedGit) Root() (string, error) {
	out, err := g.run("rev-parse", "--show-toplevel")
	return strings.TrimSpace(string(out)), err
}

func TestC1824_005_TheFixLandsInRollupAndAddsNoCommentsToTheInteractionPackage(t *testing.T) {
	root := acsassert.RepoRoot(t)
	g := rootedGit{root: root}
	out, err := g.run("merge-base", "main", "HEAD")
	if err != nil {
		t.Fatalf("resolving the cycle's base against main: %v", err)
	}
	base := strings.TrimSpace(string(out))
	changed, err := g.ChangedFiles(base)
	if err != nil {
		t.Fatalf("listing the cycle's changed files: %v", err)
	}
	if !slices.Contains(changed, rollupSource) {
		t.Errorf("%s is unchanged since %s; the atomic-write fix has not landed", rollupSource, base)
	}

	var stdout, stderr bytes.Buffer
	interactionPackage := filepath.Join(root, "go", "internal", "interaction")
	if code := commentaudit.Main([]string{"comments", "-base", base, interactionPackage}, &stdout, &stderr, g); code != 0 {
		t.Errorf("commentaudit comments -base %s %s exit=%d (want 0: the change adds no comments)\nstdout:\n%s\nstderr:\n%s", base, interactionPackage, code, stdout.String(), stderr.String())
	}
}
