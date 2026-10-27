package continuation

import "errors"

var errNoOperatorAuthority = errors.New("releasing a continuation binding erases the lineage the defect-ledger gate reads as anti-tamper evidence, and needs operator authority: pass -operator")

func RequireOperatorAuthority(operator bool) error {
	if operator {
		return nil
	}
	return errNoOperatorAuthority
}
