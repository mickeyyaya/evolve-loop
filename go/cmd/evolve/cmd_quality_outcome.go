package main

import (
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
	"github.com/mickeyyaya/evolve-loop/go/internal/pipelinerates"
	"github.com/mickeyyaya/evolve-loop/go/internal/qualitypanel"
)

const (
	churnWindow    = "commits:14-30d"
	revertWindow   = "commits:30d"
	recoveryWindow = "dossiers:all"
	gitLogSource   = "git log"
	recoverySource = "knowledge-base/cycles, .evolve/dossiers-pending"
)

func outcomeQualityReaders() []qualityReader {
	return []qualityReader{
		{index: "OQ2", read: func(root string) ([]qualitypanel.Reading, error) {
			res, err := qualitypanel.MeasureChurn(qualitypanel.ChurnInput{Repo: root, Now: time.Now(), Window: churnWindow, Source: gitLogSource})
			if err != nil {
				return nil, err
			}
			return []qualitypanel.Reading{res.OQ2}, nil
		}},
		{index: "OQ3", read: func(root string) ([]qualitypanel.Reading, error) {
			res, err := qualitypanel.MeasureRevertRate(qualitypanel.RevertRateInput{Repo: root, Now: time.Now(), Window: revertWindow, Source: gitLogSource})
			if err != nil {
				return nil, err
			}
			return []qualitypanel.Reading{res.OQ3}, nil
		}},
	}
}

func recoveryQualityReader() qualityReader {
	return qualityReader{index: "OQ4", read: func(root string) ([]qualitypanel.Reading, error) {
		all := append(dossier.ReadDir(dossier.CyclesDir(root), 0), dossier.ReadDir(dossier.PendingDir(root), 0)...)
		res := pipelinerates.MeasureRecovery(pipelinerates.RecoveryInput{Dossiers: all, Window: recoveryWindow, Source: recoverySource})
		return []qualitypanel.Reading{res.OQ4}, nil
	}}
}
