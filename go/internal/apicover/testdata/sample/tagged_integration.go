//go:build integration

package sample

// IntegrationOnlyFunc lives behind the integration build tag and MUST be
// skipped by the default (untagged) apicover measurement.
func IntegrationOnlyFunc() {}
