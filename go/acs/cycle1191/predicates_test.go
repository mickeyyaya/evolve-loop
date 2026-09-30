//go:build acs

package cycle1191

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/looppreflight"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const forgedPrev = "dead000000000000000000000000000000000000000000000000000000000beef"

const liveSealLabelPrefix = "reset-seal-"

func lineSHA(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

type fixtureLine struct {
	entry     core.LedgerEntry
	breakPrev bool
}

func phaseLine(kind string) fixtureLine {
	return fixtureLine{entry: core.LedgerEntry{
		TS: "2026-07-29T00:00:00Z", Role: "orchestrator", Kind: kind,
	}}
}

func brokenLine(kind string) fixtureLine {
	l := phaseLine(kind)
	l.breakPrev = true
	return l
}

func liveShapeSeal(label string) fixtureLine {
	return fixtureLine{entry: core.LedgerEntry{
		TS: "2026-07-29T00:00:00Z", Role: "operator", Kind: "reset",
		CycleLabel: liveSealLabelPrefix + label,
	}}
}

func dualMarkerSeal(label string) fixtureLine {
	return fixtureLine{entry: core.LedgerEntry{
		TS: "2026-07-29T00:00:00Z", Role: "operator",
		Kind:       liveSealLabelPrefix + label,
		CycleLabel: liveSealLabelPrefix + label,
	}}
}

func writeFixtureLedger(t *testing.T, rows []fixtureLine) string {
	t.Helper()
	dir := t.TempDir()

	var buf strings.Builder
	prevSHA := ""
	lastSeq := 0
	for i, row := range rows {
		e := row.entry
		e.EntrySeq = i
		switch {
		case row.breakPrev:
			e.PrevHash = forgedPrev
		case i == 0:
			e.PrevHash = ledger.ZeroSeed
		default:
			e.PrevHash = prevSHA
		}
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatalf("marshal fixture row %d: %v", i, err)
		}
		buf.Write(raw)
		buf.WriteByte('\n')
		prevSHA = lineSHA(raw)
		lastSeq = e.EntrySeq
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), []byte(buf.String()), 0o644); err != nil {
		t.Fatalf("write fixture ledger: %v", err)
	}
	tip := fmt.Sprintf("%d:%s", lastSeq, prevSHA)
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write fixture tip: %v", err)
	}
	return dir
}

func verifyFixture(t *testing.T, rows []fixtureLine) error {
	t.Helper()
	return ledger.New(writeFixtureLedger(t, rows)).Verify(context.Background())
}

func stateRoot(t *testing.T) string {
	t.Helper()
	if r := os.Getenv("EVOLVE_PROJECT_ROOT"); r != "" {
		return r
	}
	return acsassert.RepoRoot(t)
}

func TestC1191_001_seal_anchor_resolves_production_seal_shape(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		phaseLine("phase_end"),
		brokenLine("rewritten_history"),
		liveShapeSeal("cycle-108"),
		phaseLine("phase_start"),
		phaseLine("phase_end"),
	})
	if err != nil {
		t.Errorf("break→seal→valid-chain must verify OK once the walk anchors at the last operator reset-seal; got BROKEN: %v\n"+
			"the seal marker in the real ledger is cycle_label=%q with kind=%q (.evolve/ledger.jsonl:1880), "+
			"not a kind prefix — effectiveAnchorSHA must resolve the production shape", err,
			liveSealLabelPrefix+"cycle-108", "reset")
	}
}

func TestC1191_002_self_invalid_seal_must_not_anchor(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		phaseLine("phase_end"),
		brokenLine("rewritten_history"),
		func() fixtureLine { s := dualMarkerSeal("cycle-forged"); s.breakPrev = true; return s }(),
		phaseLine("phase_start"),
		phaseLine("phase_end"),
	})
	if err == nil {
		t.Error("a reset-seal whose OWN prev_hash does not chain from its predecessor must NOT anchor the walk — " +
			"verify returned OK, so a forged seal can silence the entire prefix (inbox Guard)")
		return
	}
	if !errors.Is(err, core.ErrLedgerChainBroken) {
		t.Errorf("self-invalid seal must fail as a chain break (ErrLedgerChainBroken); got %v", err)
	}
}

func TestC1191_003_break_after_last_seal_still_broken(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		phaseLine("phase_end"),
		liveShapeSeal("cycle-111"),
		phaseLine("phase_start"),
		brokenLine("post_seal_tamper"),
		phaseLine("phase_end"),
	})
	if err == nil {
		t.Error("a chain break AFTER the last reset-seal must still report BROKEN — verify returned OK, " +
			"so the seal is being read as 'stop verifying' instead of 'verify from here forward'")
	}
}

func TestC1191_004_two_seals_chain_normally(t *testing.T) {
	err := verifyFixture(t, []fixtureLine{
		phaseLine("phase_start"),
		brokenLine("rewritten_history"),
		liveShapeSeal("cycle-108"),
		phaseLine("phase_start"),
		liveShapeSeal("cycle-111"),
		phaseLine("phase_end"),
	})
	if err != nil {
		t.Errorf("two chained operator seals must resolve to the LATER anchor with the damaged prefix preserved-but-untrusted; got BROKEN: %v", err)
	}
}

func TestC1191_005_live_ledger_no_1740_wolf_cry(t *testing.T) {
	evolveDir := filepath.Join(stateRoot(t), ".evolve")
	if _, err := os.Stat(filepath.Join(evolveDir, "ledger.jsonl")); err != nil {
		t.Skipf("no live ledger at %s: %v", evolveDir, err)
	}
	err := ledger.New(evolveDir).Verify(context.Background())
	if err == nil {
		return
	}
	if strings.Contains(err.Error(), "line 1740") {
		t.Errorf("live `ledger verify` still cries wolf on the sealed line-1740 damage: %v\n"+
			"the walk must anchor at the LAST operator reset-seal (197 such entries exist in this ledger) "+
			"and report the post-seal chain status instead", err)
	}
}

const followTestFile = "go/cmd/evolve/cmd_bridge_watch_test.go"

var observingFollowTests = []string{
	"TestRunBridgeWatchFollow_SkipsMalformedAndEmptyLines",
	"TestRunBridgeWatchFollow_TailsNewLines",
}

const minFollowDeadline = 10

var (
	secondsDeadlineRe = regexp.MustCompile(`WithTimeout\([^,]+,\s*(\d+)\s*\*\s*time\.Second\s*\)`)
	msSleepRe         = regexp.MustCompile(`time\.Sleep\([^)]*time\.Millisecond`)
)

func goFuncBody(path, funcName string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, raw, 0)
	if err != nil {
		return "", fmt.Errorf("parse %s: %w", path, err)
	}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != funcName {
			continue
		}
		return string(raw[fset.Position(fd.Pos()).Offset:fset.Position(fd.End()).Offset]), nil
	}
	return "", fmt.Errorf("function %q not found in %s", funcName, path)
}

func TestC1191_006_follow_tests_race_clean_under_repetition(t *testing.T) {
	root := acsassert.RepoRoot(t)
	cmd := exec.Command("go", "test", "-race", "-count=25",
		"-run", "TestRunBridgeWatchFollow", "./cmd/evolve/")
	cmd.Dir = filepath.Join(root, "go")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("go test -race -count=25 -run TestRunBridgeWatchFollow ./cmd/evolve/ must be green: %v\n%s", err, out)
	}
}

func TestC1191_007_follow_waits_are_event_driven_with_long_deadline(t *testing.T) {
	path := filepath.Join(acsassert.RepoRoot(t), followTestFile)
	for _, fn := range observingFollowTests {
		body, err := goFuncBody(path, fn)
		if err != nil {
			t.Errorf("%s: %v", fn, err)
			continue
		}
		m := secondsDeadlineRe.FindStringSubmatch(body)
		if m == nil {
			t.Errorf("%s: no seconds-scale context deadline found — a follow test that must OBSERVE an appended "+
				"line needs an event wait bounded by a deadline >= %ds, not a millisecond window that a loaded "+
				"macOS runner can miss", fn, minFollowDeadline)
			continue
		}
		secs, convErr := strconv.Atoi(m[1])
		if convErr != nil {
			t.Errorf("%s: unparsable deadline literal %q: %v", fn, m[1], convErr)
			continue
		}
		if secs < minFollowDeadline {
			t.Errorf("%s: context deadline is %ds, want >= %ds", fn, secs, minFollowDeadline)
		}
		if n := len(msSleepRe.FindAllString(body, -1)); n != 0 {
			t.Errorf("%s: %d fixed millisecond sleep(s) remain — the wait must synchronise on the observable "+
				"event (the rendered line), retrying at the poll interval, never on a wall-clock guess", fn, n)
		}
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{
		"-c", "user.email=acs@example.com",
		"-c", "user.name=acs",
		"-c", "commit.gpgsign=false",
	}, args...)
	cmd := exec.Command("git", full...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", "acs: "+name)
}

func behindBaseRepo(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	origin := filepath.Join(base, "origin.git")
	work := filepath.Join(base, "work")
	for _, d := range []string{origin, work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	git(t, origin, "init", "--bare", "-b", "main")
	git(t, work, "init", "-b", "main")
	git(t, work, "remote", "add", "origin", origin)
	commitFile(t, work, "a.txt", "one")
	git(t, work, "push", "origin", "main")
	commitFile(t, work, "b.txt", "two")
	git(t, work, "push", "origin", "main")
	git(t, work, "reset", "--hard", "HEAD~1")
	return work
}

func TestC1191_008_boot_halts_on_base_behind_origin(t *testing.T) {
	work := behindBaseRepo(t)
	evolveDir := filepath.Join(work, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatalf("mkdir .evolve: %v", err)
	}
	res, err := looppreflight.Run(looppreflight.Options{
		ProjectRoot: work,
		EvolveDir:   evolveDir,
		ProfileDir:  filepath.Join(work, "profiles"),
		Stderr:      io.Discard,
		SkipBoot:    true,
	})
	if err != nil {
		t.Fatalf("looppreflight.Run harness fault: %v", err)
	}
	var found *looppreflight.CheckResult
	for i := range res.Checks {
		if res.Checks[i].Name == "base-divergence" {
			found = &res.Checks[i]
			break
		}
	}
	if found == nil {
		names := make([]string, 0, len(res.Checks))
		for _, c := range res.Checks {
			names = append(names, c.Name)
		}
		t.Fatalf("boot preflight runs no `base-divergence` check — lanes would still be cut from a stale base; checks=%v", names)
	}
	if found.Level != looppreflight.LevelHalt {
		t.Errorf("local main is 1 commit behind origin/main: base-divergence must HALT the batch before any lane spawns, got level=%v (%s / %s)",
			found.Level, found.Message, found.Detail)
	}
	if !strings.Contains(found.Detail, "evolve sync-main") {
		t.Errorf("the halt must name the reconcile command `evolve sync-main` so the stop comes with a next step; detail=%q", found.Detail)
	}
}
