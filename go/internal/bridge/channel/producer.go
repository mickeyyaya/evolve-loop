package channel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
)

// ProducerConfig wires a Producer to a single phase's raw logs.
type ProducerConfig struct {
	Workspace string
	Agent     string
	Phase     string
	Cycle     int
	CLI       string
	PollEvery time.Duration
	Now       func() time.Time
}

// Producer is the SOLE writer of the per-agent feed file. It opens the feed
// O_APPEND, builds one Normalizer over the phase's raw logs, and polls until
// ctx is cancelled. Callers MUST NOT write to the same feed file — one
// Producer per agent enforces the single-writer invariant.
type Producer struct{ cfg ProducerConfig }

// NewProducer constructs a Producer. PollEvery defaults to 2 s; Now defaults
// to time.Now.
func NewProducer(cfg ProducerConfig) *Producer {
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 2 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Producer{cfg: cfg}
}

// Run is the SOLE writer of the feed file. It opens the feed O_APPEND, builds
// one Normalizer over the phase's raw logs, and Polls until ctx cancels.
// A final Poll is performed after cancellation to drain any trailing output.
func (p *Producer) Run(ctx context.Context) error {
	feed, err := os.OpenFile(FeedPath(p.cfg.Workspace, p.cfg.Agent), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("channel: open feed: %w", err)
	}
	defer func() { _ = feed.Close() }()

	n := phasestream.NewNormalizer(phasestream.NormalizerConfig{
		Source: phasestream.Source{
			Producer: "normalizer",
			CLI:      p.cfg.CLI,
			Cycle:    p.cfg.Cycle,
			Phase:    p.cfg.Phase,
			Agent:    p.cfg.Agent,
		},
		TraceID:    fmt.Sprintf("cycle-%d-%s-channel", p.cfg.Cycle, p.cfg.Phase),
		StdoutPath: filepath.Join(p.cfg.Workspace, p.cfg.Phase+"-stdout.log"),
		StderrPath: filepath.Join(p.cfg.Workspace, p.cfg.Phase+"-stderr.log"),
		Sink:       feed,
		Now:        p.cfg.Now,
	})

	t := time.NewTicker(p.cfg.PollEvery)
	defer t.Stop()
	for {
		if _, err := n.Poll(); err != nil {
			fmt.Fprintf(os.Stderr, "[channel] WARN poll: %v\n", err)
		}
		select {
		case <-ctx.Done():
			_, _ = n.Poll() // final drain so a trailing answer isn't lost
			return nil
		case <-t.C:
		}
	}
}
