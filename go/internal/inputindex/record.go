package inputindex

import (
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/committedset"
)

type Record = committedset.Record

func RequestsDir(projectRoot string, cycle int) string {
	return filepath.Join(committedset.RecordsDir(projectRoot), requestsDirName, "cycle-"+strconv.Itoa(cycle))
}

func WriteRecord(projectRoot string, r Record) error {
	return committedset.WriteRecord(projectRoot, r)
}

func ReadRecord(projectRoot string, cycle int) (Record, bool) {
	return committedset.ReadRecord(projectRoot, cycle)
}

func readRecords(projectRoot string) ([]Record, []string) {
	paths, _ := filepath.Glob(filepath.Join(committedset.RecordsDir(projectRoot), "cycle-*.json"))
	var records []Record
	var notes []string
	for _, path := range paths {
		r, err := committedset.LoadRecord(path)
		if err != nil {
			notes = append(notes, "the lineage record "+filepath.Base(path)+" is unreadable, so its cycle is not in the lineage: "+err.Error())
			continue
		}
		records = append(records, r)
	}
	return records, notes
}
