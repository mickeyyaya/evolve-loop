package continuation

import (
	"fmt"

	"github.com/mickeyyaya/evolve-loop/go/internal/envchain"
)

const OperatorConfirmEnv = "EVOLVE_OPERATOR_CONFIRM"

func RequireOperatorAuthority(operator bool) error {
	if operator || envchain.Bool(OperatorConfirmEnv, nil, false) {
		return nil
	}
	return fmt.Errorf("releasing a continuation binding erases the lineage the defect-ledger gate reads as anti-tamper evidence, and needs operator authority: pass -operator, or set %s=1 in the environment", OperatorConfirmEnv)
}
