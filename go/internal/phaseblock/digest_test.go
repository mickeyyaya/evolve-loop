package phaseblock

import (
	"errors"
	"testing"
)

// fakeSource is a deterministic DigestSource for unit tests — no IO.
type fakeSource struct {
	bin, commit, prof, report, tree string
	binErr, profErr                 error
}

func (f fakeSource) BinarySHA() (string, error)  { return f.bin, f.binErr }
func (f fakeSource) BinaryCommit() string        { return f.commit }
func (f fakeSource) ProfileSHA() (string, error) { return f.prof, f.profErr }
func (f fakeSource) ReportSHA() (string, error)  { return f.report, nil }
func (f fakeSource) TreeSHA() (string, error)    { return f.tree, nil }

// Compute is the pure content-addressed digest. Combined must be a stable
// function of the integrity-relevant fields ONLY (phase, binary, commit,
// profile, report, tree, prevCombined) — never of run metadata (runID,
// completedAt) — so regenerating a phase with identical content is idempotent.
func TestCompute_DeterministicOverRunMetadata(t *testing.T) {
	src := fakeSource{bin: "binA", commit: "c0ffeecafe12", prof: "profX", report: "rep1", tree: "tree1"}
	d1, err := Compute("build", "run-1", "2026-06-25T00:00:00Z", "prev0", src)
	if err != nil {
		t.Fatal(err)
	}
	d2, err := Compute("build", "run-2", "2026-06-25T11:11:11Z", "prev0", src)
	if err != nil {
		t.Fatal(err)
	}
	if d1.Combined != d2.Combined {
		t.Errorf("Combined must not depend on run metadata: %q vs %q", d1.Combined, d2.Combined)
	}
	if d1.Combined == "" {
		t.Error("Combined is empty")
	}
	if d1.BinarySHA != "binA" || d1.BinaryCommit != "c0ffeecafe12" || d1.ReportSHA != "rep1" || d1.TreeSHA != "tree1" {
		t.Errorf("digest fields not populated from source: %+v", d1)
	}
}

func TestCompute_PrevCombinedChains(t *testing.T) {
	src := fakeSource{bin: "binA", commit: "c", prof: "p", report: "r", tree: "t"}
	a, _ := Compute("scout", "run", "t", "", src)
	b, _ := Compute("scout", "run", "t", "PREVDIGEST", src)
	if a.Combined == b.Combined {
		t.Error("prevCombined must influence Combined (the chain link)")
	}
	if b.PrevCombined != "PREVDIGEST" {
		t.Errorf("PrevCombined=%q, want PREVDIGEST", b.PrevCombined)
	}
}

func TestCompute_OmitsAbsentReportAndTree(t *testing.T) {
	src := fakeSource{bin: "binA", commit: "c", prof: "p"} // no report, no tree
	d, err := Compute("intent", "run", "t", "", src)
	if err != nil {
		t.Fatal(err)
	}
	if d.ReportSHA != "" || d.TreeSHA != "" {
		t.Errorf("absent report/tree must stay empty: %+v", d)
	}
	if d.Combined == "" {
		t.Error("Combined must still compute with absent report/tree")
	}
}

func TestCompute_PropagatesSourceErrors(t *testing.T) {
	if _, err := Compute("build", "r", "t", "", fakeSource{binErr: errors.New("bin boom")}); err == nil {
		t.Fatal("expected BinarySHA error to propagate")
	}
	if _, err := Compute("build", "r", "t", "", fakeSource{bin: "x", profErr: errors.New("prof boom")}); err == nil {
		t.Fatal("expected ProfileSHA error to propagate")
	}
}
