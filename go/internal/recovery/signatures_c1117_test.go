package recovery

import "testing"

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
