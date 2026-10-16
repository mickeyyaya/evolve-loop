package phaseintegrity

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
)

func RepinIfDrifted(statePath, binPath, runningCommit, pluginVer string, prov ProvenanceVerified) (RepinResult, error) {
	pin, err := readShipPin(statePath)
	if err != nil || pin == "" {
		return RepinResult{}, nil
	}
	data, err := os.ReadFile(binPath)
	if err != nil {
		return RepinResult{}, nil
	}
	sum := sha256.Sum256(data)
	actual := hex.EncodeToString(sum[:])
	if actual == pin {
		return RepinResult{}, nil
	}
	return RepinShipSHA(statePath, actual, runningCommit, pluginVer, prov, false)
}

func readShipPin(statePath string) (string, error) {
	raw, err := os.ReadFile(statePath)
	if err != nil {
		return "", err
	}
	var st map[string]any
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", err
	}
	pin, _ := st["expected_ship_sha"].(string)
	return pin, nil
}
