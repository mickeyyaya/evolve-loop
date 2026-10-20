// Package ledgertip holds the one encoding of .evolve/ledger.tip that every ledger writer and the verifier share.
// See docs/architecture/packages/internal-ledgertip.md.
package ledgertip

import "fmt"

func Format(seq int, lineSHA string) string {
	return fmt.Sprintf("%d:%s", seq, lineSHA)
}
