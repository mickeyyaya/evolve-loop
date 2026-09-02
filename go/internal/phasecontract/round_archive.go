package phasecontract

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// RoundArchiveFilename is the single home of the round-archive naming rule for
// a phase artifact that is regenerated across in-cycle repair rounds:
// `audit-report.md` retired after round 2 becomes `audit-report.round2.md`,
// `acs-verdict.json` becomes `acs-verdict.round2.json`. The writer
// (core.retireSupersededAuditArtifacts) and every reader of the archives (the
// dashboard's repair-round history, the repair-brief seed's "persisted from the
// previous round" set) derive the name here, so a rename cannot orphan one
// side — the cycle-1145 class the registry exists to prevent.
func RoundArchiveFilename(name string, round int) string {
	ext := filepath.Ext(name)
	return fmt.Sprintf("%s.round%d%s", name[:len(name)-len(ext)], round, ext)
}

// ParseRoundArchive is the inverse of RoundArchiveFilename: given a file name
// and the live artifact it may archive, it returns the round index. Readers
// list a workspace and parse rather than probing round 1, 2, 3 … in sequence,
// because the writer does not guarantee contiguous indices — an audit dispatch
// that died before writing its report archives nothing at its index while the
// dispatch counter still advances.
func ParseRoundArchive(filename, liveName string) (round int, ok bool) {
	ext := filepath.Ext(liveName)
	stem := liveName[:len(liveName)-len(ext)] + ".round"
	if !strings.HasPrefix(filename, stem) || !strings.HasSuffix(filename, ext) {
		return 0, false
	}
	digits := filename[len(stem) : len(filename)-len(ext)]
	n, err := strconv.Atoi(digits)
	if err != nil || n < 1 || digits != strconv.Itoa(n) {
		return 0, false
	}
	return n, true
}
