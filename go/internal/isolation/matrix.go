package isolation

import (
	"fmt"
	"slices"
)

type Row struct {
	ID        string
	Check     string
	Invariant string
	ClosedBy  string
	Open      bool
	Owner     string
}

const (
	ownerMatrix   = "cmd/evolve/e2e_cycle_cli_matrix_test.go"
	ownerContract = "internal/bridge/sandbox_contract_integration_test.go"
)

var rows = []Row{
	{ID: "M1", Check: "a full simulated cycle with claude as the only route", Invariant: "I-9", ClosedBy: "X1", Owner: ownerMatrix},
	{ID: "M2", Check: "a full simulated cycle with agy as the only route", Invariant: "I-9", ClosedBy: "X6", Open: true, Owner: ownerMatrix},
	{ID: "M3", Check: "a full simulated cycle with agy-claude as the only route", Invariant: "I-9", ClosedBy: "X6", Open: true, Owner: ownerMatrix},
	{ID: "M4", Check: "a full simulated cycle with codex as the only route", Invariant: "I-9", ClosedBy: "X6, X9", Open: true, Owner: ownerMatrix},
	{ID: "M5", Check: "a full simulated cycle with ollama as the only route", Invariant: "I-9", ClosedBy: "X6, X9", Open: true, Owner: ownerMatrix},
	{ID: "N1", Check: "CLAUDECODE=1 on the parent does not reach any seat", Invariant: "I-2", ClosedBy: "X2", Open: true, Owner: ownerMatrix},
	{ID: "N2", Check: "no LLM spawn outside the bridge launch API", Invariant: "I-1", ClosedBy: "X3", Open: true, Owner: "internal/sysexec/isolation_matrix_test.go"},
	{ID: "N3", Check: "a phase pane cannot exec claude --version", Invariant: "I-3", ClosedBy: "X4", Open: true, Owner: ownerContract},
	{ID: "N4", Check: "evolve agent run inside a phase pane is refused", Invariant: "I-3", ClosedBy: "X4", Open: true, Owner: ownerMatrix},
	{ID: "N5", Check: "a cell cannot read <ws>/.bridge/scout-stdout.log", Invariant: "I-4", ClosedBy: "X5", Open: true, Owner: ownerContract},
	{ID: "N6", Check: "a retry opens its predecessor's retrospective-report.md through the index", Invariant: "I-4, I-5", ClosedBy: "X5", Open: true, Owner: ownerContract},
	{ID: "N7", Check: "a retry cannot open an unrelated cycle's audit-report.md", Invariant: "I-5", ClosedBy: "X5", Open: true, Owner: ownerContract},
	{ID: "N8", Check: "a cell cannot read ~/.ssh", Invariant: "I-5", ClosedBy: "X5", Open: true, Owner: ownerContract},
	{ID: "N9", Check: "the network row records today's behavior (allowed)", Invariant: "none", ClosedBy: "X1", Owner: ownerContract},
	{ID: "N10", Check: "an agy pane cannot write a protected path", Invariant: "I-7", ClosedBy: "X7", Open: true, Owner: ownerContract},
	{ID: "N11", Check: "guard phase denies Task", Invariant: "I-7", ClosedBy: "X7", Open: true, Owner: "internal/guards/isolation_matrix_test.go"},
	{ID: "N12", Check: "a judge verdict in prose with no file fails the contract", Invariant: "I-8", ClosedBy: "X8", Open: true, Owner: "internal/core/isolation_matrix_test.go"},
	{ID: "N13", Check: "no family string in flow code", Invariant: "I-6", ClosedBy: "X6", Open: true, Owner: "internal/core/isolation_matrix_test.go"},
	{ID: "N14", Check: "a new manifest with known format ids needs no Go change", Invariant: "I-6", ClosedBy: "X9", Open: true, Owner: "internal/bridge/isolation_matrix_test.go"},
}

func Rows() []Row { return slices.Clone(rows) }

func Lookup(id string) (Row, bool) {
	i := slices.IndexFunc(rows, func(r Row) bool { return r.ID == id })
	if i < 0 {
		return Row{}, false
	}
	return rows[i], true
}

func Judge(id string, failure error) (string, error) {
	r, ok := Lookup(id)
	switch {
	case !ok:
		return "", fmt.Errorf("isolation matrix: no row %q; add it to isolation.Rows and to docs/architecture/llm-isolation.md §14", id)
	case !r.Open && failure != nil:
		return "", fmt.Errorf("row %s (%s) must pass: %w", r.ID, r.Check, failure)
	case !r.Open:
		return fmt.Sprintf("row %s green: %s", r.ID, r.Check), nil
	case failure == nil:
		return "", fmt.Errorf("row %s (%s) passed, but it is an expected failure until %s lands: set Open to false in isolation.Rows and mark the row green in docs/architecture/llm-isolation.md §14", r.ID, r.Check, r.ClosedBy)
	default:
		return fmt.Sprintf("row %s: expected failure until %s: %v", r.ID, r.ClosedBy, failure), nil
	}
}
