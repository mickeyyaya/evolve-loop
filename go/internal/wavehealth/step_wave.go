package wavehealth

import (
	"fmt"
	"time"
)

const brakeAction = "run the BRAKE procedure of the SOP; never kill the loop"

func waveStep(s Snapshot) Step {
	c := s.Settings
	threshold := fmt.Sprintf("age %dm and %d live lanes: 0 ships is STALL; under %.2f ships/h is SLOW", c.StallMinAgeMinutes, c.StallMinLiveLanes, c.MinShipsPerHour)
	if !s.Wave.Known {
		return notGathered("A2", threshold)
	}
	w := s.Wave.Value
	age := max(s.Now.Sub(w.Start), 0).Truncate(time.Minute)
	rate := shipRate(w.Shipped, s.Now.Sub(w.Start))
	st := Step{ID: "A2", Threshold: threshold, Result: ResultOK}
	reading := fmt.Sprintf("age %s, ships %d of %d, live %s, %.2f ships/h", age, w.Shipped, w.Started, liveText(w.PoolLive), rate)
	minAge := time.Duration(c.StallMinAgeMinutes) * time.Minute
	switch {
	case age < minAge:
		st.Reading = fmt.Sprintf("%s; age under %s", reading, minAge)
	case !w.PoolLive.Known:
		st.Reading, st.Result, st.Action = reading+"; no pool signal", ResultGap, "read evolve wave status and the loop signals"
	case w.PoolLive.Value < c.StallMinLiveLanes:
		st.Reading = fmt.Sprintf("%s; live lanes %d under %d", reading, w.PoolLive.Value, c.StallMinLiveLanes)
	case w.Shipped == 0:
		st.Reading, st.Result, st.Action = reading+"; STALL", ResultBrake, brakeAction
	case rate < c.MinShipsPerHour:
		st.Reading, st.Result, st.Action = fmt.Sprintf("%s; SLOW, under %.2f", reading, c.MinShipsPerHour), ResultBrake, brakeAction
	default:
		st.Reading = reading
	}
	return st
}

func shipRate(ships int, age time.Duration) float64 {
	if age <= 0 {
		return 0
	}
	return float64(ships) / age.Hours()
}

func liveText(live Known[int]) string {
	if !live.Known {
		return "unknown"
	}
	return fmt.Sprint(live.Value)
}
