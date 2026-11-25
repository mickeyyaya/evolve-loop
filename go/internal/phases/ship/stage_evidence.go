package ship

import "github.com/mickeyyaya/evolve-loop/go/internal/core"

const StageKeyHeldProposals = "held_proposals.stage"

type StageOpener func(cycle int) core.StageRecorder

func (p *Phase) StageEvidenceWired() bool { return p.stageEvidence != nil }

func (o *Options) recordStage(cycle int, key string, fields map[string]string) {
	if o.StageEvidence == nil {
		return
	}
	session := o.StageEvidence(cycle)
	session.Record(key, fields)
	session.Close()
}
