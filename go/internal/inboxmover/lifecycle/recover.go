package lifecycle

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

type RecoverResult struct {
	Recovered int
	Paths     []string
}

func (m *Mover) RecoverOrphans() (RecoverResult, error) {
	res := RecoverResult{Paths: []string{}}
	procDir := inboxbatch.ProcessingDir(m.inboxDir)
	if info, err := os.Stat(procDir); err != nil || !info.IsDir() {
		m.linef("recover-orphans: no processing/ dir — nothing to do")
		return res, nil
	}
	activeCycle, _ := m.activeCycle()
	if activeCycle == "" {
		activeCycle = "-1"
	}
	activeNum, _ := strconv.Atoi(activeCycle)
	for _, dir := range inboxbatch.ProcessingCycleDirs(m.inboxDir) {
		cycle, _ := inboxbatch.ParseProcessingCycle(filepath.Base(dir))
		if cycle == activeNum {
			m.linef("recover-orphans: cycle-%d/ is active — skipping", cycle)
			continue
		}
		n, paths := m.recoverDir(dir, cycle)
		res.Recovered += n
		res.Paths = append(res.Paths, paths...)
	}
	m.linef("recover-orphans: %d file(s) recovered", res.Recovered)
	return res, nil
}

func (m *Mover) recoverDir(dir string, cycle int) (int, []string) {
	cycleNum := strconv.Itoa(cycle)
	files, _ := jsonEntries(dir)
	consumed := inboxbatch.ConsumedRecords(m.inboxDir)
	n, paths := 0, []string(nil)
	for _, f := range files {
		base := f.Name()
		src := filepath.Join(dir, base)
		dest := filepath.Join(m.inboxDir, base)
		taskID := readTaskIDOrUnknown(src)
		if rec, retired := inboxbatch.CopyOf(consumed[taskID], readCreatedAt(src), base); retired {
			m.linef("recover-orphans: skipping %s (id %s already %s) — left in processing/cycle-%s/ for the operator", base, taskID, rec.Describe(), cycleNum)
			continue
		}
		if err := os.Rename(src, dest); err != nil {
			m.warn(fault{code: CodeReleaseMoveFailed, origin: "Mover.RecoverOrphans", cycle: cycle, legacy: "WARN: ",
				reason: fmt.Sprintf("recover-orphans: mv failed for %s (leaving in processing/): %v", base, err),
				fields: map[string]string{"task_id": taskID, "base": base, "err": err.Error(), "step": "recover_orphans"}})
			continue
		}
		m.linef("recovered: %s ← processing/cycle-%s/", base, cycleNum)
		m.ledgerLine(ledgerEntry{
			Action: "recover",
			TaskID: taskID,
			From:   ".evolve/inbox/processing/cycle-" + cycleNum + "/" + base,
			To:     ".evolve/inbox/" + base,
			Cycle:  intPtr(cycleNum),
			Reason: "orphan-recovery-cycle-not-active",
		})
		n++
		paths = append(paths, dest)
	}
	return n, paths
}

func readCreatedAt(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var doc struct {
		CreatedAt string `json:"created_at"`
	}
	if json.Unmarshal(raw, &doc) != nil {
		return ""
	}
	return doc.CreatedAt
}
