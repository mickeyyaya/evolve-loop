package isolation

import (
	"strconv"

	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
)

var invariantDocs = []string{
	"I-1 single launcher: a code path started an LLM binary outside the bridge launch API; fields.target names the binary (ADR-0154)",
	"I-2 hermetic environment: the allowlist would drop a launcher variable, and shadow keeps it (action=logged); sent at shadow only, while enforce sends ISOLATION_ENV_REFUSED. Fields: invariant, stage, action, cell, family, target, rule, source (ADR-0154)",
	"I-3 no nested agency: a cell tried to start an LLM binary or an evolve dispatch; fields.target names it (ADR-0154)",
	"I-4 pipeline-mediated communication: a cell tried to read raw output or to write its inputs; fields.target names the path (ADR-0154)",
	"I-5 read isolation: a cell tried to read outside its index, its worktree and its CLI files; fields.target names the path (ADR-0154)",
	"I-6 family-agnostic flow: a launch resolved through a compiled family default, not through routing data; fields.cell names the seat (ADR-0154)",
	"I-7 CLI-neutral guarantees: a gate or the fence stopped a write that no hook stopped; fields.target names the path (ADR-0154)",
	"I-8 typed flow decisions: a flow parser fell back to stdout or prose; fields.cell names the seat (ADR-0154)",
	"I-9 proven substitution: no run-time emitter; the conformance matrix TestE2ECycleCLIMatrix is its guard (ADR-0154)",
}

const (
	CodeContextRequiredUnopened signalcenter.Code = "ISOLATION_CONTEXT_REQUIRED_UNOPENED"
	CodeContextBudgetOver       signalcenter.Code = "ISOLATION_CONTEXT_BUDGET_OVER"
	CodeRequestRefused          signalcenter.Code = "ISOLATION_REQUEST_REFUSED"
	CodeRequestFailed           signalcenter.Code = "ISOLATION_REQUEST_FAILED"
	CodeEnvRefused              signalcenter.Code = "ISOLATION_ENV_REFUSED"
	CodeReadUnmeasured          signalcenter.Code = "ISOLATION_READ_UNMEASURED"
)

func ViolationCode(invariant int) signalcenter.Code {
	return signalcenter.Code("ISOLATION_I" + strconv.Itoa(invariant) + "_VIOLATION")
}

func init() {
	for i, doc := range invariantDocs {
		signalcenter.RegisterCode(signalcenter.ModuleIsolation, ViolationCode(i+1), doc)
	}
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeContextRequiredUnopened, "the cell context index: a cell closed without a read of a required entry; fields.unopened lists the ids, and fields.source is sandbox (cell-context-index.md §8)")
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeContextBudgetOver, "the cell context index: required entries of a layer passed its token budget and lost the flag; fields layer, limit_tokens, over (cell-context-index.md §6)")
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeRequestRefused, "the request channel refused a cell request through validator (schema, kind, role, input, cap, or depth) or orchestrator agent. Fields: request, kind, role, decision, precondition, reason, stage (cli-contract.md §4.5, llm-isolation.md §7.2.1)")
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeRequestFailed, "the request channel approved a request of a cell, and the dispatch of the agent failed; the same fields as ISOLATION_REQUEST_REFUSED (llm-isolation.md §7.2.1)")
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeEnvRefused, "I-2 hermetic environment at enforce: the allowlist refused launcher variables that no earlier dispatch of the process reported; one INFO event for each dispatch. Fields: invariant, action, stage, cell, family, rule, source, count, names (names holds no value and is capped below the log field limit, with a +N more tail) (llm-isolation.md I-2)")
	signalcenter.RegisterCode(signalcenter.ModuleIsolation, CodeReadUnmeasured, "the cell read report is incomplete, so its reads count as unmeasured and no open rate counts the cell; fields cell, family, stage, cause (fence-deadline, cancelled, fence-write, report-read), elapsed (only for fence-deadline and cancelled). A cancelled launch is INFO because an operator stop caused it; the other causes are WARN (cell-context-index.md §8)")
}
