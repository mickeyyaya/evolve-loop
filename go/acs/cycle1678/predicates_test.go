//go:build acs

package cycle1678

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

var (
	evolveBin      string
	evolveBuildErr error
	repoRoot       string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1678-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1678: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
		repoRoot = root
		out, berr := exec.Command("go", "-C", filepath.Join(root, "go"), "build", "-o", evolveBin, "./cmd/evolve").CombinedOutput()
		if berr != nil {
			evolveBuildErr = fmt.Errorf("go build ./cmd/evolve: %v\n%s", berr, out)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func repoRootFromCwd() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("git -C %s rev-parse --show-toplevel: %v", cwd, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runCLI(t *testing.T, root string, args ...string) (out string, code int) {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("evolve CLI unavailable: %v", evolveBuildErr)
	}
	cmd := exec.Command(evolveBin, args...)
	cmd.Dir = root
	env := os.Environ()
	kept := env[:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, "EVOLVE_PROJECT_ROOT=") {
			kept = append(kept, kv)
		}
	}
	cmd.Env = append(kept, "EVOLVE_PROJECT_ROOT="+root)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("run %s %v: %v", filepath.Base(evolveBin), args, err)
		}
		code = ee.ExitCode()
	}
	return strings.ReplaceAll(stdout.String()+stderr.String(), root, "<root>"), code
}

const (
	laneID     = "lane-work"
	routedID   = "operator-work"
	derivedID  = "role-gate-fix"
	routeQuiet = "camp-x"

	protectedPath = "go/internal/guards/role.go"

	routedReason  = "route:console-manual"
	derivedReason = "protected fix surface: " + protectedPath
)

func writeInboxItem(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "inbox")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func mixedInbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeInboxItem(t, root, "a.json",
		`{"id":"`+laneID+`","title":"lane item","weight":0.90,"campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	writeInboxItem(t, root, "b.json",
		`{"id":"`+routedID+`","title":"console item","weight":0.96,"route":"console-manual","campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	writeInboxItem(t, root, "c.json",
		`{"id":"`+derivedID+`","title":"protected derivation","weight":0.92,"files":["`+protectedPath+` (allowance)"],"kind":"bug","priority":"medium"}`)
	return root
}

func laneOnlyInbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeInboxItem(t, root, "a.json",
		`{"id":"`+laneID+`","title":"lane item","weight":0.90,"campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	writeInboxItem(t, root, "b.json",
		`{"id":"second-lane-work","title":"another lane item","weight":0.80,"campaign":"`+routeQuiet+`","kind":"feature","priority":"medium"}`)
	return root
}

func laneListingLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "- batch ") {
			lines = append(lines, l)
		}
	}
	return lines
}

func laneListingContains(out, id string) bool {
	for _, l := range laneListingLines(out) {
		if containsID(l, id) {
			return true
		}
	}
	return false
}

func containsID(s, id string) bool {
	return regexp.MustCompile(`(^|[^0-9A-Za-z_-])` + regexp.QuoteMeta(id) + `([^0-9A-Za-z_-]|$)`).MatchString(s)
}

func TestC1678_001_InboxBatchesSeparatesConsoleRoutedItemsWithReasons(t *testing.T) {
	root := mixedInbox(t)
	out, code := runCLI(t, root, "inbox", "batches")
	if code != 0 {
		t.Fatalf("RED: `evolve inbox batches` must succeed on a mixed backlog; exit=%d output=%q", code, out)
	}

	if !laneListingContains(out, laneID) {
		t.Errorf("RED: dispatchable item %q missing from the lane listing; got %q", laneID, out)
	}

	for _, id := range []string{routedID, derivedID} {
		if laneListingContains(out, id) {
			t.Errorf("RED: console-routed item %q is still rendered as a selectable lane batch — "+
				"render it OUTSIDE the `- batch …` listing (it is operator-owned, not a batch a lane may draw); got %q",
				id, out)
		}
	}

	for _, tc := range []struct{ id, reason string }{
		{routedID, routedReason},
		{derivedID, derivedReason},
	} {
		if !containsID(out, tc.id) {
			t.Errorf("RED: console-routed item %q vanished from the output entirely — "+
				"an operator worklist must SHOW operator-owned work, not hide it; got %q", tc.id, out)
			continue
		}
		if !strings.Contains(out, tc.reason) {
			t.Errorf("RED: console-routed item %q is listed without inboxbatch.PartitionConsole's reason %q; got %q",
				tc.id, tc.reason, out)
		}
	}
}

func TestC1678_002_LaneOnlyInboxKeepsEveryItemInTheBatchListing(t *testing.T) {
	root := laneOnlyInbox(t)
	out, code := runCLI(t, root, "inbox", "batches")
	if code != 0 {
		t.Fatalf("RED: `evolve inbox batches` must succeed on a lane-only backlog; exit=%d output=%q", code, out)
	}

	for _, id := range []string{laneID, "second-lane-work"} {
		if !laneListingContains(out, id) {
			t.Errorf("RED: dispatchable item %q missing from the lane listing; got %q", id, out)
		}
		total := len(regexp.MustCompile(`(^|[^0-9A-Za-z_-])`+regexp.QuoteMeta(id)+`([^0-9A-Za-z_-]|$)`).FindAllString(out, -1))
		inLane := 0
		for _, l := range laneListingLines(out) {
			inLane += len(regexp.MustCompile(`(^|[^0-9A-Za-z_-])`+regexp.QuoteMeta(id)+`([^0-9A-Za-z_-]|$)`).FindAllString(l, -1))
		}
		if total != inLane {
			t.Errorf("RED: nothing in this backlog is console-routed, yet %q appears %d time(s) outside the "+
				"selectable listing — the console section must be conditional on a real partition result; got %q",
				id, total-inLane, out)
		}
	}

	for _, reason := range []string{routedReason, derivedReason} {
		if strings.Contains(out, reason) {
			t.Errorf("RED: output claims routing reason %q over a backlog with no console-routed item; got %q", reason, out)
		}
	}
}

func TestC1678_003_InboxBatchesEdgeAndInvalidArgumentBehavior(t *testing.T) {
	t.Run("empty inbox", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		out, code := runCLI(t, root, "inbox", "batches")
		if code != 0 {
			t.Fatalf("RED: an empty inbox is not an error; exit=%d output=%q", code, out)
		}
		if lines := laneListingLines(out); len(lines) != 0 {
			t.Errorf("RED: empty inbox rendered %d batch line(s): %q", len(lines), lines)
		}
		for _, reason := range []string{routedReason, derivedReason} {
			if strings.Contains(out, reason) {
				t.Errorf("RED: empty inbox claims routing reason %q; got %q", reason, out)
			}
		}
	})

	t.Run("unknown argument", func(t *testing.T) {
		root := mixedInbox(t)
		out, code := runCLI(t, root, "inbox", "batches", "--console-only")
		if code != 10 {
			t.Errorf("RED: an unknown argument must exit 10 (usage), got %d; output=%q", code, out)
		}
		if len(laneListingLines(out)) != 0 {
			t.Errorf("RED: a refused invocation still rendered a worklist; got %q", out)
		}
	})

	t.Run("--max without a value", func(t *testing.T) {
		root := mixedInbox(t)
		out, code := runCLI(t, root, "inbox", "batches", "--max")
		if code != 10 {
			t.Errorf("RED: `--max` with no value must exit 10 (usage), got %d; output=%q", code, out)
		}
	})
}

func TestC1678_004_ConsoleRoutedItemsAreUnclaimableByALane(t *testing.T) {
	for _, tc := range []struct {
		name     string
		id       string
		wantCode int
		wantIn   string
	}{
		{"explicit route field", routedID, 3, routedReason},
		{"protected-files derivation", derivedID, 3, derivedReason},
		{"dispatchable item still claims", laneID, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := mixedInbox(t)
			out, code := runCLI(t, root, "inbox-mover", "claim", tc.id, "1678")
			if code != tc.wantCode {
				t.Fatalf("RED: claim %q exit=%d, want %d — a console-routed item must be refused (3) and a "+
					"dispatchable one claimed (0); output=%q", tc.id, code, tc.wantCode, out)
			}
			if tc.wantIn != "" && !strings.Contains(out, tc.wantIn) {
				t.Errorf("RED: the refusal of %q does not state why (want %q); got %q", tc.id, tc.wantIn, out)
			}
		})
	}
}

func runNamedPackageRaceTest(t *testing.T, pkg string) {
	t.Helper()
	if repoRoot == "" {
		t.Fatalf("repo root unresolved: %v", evolveBuildErr)
	}
	out, err := exec.Command("go", "-C", filepath.Join(repoRoot, "go"),
		"test", "-race", "-count=1", pkg).CombinedOutput()
	if err != nil {
		t.Errorf("RED: go test -race -count=1 %s failed: %v\n%s", pkg, err, out)
	}
}

func TestC1678_005_InboxbatchPackagePassesUnderRace(t *testing.T) {
	runNamedPackageRaceTest(t, "./internal/inboxbatch")
}

func TestC1678_010_LedgerPackagePassesUnderRace(t *testing.T) {
	runNamedPackageRaceTest(t, "./internal/adapters/ledger")
}

const zeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

const (
	seqA = 771301
	seqB = 881402
)

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func entryLine(fields map[string]any) []byte {
	base := map[string]any{
		"ts":        "2026-09-14T00:00:00Z",
		"cycle":     0,
		"role":      "phase",
		"kind":      "phase",
		"exit_code": 0,
	}
	for k, v := range fields {
		base[k] = v
	}
	b, err := json.Marshal(base)
	if err != nil {
		panic(err)
	}
	return b
}

type ledgerFixture struct {
	dir     string
	sealSeq int
	sealSHA string
	sealed  bool
}

func writeLedgerFixture(t *testing.T, sealSeq int, sealed, brokenTail bool) ledgerFixture {
	t.Helper()
	dir := t.TempDir()
	l0 := entryLine(map[string]any{"entry_seq": 0, "prev_hash": zeroSeed})
	l1 := entryLine(map[string]any{"entry_seq": 1, "prev_hash": sha256Hex(l0)})
	lines := [][]byte{l0, l1}

	fx := ledgerFixture{dir: dir, sealed: sealed}
	lastSeq := 2
	if !sealed {
		lines = append(lines, entryLine(map[string]any{"entry_seq": 2, "prev_hash": sha256Hex(l1)}))
	} else {
		l2 := entryLine(map[string]any{"entry_seq": 2, "prev_hash": strings.Repeat("de", 32)})
		seal := entryLine(map[string]any{
			"entry_seq": sealSeq,
			"role":      "operator",
			"kind":      fmt.Sprintf("reset-seal-cycle-%d", sealSeq),
			"prev_hash": sha256Hex(l2),
		})
		tailPrev := sha256Hex(seal)
		if brokenTail {
			tailPrev = strings.Repeat("ca", 32)
		}
		lines = append(lines, l2, seal, entryLine(map[string]any{"entry_seq": sealSeq + 1, "prev_hash": tailPrev}))
		fx.sealSeq, fx.sealSHA, lastSeq = sealSeq, sha256Hex(seal), sealSeq+1
	}

	var raw bytes.Buffer
	for _, l := range lines {
		raw.Write(l)
		raw.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), raw.Bytes(), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	tip := fmt.Sprintf("%d:%s", lastSeq, sha256Hex(lines[len(lines)-1]))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write ledger.tip: %v", err)
	}
	return fx
}

func runVerify(t *testing.T, evolveDir string, extra ...string) (out string, code int) {
	t.Helper()
	if evolveBuildErr != nil {
		t.Fatalf("evolve CLI unavailable: %v", evolveBuildErr)
	}
	args := append([]string{"ledger", "verify", "--evolve-dir", evolveDir}, extra...)
	stdout, stderr, code, err := acsassert.SubprocessOutput(evolveBin, args...)
	if err != nil && code == 0 {
		t.Fatalf("evolve ledger verify: %v", err)
	}
	return strings.ReplaceAll(stdout+stderr, evolveDir, "<evolve-dir>"), code
}

func namesSeq(out string, seq int) bool {
	return regexp.MustCompile(`(^|[^0-9])` + strconv.Itoa(seq) + `([^0-9]|$)`).MatchString(out)
}

func namesSHA(out, sha string) bool {
	return len(sha) >= 12 && strings.Contains(out, sha[:12])
}

func namesAnchor(out string, seq int, sha string) bool {
	return namesSeq(out, seq) || namesSHA(out, sha)
}

func TestC1678_006_SealedPrefixVerifiesAndNamesItsAnchor(t *testing.T) {
	fx := writeLedgerFixture(t, seqA, true, false)
	out, code := runVerify(t, fx.dir)
	if code != 0 {
		t.Fatalf("RED: a ledger whose break is covered by an eligible operator seal must verify; exit=%d output=%q", code, out)
	}
	if !namesAnchor(out, fx.sealSeq, fx.sealSHA) {
		t.Errorf("RED: successful verify does not state the sealed prefix — it must name the epoch anchor "+
			"(entry_seq=%d or line sha %s…); got %q", fx.sealSeq, fx.sealSHA[:12], out)
	}
}

func TestC1678_007_BreakAfterTheLastSealStillReportsBroken(t *testing.T) {
	fx := writeLedgerFixture(t, seqA, true, true)
	out, code := runVerify(t, fx.dir)
	if code == 0 {
		t.Fatalf("RED: a break one line PAST the last eligible seal must NOT verify; exit=0 output=%q", out)
	}
	if code != 2 {
		t.Errorf("RED: a broken chain must exit 2 (chain-broken), got %d; output=%q", code, out)
	}
	if !strings.Contains(strings.ToUpper(out), "BROKEN") {
		t.Errorf("RED: the refusal must say the chain is BROKEN; got %q", out)
	}
}

func TestC1678_008_AnchorProvenanceIsDerivedFromTheLedger(t *testing.T) {
	a := writeLedgerFixture(t, seqA, true, false)
	b := writeLedgerFixture(t, seqB, true, false)

	outA, codeA := runVerify(t, a.dir)
	outB, codeB := runVerify(t, b.dir)
	if codeA != 0 || codeB != 0 {
		t.Fatalf("RED: both sealed fixtures must verify; exits %d/%d\nA=%q\nB=%q", codeA, codeB, outA, outB)
	}
	if !namesAnchor(outA, a.sealSeq, a.sealSHA) || !namesAnchor(outB, b.sealSeq, b.sealSHA) {
		t.Fatalf("RED: each success must name its OWN anchor (A: seq=%d/%s…, B: seq=%d/%s…)\nA=%q\nB=%q",
			a.sealSeq, a.sealSHA[:12], b.sealSeq, b.sealSHA[:12], outA, outB)
	}
	if namesAnchor(outA, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: fixture A's output carries fixture B's anchor identity — the provenance is a literal, not derived; got %q", outA)
	}
	if namesAnchor(outB, a.sealSeq, a.sealSHA) {
		t.Errorf("RED: fixture B's output carries fixture A's anchor identity — the provenance is a literal, not derived; got %q", outB)
	}

	strict := writeLedgerFixture(t, 0, false, false)
	outS, codeS := runVerify(t, strict.dir)
	if codeS != 0 {
		t.Fatalf("RED: a clean chain with no seal must verify; exit=%d output=%q", codeS, outS)
	}
	for _, seq := range []int{seqA, seqB} {
		if namesSeq(outS, seq) {
			t.Errorf("RED: a fully strict verification claims an epoch anchor (entry_seq=%d) that this ledger does not have; got %q", seq, outS)
		}
	}
}

var ledgerFiles = []string{"ledger.jsonl", "ledger.tip", "ledger-anchor.json"}

func liveEvolveDir(t *testing.T) string {
	t.Helper()
	root := acsassert.RepoRoot(t)
	real, err := filepath.EvalSymlinks(filepath.Join(root, ".evolve", "ledger.jsonl"))
	if err != nil {
		t.Skipf("no live ledger to verify (%v)", err)
	}
	return filepath.Dir(real)
}

type fileStamp struct {
	size    int64
	modUnix int64
}

func stampOf(path string) (fileStamp, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return fileStamp{}, err
	}
	return fileStamp{size: fi.Size(), modUnix: fi.ModTime().UnixNano()}, nil
}

func snapshotLiveLedger(t *testing.T, src, dst string) map[string]string {
	t.Helper()
	for attempt := 1; attempt <= 6; attempt++ {
		before, err := stampOf(filepath.Join(src, "ledger.jsonl"))
		if err != nil {
			t.Skipf("no live ledger.jsonl (%v)", err)
		}
		shas := map[string]string{}
		for _, name := range ledgerFiles {
			raw, rerr := os.ReadFile(filepath.Join(src, name))
			if rerr != nil {
				if os.IsNotExist(rerr) && name == "ledger-anchor.json" {
					continue
				}
				t.Skipf("live ledger incomplete: %v", rerr)
			}
			if werr := os.WriteFile(filepath.Join(dst, name), raw, 0o644); werr != nil {
				t.Fatalf("snapshot %s: %v", name, werr)
			}
			shas[name] = sha256Hex(raw)
		}
		after, err := stampOf(filepath.Join(src, "ledger.jsonl"))
		if err != nil {
			t.Skipf("live ledger vanished mid-snapshot (%v)", err)
		}
		if before == after {
			return shas
		}
		t.Logf("snapshot attempt %d raced a concurrent append; retaking", attempt)
	}
	t.Fatalf("could not take a stable snapshot of %s/ledger.jsonl in 6 attempts", src)
	return nil
}

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

func isEligibleSeal(line []byte, prevLineSHA string) bool {
	if !bytes.Contains(line, []byte(`"role":"operator"`)) {
		return false
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(line, &raw) != nil {
		return false
	}
	if _, hasPrev := raw["prev_hash"]; !hasPrev {
		return false
	}
	var e struct {
		Role       string `json:"role"`
		Kind       string `json:"kind"`
		CycleLabel string `json:"cycle_label"`
		PrevHash   string `json:"prev_hash"`
	}
	if json.Unmarshal(line, &e) != nil || e.Role != "operator" {
		return false
	}
	if !strings.HasPrefix(e.Kind, "reset-seal-") && !strings.HasPrefix(e.CycleLabel, "reset-seal-") {
		return false
	}
	if prevLineSHA == "" {
		return e.PrevHash == zeroSeed
	}
	return e.PrevHash == prevLineSHA
}

func effectiveAnchor(lines [][]byte, sidecarSHA string) (sha string, seq int, found bool) {
	anchorSHA := sidecarSHA
	anchorIdx := -1
	reached := sidecarSHA == ""
	prevLineSHA := ""
	for i, line := range lines {
		lineSHA := sha256Hex(line)
		switch {
		case !reached && lineSHA == sidecarSHA:
			reached = true
			anchorIdx = i
		case !reached:
		default:
			if isEligibleSeal(line, prevLineSHA) {
				anchorSHA, anchorIdx = lineSHA, i
			}
		}
		prevLineSHA = lineSHA
	}
	if anchorSHA == "" || anchorIdx < 0 {
		return "", 0, false
	}
	var e struct {
		EntrySeq int `json:"entry_seq"`
	}
	if err := json.Unmarshal(lines[anchorIdx], &e); err != nil {
		return anchorSHA, 0, true
	}
	return anchorSHA, e.EntrySeq, true
}

func TestC1678_009_LiveLedgerVerifiesReportsItsScopeAndIsNotMutated(t *testing.T) {
	src := liveEvolveDir(t)
	snap := t.TempDir()
	before := snapshotLiveLedger(t, src, snap)

	raw, err := os.ReadFile(filepath.Join(snap, "ledger.jsonl"))
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	sidecar := ""
	if anchorRaw, aerr := os.ReadFile(filepath.Join(snap, "ledger-anchor.json")); aerr == nil {
		var a struct {
			AnchorLineSHA string `json:"anchor_line_sha256"`
		}
		if json.Unmarshal(anchorRaw, &a) == nil {
			sidecar = a.AnchorLineSHA
		}
	}
	wantSHA, wantSeq, haveAnchor := effectiveAnchor(splitLines(raw), sidecar)

	out, code := runVerify(t, snap)
	if code != 0 {
		t.Fatalf("RED: the real ledger must verify (the line-1740 damage is adjudicated and sealed); exit=%d output=%q", code, out)
	}
	if haveAnchor {
		if !namesAnchor(out, wantSeq, wantSHA) {
			t.Errorf("RED: the real ledger is verified FROM an epoch anchor (entry_seq=%d / %s…) but the successful "+
				"output does not state that scope — that is the wolf-cry's mirror image, a silent trust claim; got %q",
				wantSeq, wantSHA[:12], out)
		}
	} else {
		t.Logf("live ledger currently has no epoch anchor — full-strict verification; the scope assertion is 008's strict arm")
	}

	for name, sha := range before {
		got, rerr := os.ReadFile(filepath.Join(snap, name))
		if rerr != nil {
			t.Errorf("RED: verify removed %s: %v", name, rerr)
			continue
		}
		if sha256Hex(got) != sha {
			t.Errorf("RED: verify MUTATED %s (sha %s → %s) — ledger history must never be rewritten by a read",
				name, sha[:12], sha256Hex(got)[:12])
		}
	}
}

func TestC1678_011_JSONPathCarriesTheSamePartitionAsTheTextPath(t *testing.T) {
	root := mixedInbox(t)
	out, code := runCLI(t, root, "inbox", "batches", "--json")
	if code != 0 {
		t.Fatalf("RED: `evolve inbox batches --json` must succeed; exit=%d output=%q", code, out)
	}
	if !json.Valid([]byte(strings.TrimSpace(stdoutOnly(out)))) {
		t.Errorf("RED: --json output is not valid JSON; got %q", out)
	}
	for _, tc := range []struct{ id, reason string }{
		{routedID, routedReason},
		{derivedID, derivedReason},
	} {
		if !strings.Contains(out, tc.reason) {
			t.Errorf("RED: the --json path does not carry the routing reason for %q (want %q) — the partition is "+
				"wired into the text renderer only, so machine consumers still read operator-owned work as dispatchable; got %q",
				tc.id, tc.reason, out)
		}
	}
	if !containsID(out, laneID) {
		t.Errorf("RED: dispatchable item %q missing from the --json document; got %q", laneID, out)
	}
}

func stdoutOnly(out string) string {
	if i := strings.IndexAny(out, "[{"); i >= 0 {
		return out[i:]
	}
	return out
}
