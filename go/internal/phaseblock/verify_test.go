package phaseblock

import (
	"errors"
	"testing"
)

// buildChain threads prevCombined through Compute to produce a valid,
// link-consistent chain from per-phase sources.
func buildChain(t *testing.T, phases []string, srcs []fakeSource) []Digest {
	t.Helper()
	if len(phases) != len(srcs) {
		t.Fatalf("phases/srcs length mismatch")
	}
	var chain []Digest
	prev := ""
	for i, ph := range phases {
		d, err := Compute(ph, "run", "ts", prev, srcs[i])
		if err != nil {
			t.Fatal(err)
		}
		chain = append(chain, d)
		prev = d.Combined
	}
	return chain
}

// allOK is a provenance oracle that trusts every (non-empty) commit.
func allOK(commit string) bool { return commit != "" }

func TestVerify_EmptyChain_ReturnsSentinel(t *testing.T) {
	if err := Verify(nil, "binA", "cA", allOK); !errors.Is(err, ErrEmptyChain) {
		t.Fatalf("empty chain must return ErrEmptyChain (for legacy fallback), got %v", err)
	}
}

func TestVerify_SingleBinary_Pass(t *testing.T) {
	chain := buildChain(t,
		[]string{"scout", "build", "audit"},
		[]fakeSource{
			{bin: "binA", commit: "cA", prof: "p1"},
			{bin: "binA", commit: "cA", prof: "p2", report: "r2", tree: "t2"},
			{bin: "binA", commit: "cA", prof: "p3", report: "r3"},
		})
	if err := Verify(chain, "binA", "cA", allOK); err != nil {
		t.Errorf("single-binary cycle must pass: %v", err)
	}
}

func TestVerify_ResumedUnderRebuild_VerifiedProvenance_Pass(t *testing.T) {
	// scout/build ran under binA (commit cA); resumed: audit ran under binB
	// (commit cB); ship runs under binB. Both commits are ancestors of HEAD.
	chain := buildChain(t,
		[]string{"scout", "build", "audit"},
		[]fakeSource{
			{bin: "binA", commit: "cA", prof: "p1"},
			{bin: "binA", commit: "cA", prof: "p2"},
			{bin: "binB", commit: "cB", prof: "p3"},
		})
	if err := Verify(chain, "binB", "cB", allOK); err != nil {
		t.Errorf("resumed-under-verified-rebuild must pass (the cycle-384 fix): %v", err)
	}
}

func TestVerify_DifferentBinary_NoProvenance_Tamper(t *testing.T) {
	// A phase ran under binA with NO build-commit (unstamped) — unverifiable.
	chain := buildChain(t,
		[]string{"scout", "build"},
		[]fakeSource{
			{bin: "binA", commit: "", prof: "p1"}, // unstamped
			{bin: "binB", commit: "cB", prof: "p2"},
		})
	err := Verify(chain, "binB", "cB", allOK)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("unstamped differing binary must be a TamperError, got %v", err)
	}
}

func TestVerify_RunningBinaryUnverified_Tamper(t *testing.T) {
	chain := buildChain(t,
		[]string{"scout", "build"},
		[]fakeSource{
			{bin: "binA", commit: "cA", prof: "p1"},
			{bin: "binA", commit: "cA", prof: "p2"},
		})
	// running binary differs and its own commit is unverifiable.
	err := Verify(chain, "binEVIL", "", allOK)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("unverified running binary must be a TamperError, got %v", err)
	}
}

func TestVerify_NonAncestorCommit_Tamper(t *testing.T) {
	chain := buildChain(t,
		[]string{"scout", "build"},
		[]fakeSource{
			{bin: "binA", commit: "cEVIL", prof: "p1"},
			{bin: "binB", commit: "cB", prof: "p2"},
		})
	// oracle rejects cEVIL (not an ancestor of HEAD).
	prov := func(c string) bool { return c == "cB" }
	err := Verify(chain, "binB", "cB", prov)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("non-ancestor build-commit must be a TamperError, got %v", err)
	}
}

func TestVerify_BrokenChainLink_Tamper(t *testing.T) {
	chain := buildChain(t,
		[]string{"scout", "build"},
		[]fakeSource{
			{bin: "binA", commit: "cA", prof: "p1"},
			{bin: "binA", commit: "cA", prof: "p2"},
		})
	chain[1].PrevCombined = "tampered-link" // break the back-pointer
	err := Verify(chain, "binA", "cA", allOK)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("broken chain link must be a TamperError, got %v", err)
	}
}

func TestVerify_EmptyRunningSHA_Tamper(t *testing.T) {
	// A chain of empty-sha records + an empty running sha must NOT slip through
	// the single-binary fast-path: an unestablished binary identity is a tamper
	// posture, not a clean pass.
	chain := buildChain(t, []string{"scout"}, []fakeSource{{bin: "", commit: "", prof: "p"}})
	err := Verify(chain, "", "", allOK)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("empty running sha must be a TamperError, got %v", err)
	}
}

func TestVerify_BackPointerMismatch_Tamper(t *testing.T) {
	// Isolate the back-pointer link check: chain[1] is computed with a
	// self-consistent but WRONG prevCombined, so combine(chain[1])==Combined
	// (the recompute check passes) yet PrevCombined != chain[0].Combined.
	src := fakeSource{bin: "binA", commit: "cA", prof: "p"}
	d0, _ := Compute("scout", "run", "ts", "", src)
	d1, _ := Compute("build", "run", "ts", "wrong-but-self-consistent-prev", src)
	chain := []Digest{d0, d1}
	err := Verify(chain, "binA", "cA", allOK)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("back-pointer mismatch must be a TamperError, got %v", err)
	}
	if te.Reason != "broken chain link to previous phase" {
		t.Errorf("expected the back-pointer branch, got reason %q", te.Reason)
	}
}

func TestVerify_CombinedMismatch_Tamper(t *testing.T) {
	chain := buildChain(t,
		[]string{"scout", "build"},
		[]fakeSource{
			{bin: "binA", commit: "cA", prof: "p1"},
			{bin: "binA", commit: "cA", prof: "p2"},
		})
	chain[0].BinarySHA = "swapped" // mutate a field without recomputing Combined
	err := Verify(chain, "binA", "cA", allOK)
	var te *TamperError
	if !errors.As(err, &te) {
		t.Fatalf("recomputed-combined mismatch must be a TamperError, got %v", err)
	}
}
