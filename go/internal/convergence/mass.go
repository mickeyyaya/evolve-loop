package convergence

func FindingMass(findings []Finding) float64 { return defaultMass(findings) }

func MassAt(in Input, round int, bar Severity) float64 {
	return view{in: in, rules: rulesByLoop[in.Loop]}.mass(round, bar)
}
