package badignore

// Broken carries a malformed ignore directive (no reason=), which Enumerate
// must reject.
//
//apicover:ignore
func Broken() {}
