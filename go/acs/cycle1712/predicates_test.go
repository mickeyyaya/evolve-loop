//go:build acs

package cycle1712

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/apicover"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

const modelqueryPkg = "github.com/mickeyyaya/evolve-loop/go/internal/modelquery"

func runGo(t *testing.T, args ...string) (string, int) {
	t.Helper()
	stdout, stderr, code, err := acsassert.SubprocessOutput("go", args...)
	if code < 0 {
		t.Fatalf("go %s failed to launch: %v\n%s%s", strings.Join(args, " "), err, stdout, stderr)
	}
	return stdout + stderr, code
}

func modelqueryDir(t *testing.T) string {
	t.Helper()
	out, code := runGo(t, "list", "-f", "{{.Dir}}", modelqueryPkg)
	if code != 0 {
		t.Fatalf("go list %s exit=%d:\n%s", modelqueryPkg, code, out)
	}
	return strings.TrimSpace(out)
}

type promotionCase struct {
	name       string
	sel        string
	candidates []string
	want       string
}

func runPromotionCases(t *testing.T, cases []promotionCase) {
	t.Helper()
	for _, tc := range cases {
		got := modelquery.PromoteLatest(map[string]string{"deep": tc.sel}, tc.candidates, modelquery.FreshnessPolicy{})
		if got["deep"] != tc.want {
			t.Errorf("%s: PromoteLatest(deep=%q, candidates=%q) = %q, want %q", tc.name, tc.sel, tc.candidates, got["deep"], tc.want)
		}
	}
}

func TestC1712_001_DatedSnapshotsShareLineageAndPromote(t *testing.T) {
	a, b := modelquery.LineageKey("gpt-4o-2024-08-06"), modelquery.LineageKey("gpt-4o-2024-11-20")
	if a != b {
		t.Errorf("LineageKey(gpt-4o-2024-08-06)=%q != LineageKey(gpt-4o-2024-11-20)=%q — same-line dated snapshots must share a key", a, b)
	}
	runPromotionCases(t, []promotionCase{
		{"older dated snapshot promotes to the newer", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"}, "gpt-4o-2024-11-20"},
		{"older dated snapshot promotes to the newer (newer listed first)", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"}, "gpt-4o-2024-11-20"},
	})
}

func TestC1712_002_CapabilityClassesStayDistinct(t *testing.T) {
	mustDiffer := [][2]string{
		{"llama3.1:8b", "llama3.1:70b"},
		{"llama3.1:8b", "llama3.3:70b"},
		{"qwen2.5-coder:32b", "qwen2.5-coder:7b"},
		{"qwen2.5-coder:32b", "qwen2.5-coder:70b"},
		{"gpt-4o-mini-2024-07-18", "gpt-4o-2024-08-06"},
		{"gemini-2.5-flash-2025-06-17", "gemini-2.5-pro-2025-06-17"},
		{"gpt-4o-2024-08-06", "gpt-4-turbo-2024-04-09"},
	}
	for _, p := range mustDiffer {
		if ka, kb := modelquery.LineageKey(p[0]), modelquery.LineageKey(p[1]); ka == kb {
			t.Errorf("LineageKey(%q) == LineageKey(%q) = %q — capability classes collided", p[0], p[1], ka)
		}
	}
	runPromotionCases(t, []promotionCase{
		{"8b selection never jumps to the 70b size class", "llama3.1:8b", []string{"llama3.1:8b", "llama3.1:70b"}, "llama3.1:8b"},
		{"dated base selection never jumps to a newer-dated mini", "gpt-4o-2024-08-06", []string{"gpt-4o-2024-08-06", "gpt-4o-mini-2024-11-20"}, "gpt-4o-2024-08-06"},
	})
}

func TestC1712_003_NewestInLineageOrdersDatedSnapshots(t *testing.T) {
	for _, ids := range [][]string{
		{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"},
		{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"},
		{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20", "gpt-4o-2024-05-13"},
	} {
		if got := modelquery.NewestInLineage(ids); got != "gpt-4o-2024-11-20" {
			t.Errorf("NewestInLineage(%q) = %q, want gpt-4o-2024-11-20 (the latest date)", ids, got)
		}
	}
}

func TestC1712_004_KnownLimitationTestMigratedToMustMatch(t *testing.T) {
	const (
		removed  = "TestLineageKey_DatedSnapshotsStayDistinct_KnownLimitation"
		replaced = "TestLineageKey_SeparatesCapabilityClasses"
	)
	listed, code := runGo(t, "test", "-count=1", "-list", "^TestLineageKey_", modelqueryPkg)
	if code != 0 {
		t.Fatalf("go test -list %s exit=%d:\n%s", modelqueryPkg, code, listed)
	}
	names := strings.Fields(listed)
	if containsLine(names, removed) {
		t.Errorf("%s is still in the test binary — the known-limitation pin was not migrated", removed)
	}
	if !containsLine(names, replaced) {
		t.Fatalf("%s is missing from the test binary:\n%s", replaced, listed)
	}
	out, code := runGo(t, "test", "-count=1", "-run", "^"+replaced+"$", modelqueryPkg)
	if code != 0 {
		t.Errorf("%s failed:\n%s", replaced, out)
	}
	n, err := acsassert.CountInGoFunc(filepath.Join(modelqueryDir(t), "lineage_test.go"), replaced, `"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"`)
	if err != nil {
		t.Fatalf("read %s: %v", replaced, err)
	}
	if n == 0 {
		t.Errorf("%s does not carry the migrated {gpt-4o-2024-08-06, gpt-4o-2024-11-20} row", replaced)
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}

const fingerprintV1 = "sha256:1cc2285255665ad56d12d19955c0321e8059fd211cb2c8cccd00b8466abdb97d"

var fingerprintBaseline = modelquery.FingerprintInput{
	CLI:        "acs-cycle1712-baseline",
	Candidates: []string{"gpt-4o-2024-08-06", "gpt-4o-2024-11-20"},
	Policy:     modelquery.FreshnessPolicy{},
	Tiers:      []string{"fast", "balanced", "deep"},
}

func TestC1712_005_DecisionVersionBumped(t *testing.T) {
	got := modelquery.Fingerprint(fingerprintBaseline)
	if !strings.HasPrefix(got, "sha256:") {
		t.Fatalf("Fingerprint returned %q, want a sha256: rendering", got)
	}
	if got == fingerprintV1 {
		t.Errorf("Fingerprint(%+v) = %s reproduces the pre-fix v1 value — decisionVersion was not bumped", fingerprintBaseline, got)
	}
}

func TestC1712_006_DecisionSurfacePinInSync(t *testing.T) {
	out, code := runGo(t, "test", "-count=1", "-run", "^TestDecisionVersion_PinnedToAlgorithmSurface$", "-v", modelqueryPkg)
	if code != 0 {
		t.Errorf("TestDecisionVersion_PinnedToAlgorithmSurface failed (decisionSurfacePin out of sync):\n%s", out)
	}
	if !strings.Contains(out, "--- PASS: TestDecisionVersion_PinnedToAlgorithmSurface") {
		t.Errorf("TestDecisionVersion_PinnedToAlgorithmSurface did not run:\n%s", out)
	}
}

func TestC1712_007_PromoteLatestNeverDowngradesAcrossDateStamps(t *testing.T) {
	runPromotionCases(t, []promotionCase{
		{"undated newer version vs dated older version", "gpt-5", []string{"gpt-5", "gpt-4-2024-04-09"}, "gpt-5"},
		{"undated newer version vs dated older version (dated listed first)", "gpt-5", []string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{"both dated: higher version beats a later date", "gpt-5-2025-01-01", []string{"gpt-5-2025-01-01", "gpt-4-2025-06-01"}, "gpt-5-2025-01-01"},
		{"both dated: higher version beats a later date (older version listed first)", "gpt-5-2025-01-01", []string{"gpt-4-2025-06-01", "gpt-5-2025-01-01"}, "gpt-5-2025-01-01"},
		{"undated alias not replaced by a same-version dated snapshot", "gpt-4o", []string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o"},
		{"undated alias not replaced by a same-version dated snapshot (dated listed first)", "gpt-4o", []string{"gpt-4o-2024-08-06", "gpt-4o"}, "gpt-4o"},
		{"a date is not a version number", "gpt-5", []string{"gpt-5", "gpt-2024-04-09"}, "gpt-5"},
		{"a date is not a version number (dated listed first)", "gpt-5", []string{"gpt-2024-04-09", "gpt-5"}, "gpt-5"},
	})
}

func TestC1712_008_NewestInLineageComparesVersionBeforeDate(t *testing.T) {
	cases := []struct {
		ids  []string
		want string
	}{
		{[]string{"gpt-5", "gpt-4-2024-04-09"}, "gpt-5"},
		{[]string{"gpt-4-2024-04-09", "gpt-5"}, "gpt-5"},
		{[]string{"gpt-5-2025-01-01", "gpt-4-2025-06-01"}, "gpt-5-2025-01-01"},
		{[]string{"gpt-4-2025-06-01", "gpt-5-2025-01-01"}, "gpt-5-2025-01-01"},
		{[]string{"gpt-5", "gpt-2024-04-09"}, "gpt-5"},
		{[]string{"gpt-2024-04-09", "gpt-5"}, "gpt-5"},
		{[]string{"gpt-4o", "gpt-4o-2024-08-06"}, "gpt-4o"},
	}
	for _, tc := range cases {
		if got := modelquery.NewestInLineage(tc.ids); got != tc.want {
			t.Errorf("NewestInLineage(%q) = %q, want %q (version must decide before the date)", tc.ids, got, tc.want)
		}
	}
}

func TestC1712_009_UndatedPromotionUnchanged(t *testing.T) {
	runPromotionCases(t, []promotionCase{
		{"undated codex line promotes within itself", "gpt-5.5", []string{"gpt-5.5", "gpt-5.6", "gpt-5.6-mini"}, "gpt-5.6"},
		{"undated ollama line promotes within its size class", "llama3.1:8b", []string{"llama3.1:8b", "llama3.1:70b", "llama3.3:8b"}, "llama3.3:8b"},
		{"component-wise numeric order, not lexicographic", "opus-4.9", []string{"opus-4.9", "opus-4.10"}, "opus-4.10"},
	})
}

func TestC1712_010_ModelqueryVetRaceApicoverGreen(t *testing.T) {
	if out, code := runGo(t, "vet", modelqueryPkg); code != 0 {
		t.Errorf("go vet %s exit=%d:\n%s", modelqueryPkg, code, out)
	}
	profile := filepath.Join(t.TempDir(), "coverage.txt")
	if out, code := runGo(t, "test", "-count=1", "-race", "-coverprofile="+profile, modelqueryPkg); code != 0 {
		t.Fatalf("go test -race %s exit=%d:\n%s", modelqueryPkg, code, out)
	}
	funcOut, coverErr, code, err := acsassert.SubprocessOutput("go", "tool", "cover", "-func="+profile)
	if code != 0 {
		t.Fatalf("go tool cover -func exit=%d: %v\n%s", code, err, coverErr)
	}
	funcFile := filepath.Join(t.TempDir(), "coverage.func.txt")
	if err := os.WriteFile(funcFile, []byte(funcOut), 0o644); err != nil {
		t.Fatalf("write %s: %v", funcFile, err)
	}
	var stdout, stderr bytes.Buffer
	if rc := apicover.Main([]string{"-enforce", "-cover", funcFile, modelqueryDir(t)}, &stdout, &stderr); rc != 0 {
		t.Errorf("apicover -enforce %s exit=%d:\n%s%s", modelqueryPkg, rc, stdout.String(), stderr.String())
	}
}

const (
	pendingInboxRel  = ".evolve/inbox/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json"
	consumedInboxRel = ".evolve/inbox/consumed/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json"
)

var (
	unchangedContentClaim  = regexp.MustCompile(`(?i)\bcontents?\b[^.;]{0,20}\bunchanged\b|\bunchanged\s+contents?\b|\bsame\s+contents?\b|\bbyte[- ](for[- ]byte|identical)\b`)
	consumeStampDisclosure = regexp.MustCompile("(?i)\\bstamp|\\bprovenance\\b|consumed`?\\s+(block|field|key|object|marker)")
	reserializeDisclosure  = regexp.MustCompile(`(?i)re-?serializ|re-?marshal|re-?encod|re-?format|re-?order|\bescap|key order|sorted keys`)
	scanStartQualifier     = regexp.MustCompile(`(?i)\beither\b|\bitself\b|never (a )?downgrad|\bor (the |its )?(start|starting|incumbent|first)\b|\bor strictly newer\b`)
	sentenceEnd            = regexp.MustCompile(`[.!?](\s|$)`)
)

func explanationDocument(t *testing.T) (root, rel, body string) {
	t.Helper()
	root = acsassert.RepoRoot(t)
	docs, err := filepath.Glob(filepath.Join(root, "docs", "explain", "builds", "cycle-1712-*.md"))
	if err != nil || len(docs) != 1 {
		t.Fatalf("want exactly one cycle-1712 explanation document under docs/explain/builds, got %v (err %v)", docs, err)
	}
	rel, err = filepath.Rel(root, docs[0])
	if err != nil {
		t.Fatalf("relativise %s: %v", docs[0], err)
	}
	if _, _, code, _ := acsassert.SubprocessOutput("git", "-C", root, "ls-files", "--error-unmatch", rel); code != 0 {
		t.Errorf("%s is untracked — it would be dropped at ship", rel)
	}
	raw, err := os.ReadFile(docs[0])
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return root, rel, string(raw)
}

func docSection(t *testing.T, rel, body, heading string) string {
	t.Helper()
	section, ok, err := reportdoc.Section(body, heading)
	if err != nil || !ok {
		t.Fatalf("%s: no single ## %s section (ok=%v err=%v)", rel, heading, ok, err)
	}
	return section
}

func changedAreaEntry(changedAreas, path string) string {
	prefix := "- `" + path + "`"
	for _, line := range strings.Split(changedAreas, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

type rename struct {
	from       string
	similarity int
}

func renamesSince(t *testing.T, root, base string) map[string]rename {
	t.Helper()
	out, stderr, code, err := acsassert.SubprocessOutput("git", "-C", root, "diff", "-M", "--name-status", base)
	if code != 0 {
		t.Fatalf("git diff -M --name-status %s exit=%d: %v\n%s", base, code, err, stderr)
	}
	renames := map[string]rename{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 3 || !strings.HasPrefix(fields[0], "R") {
			continue
		}
		similarity, err := strconv.Atoi(fields[0][1:])
		if err != nil {
			t.Fatalf("git diff: unparseable rename status %q in %q", fields[0], line)
		}
		renames[fields[2]] = rename{from: fields[1], similarity: similarity}
	}
	return renames
}

func TestC1712_011_ExplanationRenameEntryMatchesTheDiff(t *testing.T) {
	root, rel, body := explanationDocument(t)
	fields, err := reportdoc.Fields(docSection(t, rel, body, "Build Binding"), "Cycle", "Base SHA")
	if err != nil {
		t.Fatalf("%s Build Binding: %v", rel, err)
	}
	base := fields["base sha"]
	renames := renamesSince(t, root, base)
	moved, ok := renames[consumedInboxRel]
	if !ok || moved.from != pendingInboxRel {
		t.Fatalf("git diff -M %s does not report %s -> %s as a rename (renames: %v)", base, pendingInboxRel, consumedInboxRel, renames)
	}
	raw, err := os.ReadFile(filepath.Join(root, consumedInboxRel))
	if err != nil {
		t.Fatalf("read %s: %v", consumedInboxRel, err)
	}
	var record map[string]any
	if err := json.Unmarshal(raw, &record); err != nil {
		t.Fatalf("parse %s: %v", consumedInboxRel, err)
	}
	if _, stamped := record["consumed"]; !stamped {
		t.Errorf("%s no longer carries the ship's `consumed` provenance stamp — correct the explanation, not the consumed record", consumedInboxRel)
	}
	changedAreas := docSection(t, rel, body, "Changed Areas")
	for dst, r := range renames {
		if entry := changedAreaEntry(changedAreas, dst); r.similarity < 100 && unchangedContentClaim.MatchString(entry) {
			t.Errorf("%s Changed Areas says %s is unchanged, but git scores its rename from %s at %d%%: %q", rel, dst, r.from, r.similarity, entry)
		}
	}
	entry := changedAreaEntry(changedAreas, consumedInboxRel)
	if !consumeStampDisclosure.MatchString(entry) {
		t.Errorf("%s Changed Areas entry for %s does not disclose the `consumed` provenance stamp the move added: %q", rel, consumedInboxRel, entry)
	}
	if !reserializeDisclosure.MatchString(entry) {
		t.Errorf("%s Changed Areas entry for %s does not disclose that the move re-serialized the record (keys re-sorted, `>` escaped): %q", rel, consumedInboxRel, entry)
	}
}

func TestC1712_012_ExplanationScanGuaranteeAdmitsTheStart(t *testing.T) {
	for _, ids := range [][]string{
		{"gpt-4o-2024-11-20", "gpt-4o-2024-08-06"},
		{"gpt-5", "gpt-4-2024-04-09"},
		{"gpt-4o", "gpt-4o-2024-08-06"},
	} {
		if got := modelquery.NewestInLineage(ids); got != ids[0] {
			t.Fatalf("NewestInLineage(%q) = %q, want the start %q — a scan with nothing strictly newer must end at its start", ids, got, ids[0])
		}
	}
	_, rel, body := explanationDocument(t)
	prose := strings.Join(strings.Fields(docSection(t, rel, body, "Design Decisions")), " ")
	for _, sentence := range sentenceEnd.Split(prose, -1) {
		if strings.Contains(strings.ToLower(sentence), "strictly newer") && !scanStartQualifier.MatchString(sentence) {
			t.Errorf("%s Design Decisions overstates the scan guarantee — the winner can be the start itself (a tie, or nothing newer): %q", rel, sentence)
		}
	}
}
