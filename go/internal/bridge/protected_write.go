package bridge

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/sandbox"
	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func (deps Deps) guaranteeStage() string {
	return deps.isolationStage(deps.GuaranteeStage, policy.Policy.IsolationGuaranteeStage)
}

func ownWorktree(req SandboxWrapRequest, realWritePaths []string) string {
	if req.Worktree == "" {
		return ""
	}
	return realWritePaths[0]
}

func (deps Deps) withProtectedWrites(cfg sandbox.Config, realWorktree, realRoot string) sandbox.Config {
	stage := deps.guaranteeStage()
	if stage == envStageOff {
		return cfg
	}
	out := cfg
	out.ProtectedWriteSubpaths, out.ProtectedWritePrefixes = slices.Clone(cfg.ProtectedWriteSubpaths), slices.Clone(cfg.ProtectedWritePrefixes)
	out.ProtectedWriteEnforce = stage == envStageEnforce
	bases := []string{realWorktree}
	if out.ProtectedWriteEnforce {
		bases = append(bases, realRoot)
	}
	for _, base := range bases {
		if base == "" {
			continue
		}
		for _, e := range guards.ProtectedSurfaceManifest {
			path := filepath.Join(base, filepath.FromSlash(e.Fragment))
			if strings.HasSuffix(e.Fragment, "/") {
				out.ProtectedWriteSubpaths = append(out.ProtectedWriteSubpaths, path)
			} else {
				out.ProtectedWritePrefixes = append(out.ProtectedWritePrefixes, path)
			}
		}
	}
	return out
}
