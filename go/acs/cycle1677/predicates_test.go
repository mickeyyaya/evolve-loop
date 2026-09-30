//go:build acs

package cycle1677

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

const zeroSeed = "0000000000000000000000000000000000000000000000000000000000000000"

const (
	seqA = 770101
	seqB = 880202
)

var (
	evolveBin      string
	evolveBuildErr error
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "acs-cycle1677-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "acs/cycle1677: tempdir:", err)
		os.Exit(1)
	}
	evolveBin = filepath.Join(dir, "evolve")
	root, err := repoRootFromCwd()
	if err != nil {
		evolveBuildErr = err
	} else {
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

type fixture struct {
	dir      string
	sealSeq  int
	sealSHA  string
	tailSeq  int
	hasSeal  bool
	numLines int
}

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

func writeFixture(t *testing.T, sealSeq int, sealed, brokenTail bool) fixture {
	t.Helper()
	dir := t.TempDir()
	var lines [][]byte
	l0 := entryLine(map[string]any{"entry_seq": 0, "prev_hash": zeroSeed})
	lines = append(lines, l0)
	l1 := entryLine(map[string]any{"entry_seq": 1, "prev_hash": sha256Hex(l0)})
	lines = append(lines, l1)

	fx := fixture{dir: dir, hasSeal: sealed}
	if !sealed {
		l2 := entryLine(map[string]any{"entry_seq": 2, "prev_hash": sha256Hex(l1)})
		lines = append(lines, l2)
		fx.tailSeq = 2
	} else {
		l2 := entryLine(map[string]any{"entry_seq": 2, "prev_hash": strings.Repeat("de", 32)})
		lines = append(lines, l2)
		seal := entryLine(map[string]any{
			"entry_seq": sealSeq,
			"role":      "operator",
			"kind":      fmt.Sprintf("reset-seal-cycle-%d", sealSeq),
			"prev_hash": sha256Hex(l2),
		})
		lines = append(lines, seal)
		fx.sealSeq = sealSeq
		fx.sealSHA = sha256Hex(seal)
		tailPrev := sha256Hex(seal)
		if brokenTail {
			tailPrev = strings.Repeat("ca", 32)
		}
		lines = append(lines, entryLine(map[string]any{"entry_seq": sealSeq + 1, "prev_hash": tailPrev}))
		fx.tailSeq = sealSeq + 1
	}

	var raw bytes.Buffer
	for _, l := range lines {
		raw.Write(l)
		raw.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, "ledger.jsonl"), raw.Bytes(), 0o644); err != nil {
		t.Fatalf("write ledger.jsonl: %v", err)
	}
	last := lines[len(lines)-1]
	tip := fmt.Sprintf("%d:%s", fx.tailSeq, sha256Hex(last))
	if err := os.WriteFile(filepath.Join(dir, "ledger.tip"), []byte(tip), 0o644); err != nil {
		t.Fatalf("write ledger.tip: %v", err)
	}
	fx.numLines = len(lines)
	return fx
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

func TestC1677_001_SealedPrefixIsStatedOnSuccessfulVerify(t *testing.T) {
	fx := writeFixture(t, seqA, true, false)
	out, code := runVerify(t, fx.dir)
	if code != 0 {
		t.Fatalf("RED: sealed-prefix ledger must verify; exit=%d output=%q", code, out)
	}
	if !namesAnchor(out, fx.sealSeq, fx.sealSHA) {
		t.Errorf("RED: successful verify does not state the sealed prefix — output must name the epoch anchor "+
			"(entry_seq=%d or line sha %s…); got %q", fx.sealSeq, fx.sealSHA[:12], out)
	}
}

func TestC1677_002_ProvenanceIsDerivedFromTheLedgerNotHardcoded(t *testing.T) {
	a := writeFixture(t, seqA, true, false)
	b := writeFixture(t, seqB, true, false)
	strict := writeFixture(t, 0, false, false)

	outA, codeA := runVerify(t, a.dir)
	outB, codeB := runVerify(t, b.dir)
	outStrict, codeStrict := runVerify(t, strict.dir)
	if codeA != 0 || codeB != 0 || codeStrict != 0 {
		t.Fatalf("RED: all three fixtures must verify; exits A=%d B=%d strict=%d", codeA, codeB, codeStrict)
	}

	if !namesAnchor(outA, a.sealSeq, a.sealSHA) {
		t.Errorf("RED: fixture A does not name its own anchor (entry_seq=%d / %s…): %q", a.sealSeq, a.sealSHA[:12], outA)
	}
	if !namesAnchor(outB, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: fixture B does not name its own anchor (entry_seq=%d / %s…): %q", b.sealSeq, b.sealSHA[:12], outB)
	}
	if namesAnchor(outA, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: fixture A names fixture B's anchor — provenance is hardcoded, not derived: %q", outA)
	}
	if namesAnchor(outB, a.sealSeq, a.sealSHA) {
		t.Errorf("RED: fixture B names fixture A's anchor — provenance is hardcoded, not derived: %q", outB)
	}
	if namesAnchor(outStrict, a.sealSeq, a.sealSHA) || namesAnchor(outStrict, b.sealSeq, b.sealSHA) {
		t.Errorf("RED: a fully strict chain reports a sealed prefix it does not have: %q", outStrict)
	}
}

func TestC1677_003_BreakAfterTheLastEligibleSealStillFails(t *testing.T) {
	fx := writeFixture(t, seqA, true, true)
	out, code := runVerify(t, fx.dir)
	if code == 0 {
		t.Fatalf("RED: a break AFTER the last eligible seal must NOT verify; exit=0 output=%q", out)
	}
	if code != 2 {
		t.Errorf("RED: chain-broken must exit 2 (the documented verify contract); got %d, output=%q", code, out)
	}
	if !strings.Contains(out, "BROKEN") || !strings.Contains(out, "chain broken") {
		t.Errorf("RED: a post-seal break must be reported as a broken chain; got %q", out)
	}
	if strings.Contains(out, "OK:") {
		t.Errorf("RED: broken verification must not also emit a success line; got %q", out)
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

func splitLines(raw []byte) [][]byte {
	var out [][]byte
	for _, l := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(l)) > 0 {
			out = append(out, l)
		}
	}
	return out
}

func TestC1677_004_RealLedgerVerifiesNamesItsScopeAndIsNotMutated(t *testing.T) {
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
		t.Fatalf("RED: the real ledger must verify; exit=%d output=%q", code, out)
	}
	if haveAnchor {
		if !namesAnchor(out, wantSeq, wantSHA) {
			t.Errorf("RED: the real ledger is verified from an epoch anchor (entry_seq=%d / %s…) but the "+
				"successful output does not state that scope; got %q", wantSeq, wantSHA[:12], out)
		}
	} else {
		t.Logf("live ledger currently has no epoch anchor — full-strict verification; scope assertion is the strict arm of 002")
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

func TestC1677_005_DeepPathReportsTheSameProvenance(t *testing.T) {
	fx := writeFixture(t, seqA, true, false)
	out, code := runVerify(t, fx.dir, "--deep")
	if code != 0 {
		t.Fatalf("RED: --deep must verify the sealed-prefix ledger; exit=%d output=%q", code, out)
	}
	if !namesAnchor(out, fx.sealSeq, fx.sealSHA) {
		t.Errorf("RED: --deep success does not state the sealed prefix (entry_seq=%d / %s…); got %q",
			fx.sealSeq, fx.sealSHA[:12], out)
	}
}
