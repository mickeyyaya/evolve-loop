package core

// AdvisorDepthExceeded is the injectable recursion-depth guard for PhaseAdvisor
// (ADR-0052 §4.3, defense-in-depth). The PRIMARY guard is the mint denylist in
// mintConfigsFrom; this injectable seam is the secondary backstop.
//
// EVOLVE_ADVISOR_DEPTH was removed in cycle-10 flag-reduction. The guard now
// always returns false — the env-map signal is retired; the mint denylist
// (reservedAdvisorNames) is the sole active recursion gate.
func AdvisorDepthExceeded(_ map[string]string) bool {
	return false
}
