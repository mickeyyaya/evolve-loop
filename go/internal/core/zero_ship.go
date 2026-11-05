package core

import "fmt"

const zeroShipRule = "zero-ship"

func EvaluateZeroShip(waveShips []int, ceiling int) BlockerVerdict {
	if ceiling <= 0 {
		return BlockerVerdict{}
	}
	run := 0
	for i := len(waveShips) - 1; i >= 0 && waveShips[i] == 0; i-- {
		run++
	}
	if run < ceiling {
		return BlockerVerdict{}
	}
	return BlockerVerdict{
		Halt: true, Rule: zeroShipRule, Count: run,
		Reason: fmt.Sprintf("%d consecutive waves landed no ship (ceiling %d)", run, ceiling),
	}
}
