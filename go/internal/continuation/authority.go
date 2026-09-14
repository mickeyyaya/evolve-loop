package continuation

// authority.go — the operator-authority gate every path that can DROP a
// continuation binding shares.
//
// A binding is the lineage the defect-ledger gate reads as anti-tamper
// evidence (ADR-0085/0089, cycle-1285), and `evolve continuation release`
// shipped in cycle 1515 with no gate at all: its FlagSet declared only
// -project-root and the command dropped straight from arg-parsing to an
// unconditional release. The phase guard (internal/guards/phase.go) denies
// in-process Agent/Task dispatch during a cycle but says nothing about a Bash
// invocation of the evolve binary, so ANY Bash-capable process — an in-cycle
// agent included — could erase a live scope's lineage. The registry's
// "ORCHESTRATOR-side only" authority invariant (DeleteRegistryEntry's contract)
// was widened by omission, not by decision.
//
// The check lives HERE rather than in cmd/evolve so a future in-process caller
// cannot route around the gate the CLI honours — a private copy in the command
// is exactly the drift that produced audit cycle-1507's H2.

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
)

// OperatorConfirmEnv is the non-interactive authority path: the environment
// fallback for -operator, for a script or a console session that cannot pass
// the flag. Exported because operators type this name — one home for it, so
// the gate, the flag's help text and the docs cannot drift apart.
const OperatorConfirmEnv = "EVOLVE_OPERATOR_CONFIRM"

// RequireOperatorAuthority reports whether the caller may release a
// continuation binding: nil when it may, and guidance naming BOTH authority
// paths when it may not.
//
// operator is the caller's explicit authorization (the -operator flag).
// Without it the environment fallback is consulted through envchain.Bool,
// which reads the VALUE and not mere presence: OperatorConfirmEnv=0 is a
// refusal rather than consent, and a variable exported empty in a lane's
// environment never authorizes an erasure.
//
// The returned error IS the CLI's refusal text. Keeping the guidance in the
// helper is what makes the message and the gate impossible to drift apart.
func RequireOperatorAuthority(operator bool) error {
	if operator || envchain.Bool(OperatorConfirmEnv, nil, false) {
		return nil
	}
	return fmt.Errorf("releasing a continuation binding erases the lineage the defect-ledger gate reads as anti-tamper evidence, and needs operator authority: pass -operator, or set %s=1 in the environment", OperatorConfirmEnv)
}
