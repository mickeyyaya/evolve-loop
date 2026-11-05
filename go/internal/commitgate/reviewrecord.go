package commitgate

import (
	"encoding/json"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/launchrecord"
)

type ReviewRecord struct {
	Attestation   Attestation           `json:"attestation"`
	LaunchRecords []launchrecord.Record `json:"launch_records"`
}

func (o Options) writeReviewRecord(att *Attestation, res *Result) int {
	body, err := json.MarshalIndent(ReviewRecord{Attestation: *att, LaunchRecords: res.launchRecords}, "", "  ")
	if err == nil {
		err = atomicwrite.Bytes(filepath.Join(o.ReviewDir, att.TreeStateSHA+".json"), append(body, '\n'))
	}
	if err != nil {
		res.log("cannot write review record: %v", err)
		return ExitGitFatal
	}
	return ExitPass
}
