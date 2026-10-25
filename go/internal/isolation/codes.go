package isolation

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var invariantDocs = []string{
	"I-1 single launcher: a code path started an LLM binary outside the bridge launch API; fields.target names the binary (ADR-0154)",
	"I-2 hermetic environment: the allowlist dropped a variable of the launcher; in shadow the child still got it (action=logged), in enforce it did not (action=refused); fields invariant, stage, action, cell, family, target (the name, never its value), rule, source (ADR-0154)",
	"I-3 no nested agency: a cell tried to start an LLM binary or an evolve dispatch; fields.target names it (ADR-0154)",
	"I-4 pipeline-mediated communication: a cell tried to read raw output or to write its inputs; fields.target names the path (ADR-0154)",
	"I-5 read isolation: a cell tried to read outside its index, its worktree and its CLI files; fields.target names the path (ADR-0154)",
	"I-6 family-agnostic flow: a launch resolved through a compiled family default, not through routing data; fields.cell names the seat (ADR-0154)",
	"I-7 CLI-neutral guarantees: a gate or the fence stopped a write that no hook stopped; fields.target names the path (ADR-0154)",
	"I-8 typed flow decisions: a flow parser fell back to stdout or prose; fields.cell names the seat (ADR-0154)",
	"I-9 proven substitution: no run-time emitter; the conformance matrix TestE2ECycleCLIMatrix is its guard (ADR-0154)",
}

func ViolationCode(invariant int) signalcenter.Code {
	return signalcenter.Code("ISOLATION_I" + strconv.Itoa(invariant) + "_VIOLATION")
}

func init() {
	for i, doc := range invariantDocs {
		signalcenter.RegisterCode(signalcenter.ModuleIsolation, ViolationCode(i+1), doc)
	}
}
