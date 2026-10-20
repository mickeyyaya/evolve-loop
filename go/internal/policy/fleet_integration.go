package policy

import "fmt"

const (
	FleetIntegrationShadow  = "shadow"
	FleetIntegrationEnforce = "enforce"
)

type FleetIntegrationPolicy struct {
	Stage string `json:"stage,omitempty"`
}

func (p Policy) FleetIntegrationStage() (string, error) {
	if p.Fleet == nil || p.Fleet.Integration == nil || p.Fleet.Integration.Stage == "" {
		return FleetIntegrationShadow, nil
	}
	switch v := p.Fleet.Integration.Stage; v {
	case FleetIntegrationShadow, FleetIntegrationEnforce:
		return v, nil
	default:
		return "", fmt.Errorf("policy: fleet.integration.stage must be shadow or enforce, got %q", v)
	}
}
