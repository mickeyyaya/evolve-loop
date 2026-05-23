// runner.go — clock helper for the native ship path. CmdRunner +
// execRunner are defined in ship.go (the existing dispatcher).
package ship

import "time"

func defaultNow() Now {
	t := time.Now().UTC()
	return Now{
		Unix:    t.Unix(),
		RFC3339: t.Format(time.RFC3339),
	}
}
