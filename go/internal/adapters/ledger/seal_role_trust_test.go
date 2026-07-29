package ledger

// seal_role_trust_test.go — cycle-1194 continuation (ADR-0081 audit defect):
// the epoch-anchor resolver must trust an in-band seal ONLY when its Role is
// exactly "operator" — the marker core.SealCycle writes for an explicit human
// `evolve cycle reset`. A line whose Role is "operator-autoseal" (what the
// unattended boot self-heal path, core.AutosealStaleMarker, now writes) must
// NOT move the anchor, even when it is itself perfectly hash-valid — hash
// validity alone proves nothing about WHO wrote the line, only that the bytes
// chain correctly, which requires no secret at all.

import "testing"

// autosealLine builds a hash-valid line shaped exactly like a real in-band
// seal, but with the automated-recovery role instead of "operator".
func autosealLine(prevSHA string) string {
	return `{"ts":"2026-05-01T00:04:00Z","cycle":0,"cycle_label":"reset-seal-cycle-4",` +
		`"role":"operator-autoseal","kind":"reset","exit_code":0,"entry_seq":4,"prev_hash":"` + prevSHA + `"}`
}

// operatorSealLine builds the genuine human-sealed counterpart.
func operatorSealLine(prevSHA string) string {
	return `{"ts":"2026-05-01T00:04:00Z","cycle":0,"cycle_label":"reset-seal-cycle-4",` +
		`"role":"operator","kind":"reset","exit_code":0,"entry_seq":4,"prev_hash":"` + prevSHA + `"}`
}

// TestEffectiveAnchorSHA_AutosealRole_DoesNotMoveAnchor is the regression
// guard: appending a hash-valid "operator-autoseal" line must NOT advance the
// epoch anchor, because that role is reachable by anything that can kill the
// owning process — no human sign-off.
func TestEffectiveAnchorSHA_AutosealRole_DoesNotMoveAnchor(t *testing.T) {
	lines, _ := chainLines()
	extended := append(append([]string(nil), lines...), autosealLine(sha256Of(lines[len(lines)-1])))
	got := effectiveAnchorSHA(bytesLines(extended), "")
	if got != "" {
		t.Errorf("an operator-autoseal line must not move the anchor; got %q", got)
	}
}

// TestEffectiveAnchorSHA_OperatorRole_MovesAnchor is the positive control: a
// genuine human-role seal, identical in every other respect, DOES move the
// anchor — proving the split is on Role, not some other field.
func TestEffectiveAnchorSHA_OperatorRole_MovesAnchor(t *testing.T) {
	lines, _ := chainLines()
	last := sha256Of(lines[len(lines)-1])
	sealLine := operatorSealLine(last)
	extended := append(append([]string(nil), lines...), sealLine)
	got := effectiveAnchorSHA(bytesLines(extended), "")
	if want := sha256Of(sealLine); got != want {
		t.Errorf("an operator-role seal must move the anchor to itself; got %q want %q", got, want)
	}
}
