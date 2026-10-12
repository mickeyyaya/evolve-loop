package ciparity

import (
	"fmt"
	"strings"
)

func GraduationPrescription(pkgs []string) string {
	var b strings.Builder
	for _, pkg := range pkgs {
		fmt.Fprintf(&b, "  %s:\n", pkg)
		fmt.Fprintf(&b, "    1. append this line to go/.apicover-enforce:  %s\n", pkg)
		recursive := strings.Contains(pkg, "...")
		if recursive {
			b.WriteString("    2. add an apicover_named_test.go in EACH package the pattern covers, naming every exported symbol in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)\n")
			continue
		}
		dir := "go/" + strings.TrimPrefix(pkg, "./")
		fmt.Fprintf(&b, "    2. create %s/apicover_named_test.go naming every exported symbol of the package in a real assertion that executes it (an enrolled-but-unnamed package fails the gate too)\n", dir)
	}
	return strings.TrimRight(b.String(), "\n")
}
