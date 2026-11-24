package heldproposal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func heldFor(families ...string) Record {
	return Record{Cycle: 7, Tree: "tree-a", Status: StatusHeld, DiffSHA256: "diff-a", BuilderFamilies: families}
}

func passFor(family string) (AuditReport, []byte) {
	body := []byte("# audit\n")
	return AuditReport{Cycle: 7, Tree: "tree-a", DiffSHA256: "diff-a", Family: family, CLI: family + "-tmux", Tier: policy.DeepTier, Verdict: AuditPass, BodySHA256: DigestOf(body)}, body
}

func TestVerifyAudit_EachRefusalHasItsOwnTypedError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		rec    Record
		mutate func(r *AuditReport, body *[]byte)
		absent bool
		want   error
	}{
		{"absent", heldFor("gemini"), nil, true, ErrAuditMissing},
		{"fail", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Verdict = AuditFail }, false, ErrAuditNotPass},
		{"wait", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Verdict = AuditWait }, false, ErrAuditNotPass},
		{"warn", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Verdict = "WARN" }, false, ErrAuditNotPass},
		{"builder family", heldFor("claude"), nil, false, ErrAuditBuilderFamily},
		{"one of two builder families", heldFor("claude", "gemini"), nil, false, ErrAuditBuilderFamily},
		{"no family on the report", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Family = "" }, false, ErrAuditBuilderFamily},
		{"unattributed hold", heldFor(), nil, false, ErrAuditUnattributed},
		{"blank builder family", heldFor(""), nil, false, ErrAuditUnattributed},
		{"other tree", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Tree = "tree-b" }, false, ErrAuditTreeDiffers},
		{"other diff", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.DiffSHA256 = "diff-b" }, false, ErrAuditDiffDiffers},
		{"below deep", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Tier = "balanced" }, false, ErrAuditNotDeep},
		{"an unknown tier word", heldFor("gemini"), func(r *AuditReport, _ *[]byte) { r.Tier = "mystery-model" }, false, ErrAuditNotDeep},
		{"changed body", heldFor("gemini"), func(_ *AuditReport, b *[]byte) { *b = []byte("forged") }, false, ErrAuditBodyDiffers},
		{"no body digest", heldFor("gemini"), func(r *AuditReport, b *[]byte) { r.BodySHA256, *b = "", nil }, false, ErrAuditBodyDiffers},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			report, body := passFor("claude")
			if tc.mutate != nil {
				tc.mutate(&report, &body)
			}
			err := VerifyAudit(tc.rec, report, !tc.absent, body)
			if !errors.Is(err, tc.want) {
				t.Errorf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestVerifyAudit_AnotherModelFamilyAtDeepPasses(t *testing.T) {
	t.Parallel()
	report, body := passFor("claude")
	if err := VerifyAudit(heldFor("gemini"), report, true, body); err != nil {
		t.Errorf("PASS from claude for a gemini build = %v, want nil", err)
	}
	report.Tier = "top"
	if err := VerifyAudit(heldFor("gemini"), report, true, body); err != nil {
		t.Errorf("top tier = %v, want nil: top is above deep", err)
	}
}

func TestAuditReport_RoundTripsWithItsBodyAndSitsBesideTheRecord(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	report, body := passFor("claude")
	report.At = time.Unix(1, 0).UTC()
	if err := WriteRecord(dir, heldFor("gemini")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAuditReport(dir, report, body); err != nil {
		t.Fatal(err)
	}

	got, gotBody, found, err := ReadAuditReport(dir, 7)

	if err != nil || !found || got.Family != "claude" || string(gotBody) != string(body) || got.BodySHA256 != DigestOf(body) {
		t.Errorf("read = %+v %q %v %v, want the written report and body", got, gotBody, found, err)
	}
	records, err := ReadRecords(dir)
	if err != nil || len(records) != 1 {
		t.Errorf("records = %v, %v, want the audit files ignored by the hold listing", records, err)
	}
	for _, path := range []string{AuditReportPath(dir, 7), AuditBodyPath(dir, 7), AuditDiffPath(dir, 7)} {
		if filepath.Dir(path) != HeldDir(dir) {
			t.Errorf("%s is not beside the hold record", path)
		}
	}
}

func TestWriteAuditReport_RefusesAnIncompleteReport(t *testing.T) {
	t.Parallel()
	if err := WriteAuditReport(t.TempDir(), AuditReport{Cycle: 0, Tree: "t", Verdict: AuditPass}, nil); !errors.Is(err, ErrBadRecord) {
		t.Errorf("err = %v, want ErrBadRecord", err)
	}
}

func TestReadAuditReport_AMissingReportIsNotFoundAndACorruptOneIsAnError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if _, _, found, err := ReadAuditReport(dir, 7); found || err != nil {
		t.Errorf("missing = %v, %v, want not found and no error", found, err)
	}
	if err := os.MkdirAll(HeldDir(dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(AuditReportPath(dir, 7), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadAuditReport(dir, 7); err == nil {
		t.Error("a corrupt report must be an error")
	}
	if err := os.WriteFile(AuditReportPath(dir, 7), []byte(`{"cycle":7,"tree":"t","verdict":"PASS"}`), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(AuditBodyPath(dir, 7), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := ReadAuditReport(dir, 7); err == nil {
		t.Error("an unreadable body must be an error, not a missing body")
	}
}

func TestAuditReport_AWaitReportHasNoBodyAndReadsBackWithNone(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WriteAuditReport(dir, AuditReport{Cycle: 7, Tree: "t", Verdict: AuditWait, Reason: WaitSeatFailed}, nil); err != nil {
		t.Fatal(err)
	}

	got, body, found, err := ReadAuditReport(dir, 7)

	if err != nil || !found || got.Verdict != AuditWait || body != nil {
		t.Errorf("read = %+v %q %v %v, want a WAIT report with no body", got, body, found, err)
	}
}

func TestWriteAuditReport_AFailedBodyWriteIsAnErrorAndLeavesNoReport(t *testing.T) {
	t.Parallel()
	report, body := passFor("claude")
	fileInTheWay := t.TempDir()
	if err := os.MkdirAll(filepath.Join(fileInTheWay, "landing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(HeldDir(fileInTheWay), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteAuditReport(fileInTheWay, report, body); err == nil {
		t.Error("a held directory that is a file must fail the body write")
	}

	dirInTheWay := t.TempDir()
	if err := os.MkdirAll(AuditBodyPath(dirInTheWay, 7), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := WriteAuditReport(dirInTheWay, report, body); err == nil {
		t.Error("a body path that is a directory must fail the body write")
	}
	if _, err := os.Stat(AuditReportPath(dirInTheWay, 7)); err == nil {
		t.Error("a report whose body failed must not be written: it would name a digest with no body")
	}
}

func TestIndependent_RefusesABuilderFamilyAnEmptyFamilyAndATierBelowDeep(t *testing.T) {
	t.Parallel()
	builders := []string{"claude", "gpt"}
	cases := []struct {
		name, family, tier string
		want               error
	}{
		{"another family at deep", "gemini", "deep", nil},
		{"another family at top", "gemini", "top", nil},
		{"a builder family", "claude", "deep", ErrAuditBuilderFamily},
		{"the second builder family", "gpt", "deep", ErrAuditBuilderFamily},
		{"an empty family", "", "deep", ErrAuditBuilderFamily},
		{"another family below deep", "gemini", "balanced", ErrAuditNotDeep},
		{"another family at an unknown tier", "gemini", "mystery", ErrAuditNotDeep},
	}
	for _, tc := range cases {
		if err := Independent(tc.family, tc.tier, builders); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}
	if err := Independent("claude", "deep", nil); err != nil {
		t.Errorf("no builder family listed: %v; the unattributed check belongs to VerifyAudit", err)
	}
}

func TestWriteAuditDiff_WritesTheDiffBesideTheRecordAndReturnsItsDigest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	digest, err := WriteAuditDiff(dir, 7, []byte("diff body\n"))

	got, rerr := os.ReadFile(AuditDiffPath(dir, 7))
	if err != nil || rerr != nil || string(got) != "diff body\n" || digest != DigestOf([]byte("diff body\n")) {
		t.Errorf("digest %q err %v read %q %v, want the file and its digest", digest, err, got, rerr)
	}
	if filepath.Dir(AuditDiffPath(dir, 7)) != HeldDir(dir) {
		t.Error("the diff is not beside the hold record")
	}
}

func TestWriteAuditDiff_AFailedWriteReturnsNoDigest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.MkdirAll(AuditDiffPath(dir, 7), 0o755); err != nil {
		t.Fatal(err)
	}

	digest, err := WriteAuditDiff(dir, 7, []byte("x"))

	if err == nil || digest != "" {
		t.Errorf("digest %q err %v, want an error and no digest", digest, err)
	}
}
