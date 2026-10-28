package inputindex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

type Record struct {
	Cycle   int      `json:"cycle"`
	RunID   string   `json:"run_id,omitempty"`
	ItemIDs []string `json:"item_ids"`
}

func recordsDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "lineage")
}

func RequestsDir(projectRoot string, cycle int) string {
	return filepath.Join(recordsDir(projectRoot), requestsDirName, "cycle-"+strconv.Itoa(cycle))
}

func recordPath(projectRoot string, cycle int) string {
	return filepath.Join(recordsDir(projectRoot), "cycle-"+strconv.Itoa(cycle)+".json")
}

func WriteRecord(projectRoot string, r Record) error {
	dir := recordsDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("inputindex: make %s: %w", dir, err)
	}
	return atomicwrite.JSON(recordPath(projectRoot, r.Cycle), r)
}

func ReadRecord(projectRoot string, cycle int) (Record, bool) {
	r, err := loadRecord(recordPath(projectRoot, cycle))
	return r, err == nil
}

func loadRecord(path string) (Record, error) {
	var r Record
	raw, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(raw, &r)
	}
	return r, err
}

func readRecords(projectRoot string) ([]Record, []string) {
	paths, _ := filepath.Glob(filepath.Join(recordsDir(projectRoot), "cycle-*.json"))
	var records []Record
	var notes []string
	for _, path := range paths {
		r, err := loadRecord(path)
		if err != nil {
			notes = append(notes, "the lineage record "+filepath.Base(path)+" is unreadable, so its cycle is not in the lineage: "+err.Error())
			continue
		}
		records = append(records, r)
	}
	return records, notes
}
