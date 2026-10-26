package inputindex

import (
	"slices"
	"testing"
)

func cellRecord(phase, source string, listed []AccessRow, opened ...string) AccessRecord {
	return AccessRecord{Type: AccessCell, Phase: phase, Source: source, Listed: listed, Opened: opened}
}

func TestStats_CountsTheSandboxCellsThatListedAndOpenedEachKindAndLeavesOutAScan(t *testing.T) {
	rows := []AccessRow{{ID: "d1", Kind: "scout-report", Layer: LayerUpstream, Required: true}, {ID: "d2", Kind: "test-report", Layer: LayerUpstream}, {ID: "l3", Kind: "retrospective", Layer: LayerRecovery}}
	records := []AccessRecord{
		cellRecord("build", AccessSourceSandbox, rows, "d1"),
		cellRecord("build", AccessSourceSandbox, rows, "d1", "d2"),
		cellRecord("build", AccessSourceSandbox, rows, "d1", "d2", "l3"),
		cellRecord("build", AccessSourceNone, rows),
		cellRecord("audit", AccessSourceSandbox, rows[:1]),
		{Type: AccessOpen, Phase: "build", Source: AccessSourceSandbox, Entry: "d1", Kind: "scout-report"},
	}
	got := Stats(records, "build")
	want := []KindStat{
		{Phase: "build", Kind: "retrospective", Layer: LayerRecovery, Listed: 2, Opened: 0, Required: 0, Rate: 0},
		{Phase: "build", Kind: "scout-report", Layer: LayerUpstream, Listed: 2, Opened: 2, Required: 2, Rate: 1},
		{Phase: "build", Kind: "test-report", Layer: LayerUpstream, Listed: 2, Opened: 1, Required: 0, Rate: 0.5},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Stats(build) =\n%+v\nwant\n%+v\nthe source none cell and the scan cell that opened each row are left out", got, want)
	}
	if all := Stats(records, ""); len(all) != 4 || all[0].Phase != "audit" || all[0].Listed != 1 || all[0].Rate != 0 {
		t.Fatalf("Stats of each phase = %+v", all)
	}
	if one := Stats([]AccessRecord{cellRecord("build", AccessSourceSandbox, rows[:1], "d1")}, "build"); one[0].Opened != 1 {
		t.Fatalf("a cell with one row that opened it is not a scan: %+v", one)
	}
}
