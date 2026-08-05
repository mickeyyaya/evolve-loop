package ciparity

// graduation_prescription.go — cycle-1329 (audit-warn-prescription-gate):
// GraduationPrescription is the shared home for "what does an ungraduated
// package need" text, relocated here (exported) from the build-entry seam's
// package-private graduationPrescription (phase_bindings_graduation.go:81)
// so BOTH the build seam (core) and the audit seam (phases/audit) render the
// identical copy-pasteable fix. Both callers already import ciparity for
// NewUngraduatedPackages — the shared detection predicate — so this is the
// natural single home rather than audit importing core for one helper.
//
// Behavior is a byte-identical move: same two-step-per-package text, same
// "..."-suffixed recursive-pattern branch (a pattern names no single
// directory, so step 2 falls back to "EACH package the pattern covers"
// instead of a bogus ".../apicover_named_test.go" path).

import (
	"fmt"
	"strings"
)

// GraduationPrescription renders the exact two edits per ungraduated
// package: the go/.apicover-enforce line to append and the
// apicover_named_test.go path to create, copy-pasteable, derived from the
// same "./internal/<pkg>" enforce-list form the detector matched on so the
// prescription cannot name a package the check did not flag. Empty input
// returns "" — no ungraduated packages means nothing to prescribe.
func GraduationPrescription(pkgs []string) string {
	var b strings.Builder
	for _, pkg := range pkgs {
		fmt.Fprintf(&b, "  %s:\n", pkg)
		fmt.Fprintf(&b, "    1. append this line to go/.apicover-enforce:  %s\n", pkg)
		// A recursive pattern names no single directory, so prescribing
		// "<pattern>/apicover_named_test.go" would hand the builder an invalid
		// path (review LOW). Name the obligation without the bogus path.
		if strings.Contains(pkg, "...") {
			b.WriteString("    2. add an apicover_named_test.go in EACH package the pattern covers, naming every exported symbol in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)\n")
			continue
		}
		dir := "go/" + strings.TrimPrefix(pkg, "./")
		fmt.Fprintf(&b, "    2. create %s/apicover_named_test.go naming every exported symbol of the package in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)\n", dir)
	}
	return strings.TrimRight(b.String(), "\n")
}
