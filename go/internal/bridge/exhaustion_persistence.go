package bridge

const exhaustionPersistObservations = 2

type exhaustionGate struct {
	threshold int
	streak    int
}

func newExhaustionGate() *exhaustionGate {
	return &exhaustionGate{threshold: exhaustionPersistObservations}
}

func (g *exhaustionGate) observe(matched bool) bool {
	if !matched {
		g.streak = 0
		return false
	}
	g.streak++
	return g.streak >= g.threshold
}
