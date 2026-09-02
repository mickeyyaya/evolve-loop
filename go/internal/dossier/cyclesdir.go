package dossier

import "path/filepath"

// CyclesDir is the committed home of the per-cycle dossiers,
// <projectRoot>/knowledge-base/cycles — a protocol-committed corpus
// (ADR-0094: tracked, read back by the dossier-closeout gate and the recent
// outcomes chronicle). The producer (core.dossier_producer), the chronicle
// seed (core.cyclerun_chronicle), ReadCommitted, `evolve dossier verify` and
// the dashboard's ship-rate history resolve the directory here.
// internal/contextfillcorrelate still spells the join inline (it is a
// deliberate leaf package that does not import dossier) — an unmigrated
// reader to carry along on any move, not a second owner.
func CyclesDir(projectRoot string) string {
	return filepath.Join(projectRoot, "knowledge-base", "cycles")
}
