package recovery

// signatures_c1117_test.go — owner-package coverage for FatalPaneDetector.
// Signatures (cycle-1117). The behavioural contract is pinned across the seam
// that consumes it (bridge's TestC1117_SignaturesAccessorMatchesRegistry), but
// the ADR-0050 apicover floor is per-package: the accessor must also be named
// and exercised HERE, by the package that owns it.

import "testing"

// TestSignaturesReportsLiveRegistry asserts the two properties the bridge
// protect-list depends on: every reported entry is one the detector actually
// fires on, and the report tracks the LIVE registry rather than a
// construction-time snapshot (a promoted signature would otherwise go
// unprotected against a prompt that quotes it).
func TestSignaturesReportsLiveRegistry(t *testing.T) {
	det := SeedDetector()
	seeded := det.Signatures()
	if len(seeded) == 0 {
		t.Fatal("Signatures() on the seeded registry returned nothing")
	}
	for _, s := range seeded {
		if _, _, ok := det.Detect("preamble" + s + "\ntail"); !ok {
			t.Errorf("Signatures() reported %q but Detect does not fire on it", s)
		}
	}

	det.Promote(FatalSignature{Substr: "c1117 owner-package marker", Cause: CauseDeadShell, Note: "accessor liveness"})
	live := det.Signatures()
	if len(live) != len(seeded)+1 {
		t.Errorf("Signatures() returned %d entries after Promote, want %d — the accessor snapshots instead of reporting the live registry", len(live), len(seeded)+1)
	}
	if got := live[len(live)-1]; got != "c1117 owner-package marker" {
		t.Errorf("promoted signature = %q, want the promoted substring appended last (promotions must never shadow a seed)", got)
	}
}

// TestSignaturesSkipsEmptyAndNilReceiver covers the two boundaries a
// Contains-keyed consumer cannot survive: an empty substring (contained in
// every line — it would protect the whole pane and defeat the strip) and a nil
// detector.
func TestSignaturesSkipsEmptyAndNilReceiver(t *testing.T) {
	det := NewFatalPaneDetector([]FatalSignature{
		{Substr: "", Cause: CauseDeadShell},
		{Substr: "real", Cause: CauseDeadShell},
	})
	got := det.Signatures()
	if len(got) != 1 || got[0] != "real" {
		t.Errorf("Signatures() = %q, want only [real] — an empty substring must never reach a protect-list", got)
	}

	var nilDet *FatalPaneDetector
	if s := nilDet.Signatures(); s != nil {
		t.Errorf("nil detector Signatures() = %q, want nil", s)
	}
}
