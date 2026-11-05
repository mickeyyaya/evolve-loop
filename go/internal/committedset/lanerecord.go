package committedset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

type Record struct {
	Cycle     int      `json:"cycle"`
	RunID     string   `json:"run_id,omitempty"`
	ItemIDs   []string `json:"item_ids"`
	GoalHash  string   `json:"goal_hash,omitempty"`
	RoutedIDs []string `json:"routed_ids,omitempty"`
}

type Drift struct {
	Path   string
	Record []string
	File   []string
}

var workspaceRe = regexp.MustCompile(`^cycle-(\d+)$`)

func RecordsDir(projectRoot string) string {
	return filepath.Join(projectRoot, ".evolve", "lineage")
}

func RecordPath(projectRoot string, cycle int) string {
	return filepath.Join(RecordsDir(projectRoot), "cycle-"+strconv.Itoa(cycle)+".json")
}

func WriteRecord(projectRoot string, r Record) error {
	dir := RecordsDir(projectRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("committedset: make %s: %w", dir, err)
	}
	return atomicwrite.JSON(RecordPath(projectRoot, r.Cycle), r)
}

func ReadRecord(projectRoot string, cycle int) (Record, bool) {
	r, err := LoadRecord(RecordPath(projectRoot, cycle))
	return r, err == nil
}

func LoadRecord(path string) (Record, error) {
	var r Record
	raw, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(raw, &r)
	}
	return r, err
}

func workspaceTarget(workspace string) (projectRoot string, cycle int, ok bool) {
	m := workspaceRe.FindStringSubmatch(filepath.Base(workspace))
	runs := filepath.Dir(workspace)
	evolve := filepath.Dir(runs)
	if m == nil || filepath.Base(runs) != "runs" || filepath.Base(evolve) != ".evolve" {
		return "", 0, false
	}
	cycle, err := strconv.Atoi(m[1])
	if err != nil {
		return "", 0, false
	}
	return filepath.Dir(evolve), cycle, true
}

func workspaceRecord(workspace string) (Record, bool) {
	root, cycle, ok := workspaceTarget(workspace)
	if !ok {
		return Record{}, false
	}
	return ReadRecord(root, cycle)
}

func ReadLaneDoc(workspace string) (LanePinDoc, error) {
	if workspace == "" {
		return LanePinDoc{}, os.ErrNotExist
	}
	if r, ok := workspaceRecord(workspace); ok {
		return LanePinDoc{TodoIDs: nonBlank(r.ItemIDs), GoalHash: r.GoalHash}, nil
	}
	body, err := os.ReadFile(filepath.Join(workspace, LanePinFile))
	if err != nil {
		return LanePinDoc{}, err
	}
	var doc LanePinDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return LanePinDoc{}, fmt.Errorf("parse %s: %w", LanePinFile, err)
	}
	doc.TodoIDs = nonBlank(doc.TodoIDs)
	return doc, nil
}

func LaneDoc(workspace string) (LanePinDoc, bool) {
	doc, err := ReadLaneDoc(workspace)
	return doc, err == nil
}

func LaneDrift(workspace string) (Drift, bool) {
	r, ok := workspaceRecord(workspace)
	if !ok {
		return Drift{}, false
	}
	var file LanePinDoc
	if !readJSON(workspace, LanePinFile, &file) {
		return Drift{}, false
	}
	d := Drift{Path: filepath.Join(workspace, LanePinFile), Record: nonBlank(r.ItemIDs), File: nonBlank(file.TodoIDs)}
	return d, !slices.Equal(d.Record, d.File)
}

func RecordRoutes(projectRoot string, cycle int, ids []string) error {
	r, ok := ReadRecord(projectRoot, cycle)
	if !ok {
		return fmt.Errorf("committedset: no lineage record for cycle %d under %s", cycle, projectRoot)
	}
	for _, id := range nonBlank(ids) {
		if !slices.Contains(r.RoutedIDs, id) {
			r.RoutedIDs = append(r.RoutedIDs, id)
		}
	}
	return WriteRecord(projectRoot, r)
}

func RecordWorkspaceRoutes(workspace string, ids []string) error {
	root, cycle, ok := workspaceTarget(workspace)
	if !ok {
		return fmt.Errorf("committedset: workspace %s is not a cycle workspace", workspace)
	}
	return RecordRoutes(root, cycle, ids)
}

func RoutedIDs(workspace string) []string {
	r, ok := workspaceRecord(workspace)
	if !ok {
		return nil
	}
	return slices.Clone(r.RoutedIDs)
}

func UnroutedPin(workspace string) []string {
	routed := RoutedIDs(workspace)
	var pin []string
	for _, id := range LanePin(workspace) {
		if !slices.Contains(routed, id) {
			pin = append(pin, id)
		}
	}
	return pin
}
