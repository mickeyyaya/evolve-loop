package bridge

import "fmt"

func HasToolUse(cli string) bool {
	m, err := LoadManifest(cli)
	return err == nil && m.HasCapability(CapabilityTools)
}

func LacksToolUse(cli string) bool {
	m, err := LoadManifest(cli)
	return err == nil && !m.HasCapability(CapabilityTools)
}

func InstructionFile(cli string) (string, error) {
	m, err := LoadManifest(cli)
	if err != nil {
		return "", err
	}
	return requireInstructionFile(cli, m.InstructionFile)
}

func requireInstructionFile(cli, file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("bridge:manifest: cli=%s names no instruction_file; set %q or the file its CLI reads", cli, InstructionInject)
	}
	return file, nil
}
