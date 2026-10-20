package policy

import (
	"fmt"
	"io"
	"os"
)

var legacyLabelWarnings io.Writer = os.Stderr

var legacyAgentLabels = map[string]string{"retro": "retrospective"}

func legacyLabelValue[V any](values map[string]V, block, label string, usable func(V) bool) (V, bool) {
	var zero V
	legacy, ok := legacyAgentLabels[label]
	if !ok {
		return zero, false
	}
	v, ok := values[legacy]
	if !ok || !usable(v) {
		return zero, false
	}
	fmt.Fprintf(legacyLabelWarnings, "[policy] WARN %s.%s applies to the %q launch label; rename the key to %s.%s\n", block, legacy, label, block, label)
	return v, true
}
