package ledger

import "testing"

// autosealLine is hash-valid and shaped exactly like a real in-band seal, but with the autoseal role.
func autosealLine(prevSHA string) string {
	return `{"ts":"2026-05-01T00:04:00Z","cycle":0,"cycle_label":"reset-seal-cycle-4",` +
		`"role":"operator-autoseal","kind":"reset","exit_code":0,"entry_seq":4,"prev_hash":"` + prevSHA + `"}`
}

func operatorSealLine(prevSHA string) string {
	return `{"ts":"2026-05-01T00:04:00Z","cycle":0,"cycle_label":"reset-seal-cycle-4",` +
		`"role":"operator","kind":"reset","exit_code":0,"entry_seq":4,"prev_hash":"` + prevSHA + `"}`
}

func TestEffectiveAnchorSHA_AutosealRole_DoesNotMoveAnchor(t *testing.T) {
	lines, _ := chainLines()
	extended := append(append([]string(nil), lines...), autosealLine(sha256Of(lines[len(lines)-1])))
	got, _ := effectiveAnchorSHA(bytesLines(extended), "")
	if got != "" {
		t.Errorf("an operator-autoseal line must not move the anchor; got %q", got)
	}
}

func TestEffectiveAnchorSHA_OperatorRole_MovesAnchor(t *testing.T) {
	lines, _ := chainLines()
	last := sha256Of(lines[len(lines)-1])
	sealLine := operatorSealLine(last)
	extended := append(append([]string(nil), lines...), sealLine)
	got, _ := effectiveAnchorSHA(bytesLines(extended), "")
	if want := sha256Of(sealLine); got != want {
		t.Errorf("an operator-role seal must move the anchor to itself; got %q want %q", got, want)
	}
}
