package config

import "testing"

func TestIsRegistryFault_RegistryAndSpineCodesOnly(t *testing.T) {
	for _, code := range []string{codeRegistryUnreadable, codeRegistryMalformed, codeWeakSpine} {
		if !IsRegistryFault(Warning{Code: code}) {
			t.Errorf("%s is a registry fault", code)
		}
	}
	for _, code := range []string{codeUnknownValue, "", "phase-disabled"} {
		if IsRegistryFault(Warning{Code: code}) {
			t.Errorf("%s is not a registry fault", code)
		}
	}
}
