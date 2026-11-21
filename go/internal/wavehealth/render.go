package wavehealth

import (
	"fmt"
	"strings"
)

func (r Report) Text() string {
	var b strings.Builder
	for _, st := range r.Steps {
		fmt.Fprintf(&b, "%-8s %-9s reading: %s | threshold: %s\n", st.ID, st.Result, st.Reading, st.Threshold)
		if st.Action != "" && st.Result != ResultOK {
			fmt.Fprintf(&b, "    action: %s\n", st.Action)
		}
	}
	fmt.Fprintf(&b, "outcome: %s\n", r.Outcome)
	return b.String()
}
