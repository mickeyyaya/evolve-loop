package modelquery

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
)

// decisionVersion namespaces the fingerprint to the CURRENT classification +
// promotion algorithm. Bump it by hand whenever buildClassifyPrompt, the tier
// vocabulary semantics, or the PromoteLatest/CompleteTiers algorithm changes —
// that is what makes the reuse gate correct rather than merely fast: without
// it, an algorithm fix would be silently reused away for every CLI whose id
// list happens to be unchanged.
const decisionVersion = "v1"

// FingerprintInput is everything the classify+promote decision depends on for
// one CLI. Two equal fingerprints mean the decision inputs are identical and
// the prior tier map may be reused without a classifier call.
type FingerprintInput struct {
	// CLI is the base CLI name the candidates belong to.
	CLI string
	// Candidates are the family-filtered model ids offered to the classifier.
	// Order-insensitive: pane order is presentation, not identity.
	Candidates []string
	// Policy is the CLI's freshness policy (part of the promotion decision).
	Policy FreshnessPolicy
	// Tiers is the canonical tier vocabulary the prompt asks for.
	Tiers []string
}

// Fingerprint renders in canonically ("sha256:<hex>"). Every field and every
// list member is length-prefixed and NUL-separated, so adjacent values cannot
// be re-split into a colliding rendering (agy ids contain spaces and parens —
// a bare separator join would be ambiguous). Candidates and Tiers are sorted
// on copies; the caller's slices are never mutated.
func Fingerprint(in FingerprintInput) string {
	h := sha256.New()
	writeField := func(s string) {
		fmt.Fprintf(h, "%d:%s\x00", len(s), s)
	}
	writeList := func(items []string) {
		sorted := append([]string(nil), items...)
		sort.Strings(sorted)
		writeField(strconv.Itoa(len(sorted)))
		for _, it := range sorted {
			writeField(it)
		}
	}
	writeField(decisionVersion)
	writeField(in.CLI)
	writeList(in.Candidates)
	writeField(strconv.FormatBool(in.Policy.PreferAlias))
	writeList(in.Policy.AliasIDs)
	writeList(in.Tiers)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
