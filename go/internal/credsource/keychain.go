package credsource

import (
	"context"
	"fmt"
)

type keychain struct {
	run Runner
}

func (keychain) Name() string { return "keychain" }

func (k keychain) Lookup(ctx context.Context, host string) (Answer, error) {
	out, err := k.run(ctx, "security", "find-generic-password", "-s", Service, "-a", host, "-w")
	if err != nil {
		return Answer{}, fmt.Errorf("keychain: %w", err)
	}
	return secretAnswer(k.Name(), out)
}
