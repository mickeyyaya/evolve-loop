package agentrequest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/inputindex"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const (
	evidenceRequest = "request"
	evidenceInput   = "input"
	requestCopy     = "request.json"
)

func EvidenceWorkspace(cell Cell, name string) string {
	return filepath.Join(cell.ProjectRoot, ".evolve", "runs", "request-"+strconv.Itoa(cell.Cycle)+"-"+name)
}

func writeEvidence(p Pending, raw []byte) (Pending, error) {
	p.Evidence = EvidenceWorkspace(p.Cell, p.Name)
	copied := filepath.Join(p.Evidence, requestCopy)
	err := errors.Join(os.RemoveAll(p.Evidence), os.MkdirAll(p.Evidence, 0o755), os.WriteFile(copied, raw, 0o644))
	request, ok, entryErr := inputindex.FileEntry(p.Cell.ProjectRoot, copied, evidenceRequest, evidenceRequest, "the request that the "+p.Cell.Phase+" agent wrote")
	if entryErr == nil && !ok {
		entryErr = fmt.Errorf("%s is not a plain file of the project", copied)
	}
	if err = errors.Join(err, entryErr); err == nil {
		var idx inputindex.Index
		idx, p = evidenceIndex(p, request)
		err = inputindex.Write(p.Evidence, idx)
	}
	if err != nil {
		return p, fmt.Errorf("evidence of %s: %w", p.Name, err)
	}
	return p, nil
}

func evidenceIndex(p Pending, request inputindex.Entry) (inputindex.Index, Pending) {
	request.ID, request.Cycle, request.Required = "q1", p.Cell.Cycle, true
	idx := inputindex.Index{Schema: inputindex.Schema, Cycle: p.Cell.Cycle, Phase: phasecontract.FlowSeatSelfhealOrchestrator, ItemIDs: []string{}, Lineage: []int{},
		Required: []string{request.ID}, Entries: []inputindex.Entry{request}, Notes: []string{}}
	p.EntryIDs = []string{request.ID}
	for i, e := range p.entries {
		e.ID, e.Cycle = "i"+strconv.Itoa(i+1), p.Cell.Cycle
		idx.Entries, p.EntryIDs = append(idx.Entries, e), append(p.EntryIDs, e.ID)
		p.Inputs = append(p.Inputs, filepath.Join(p.Cell.ProjectRoot, filepath.FromSlash(e.Path)))
	}
	return idx, p
}
