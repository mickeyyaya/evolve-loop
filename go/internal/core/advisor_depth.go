package core

// AdvisorDepthExceeded is PhaseAdvisor's recursion-depth backstop; it reports false, as the mint denylist is the active guard.
func AdvisorDepthExceeded(_ map[string]string) bool {
	return false
}
