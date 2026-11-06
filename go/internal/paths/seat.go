package paths

import "path/filepath"

const (
	seatRootName     = ".evolve-seat"
	seatHolderName   = "holder"
	seatTokenName    = "token"
	SeatSettingsName = "settings.json"
)

func SeatRoot(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(home, seatRootName)
}

func SeatHolderDir(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(SeatRoot(home), seatHolderName)
}

func SeatTokenPath(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(SeatHolderDir(home), seatTokenName)
}

func SeatSettingsPath(home string) string {
	if home == "" {
		return ""
	}
	return filepath.Join(SeatRoot(home), SeatSettingsName)
}
