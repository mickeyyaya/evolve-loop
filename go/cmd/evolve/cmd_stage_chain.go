package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/flock"
	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/ledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/stageevidence"
)

type stageEvidenceSite struct {
	root    string
	ledger  *ledger.FileLedger
	signals *signalcenter.Center
}

func newStageEvidenceSite(root string, ld *ledger.FileLedger, signals *signalcenter.Center) stageEvidenceSite {
	return stageEvidenceSite{root: root, ledger: ld, signals: signals}
}

func (s stageEvidenceSite) coreEvidence() core.StageEvidence {
	return core.StageEvidence{
		Open:   func(cycle int) core.StageRecorder { return s.open(cycle) },
		Sealer: s.ledger,
	}
}

func (s stageEvidenceSite) shipOpener() ship.StageOpener {
	return func(cycle int) core.StageRecorder { return s.open(cycle) }
}

func (s stageEvidenceSite) open(cycle int) *stageevidence.Recorder {
	scope := stageevidence.CycleScope(cycle)
	return stageevidence.NewRecorder(stageevidence.RecorderConfig{
		EvolveDir: paths.EvolveDirOf(s.root),
		Scope:     scope,
		Session:   stageSessionID(),
		Build:     stageBuildID(),
		Lock:      flock.Lock,
		Version:   s.versionOf(),
		OnFault:   func(err error) { s.fault(cycle, err) },
	})
}

var errUnknownStageKey = errors.New("unknown stage key")

func (s stageEvidenceSite) versionOf() func(key string) (string, error) {
	policyBytes, policyErr := os.ReadFile(filepath.Join(paths.EvolveDirOf(s.root), "policy.json"))
	return func(key string) (string, error) {
		if policyErr != nil {
			return "", fmt.Errorf("read policy.json: %w", policyErr)
		}
		entry, ok := lookupStageEntry(key)
		if !ok {
			return "", fmt.Errorf("%w %q", errUnknownStageKey, key)
		}
		return stageevidence.DecisionVersion(filepath.Join(s.root, "go"), entry.Shadow.Closure, policyBytes, entry.Shadow.PolicySubtree)
	}
}

func (s stageEvidenceSite) fault(cycle int, err error) {
	s.signals.Emit(core.StageFaultEvent(cycle, "stageEvidenceSite.fault", "written", err))
}

var stageSessionSeq atomic.Uint64

func stageSessionID() string {
	return fmt.Sprintf("p%d-%x-%d", os.Getpid(), time.Now().UnixNano(), stageSessionSeq.Add(1))
}

func stageBuildID() string {
	info, ok := debug.ReadBuildInfo()
	return buildIDOf(info, ok)
}

func buildIDOf(info *debug.BuildInfo, ok bool) string {
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" && setting.Value != "" {
			return setting.Value
		}
	}
	return "unknown"
}

func stageLedgerReader(root string) stageevidence.ChainReader {
	return ledger.New(paths.EvolveDirOf(root)).StageChain()
}
