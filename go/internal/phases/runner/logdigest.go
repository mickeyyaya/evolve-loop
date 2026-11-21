package runner

import (
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/logread"
)

func withLogDigest(prompt string, req core.PhaseRequest) string {
	if req.Spec == nil || req.Spec.LogAccess == nil {
		return prompt
	}
	reader := logread.Reader{RunDir: req.Workspace, Cycle: req.Cycle}
	return prompt + "\n\n## Logs (D0)\n" + strings.Join(reader.DispatchBlock(), "\n") + "\n"
}
