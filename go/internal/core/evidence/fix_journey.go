package evidence

import (
	"errors"
	"fmt"
	"strings"
)

const KeyFixJourney = "Fix-Journey"

var ErrFixJourneyInvalid = errors.New("invalid Fix-Journey trailer")

type FixJourney struct {
	Present bool
	ID      string
	Reason  string
}

func (f FixJourney) None() bool { return f.Present && f.ID == "" }

func ParseFixJourney(msg string) (FixJourney, error) {
	lines := strings.Split(msg, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if key, val, _ := cutTrailer(lines[i]); key == KeyFixJourney {
			return fixJourneyOf(val)
		}
	}
	return FixJourney{}, nil
}

func fixJourneyOf(val string) (FixJourney, error) {
	word, reason, _ := strings.Cut(val, " ")
	reason = strings.TrimSpace(reason)
	switch {
	case word == "none" && reason != "":
		return FixJourney{Present: true, Reason: reason}, nil
	case word != "none" && reason == "" && word != "":
		return FixJourney{Present: true, ID: word}, nil
	}
	return FixJourney{}, fmt.Errorf("%w: %q is not <journey id> or none <reason>", ErrFixJourneyInvalid, val)
}
