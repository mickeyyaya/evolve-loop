package ship

import (
	"context"
	"fmt"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/credsource"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	credentialProbeTimeout = 5 * time.Second
	credentialReportTail   = "; the push and gh keep today's credential path"
)

func CredentialReport(ctx context.Context, projectRoot string, open func() (credsource.Source, error)) string {
	stage, err := policy.CredentialSourceStageFor(projectRoot)
	switch {
	case err != nil:
		return fmt.Sprintf("[credential] the stage did not load: %v%s", err, credentialReportTail)
	case stage != policy.CredentialSourceStageShadow:
		return ""
	}
	host := credsource.DefaultHost
	src, err := open()
	if err != nil {
		return fmt.Sprintf("[credential] %s: no source for %s: %v%s", stage, host, err, credentialReportTail)
	}
	ctx, cancel := context.WithTimeout(ctx, credentialProbeTimeout)
	defer cancel()
	answer, err := src.Lookup(ctx, host)
	if err != nil {
		return fmt.Sprintf("[credential] %s: source %s did not answer for %s: %v%s", stage, src.Name(), host, err, credentialReportTail)
	}
	return fmt.Sprintf("[credential] %s: source %s answered for %s%s", stage, answer.Source, host, credentialReportTail)
}

func (o *Options) credentialProbe(ctx context.Context) string {
	open := o.credentialSource
	if open == nil {
		open = credsource.Default
	}
	return CredentialReport(ctx, o.ProjectRoot, open)
}
