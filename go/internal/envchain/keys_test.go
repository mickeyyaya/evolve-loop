package envchain

import "testing"

func TestContractCorrectionRetriesConstants(t *testing.T) {
	if KeyContractCorrectionRetries != "EVOLVE_CONTRACT_CORRECTION_RETRIES" {
		t.Errorf("key = %q", KeyContractCorrectionRetries)
	}
	if DefContractCorrectionRetries != 2 {
		t.Errorf("default = %d, want 2", DefContractCorrectionRetries)
	}
	if MaxContractCorrectionRetries != 5 {
		t.Errorf("max = %d, want 5", MaxContractCorrectionRetries)
	}
}
