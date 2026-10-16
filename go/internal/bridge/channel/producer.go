package channel

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasestream"
)

type ProducerConfig struct {
	Workspace  string
	Agent      string
	Phase      string
	Cycle      int
	CLI        string
	StdoutPath string
	StderrPath string
	PollEvery  time.Duration
	Now        func() time.Time
	Warn       io.Writer
}

const sourceMissThreshold = 20

type Producer struct{ cfg ProducerConfig }

func NewProducer(cfg ProducerConfig) *Producer {
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 2 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Producer{cfg: cfg}
}

var openFeed = os.OpenFile

func (p *Producer) Run(ctx context.Context) error {
	feed, err := openFeed(FeedPath(p.cfg.Workspace, p.cfg.Agent), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("channel: open feed: %w", err)
	}
	defer func() { _ = feed.Close() }()

	stdoutPath := p.cfg.StdoutPath
	if stdoutPath == "" {
		stdoutPath = filepath.Join(p.cfg.Workspace, p.cfg.Phase+"-stdout.log")
	}
	stderrPath := p.cfg.StderrPath
	if stderrPath == "" {
		stderrPath = filepath.Join(p.cfg.Workspace, p.cfg.Phase+"-stderr.log")
	}

	n := phasestream.NewNormalizer(phasestream.NormalizerConfig{
		Source: phasestream.Source{
			Producer: "normalizer",
			CLI:      p.cfg.CLI,
			Cycle:    p.cfg.Cycle,
			Phase:    p.cfg.Phase,
			Agent:    p.cfg.Agent,
		},
		TraceID:    fmt.Sprintf("cycle-%d-%s-channel", p.cfg.Cycle, p.cfg.Phase),
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
		Sink:       feed,
		Now:        p.cfg.Now,
	})

	warn := p.cfg.Warn
	if warn == nil {
		warn = os.Stderr
	}
	sourceSeen := false
	missedPolls := 0

	t := time.NewTicker(p.cfg.PollEvery)
	defer t.Stop()
	for {
		if _, err := n.Poll(); err != nil {
			fmt.Fprintf(warn, "[channel] WARN poll: %v\n", err)
		}
		if !sourceSeen {
			if _, err := os.Stat(stdoutPath); err == nil {
				sourceSeen = true
			} else {
				missedPolls++
				if missedPolls >= sourceMissThreshold {
					fmt.Fprintf(warn, "[channel] WARN content source never appeared after %d polls: %s — feed will be empty (agent/phase name mismatch or wrong CLI family?)\n", missedPolls, stdoutPath)
					sourceSeen = true
				}
			}
		}
		select {
		case <-ctx.Done():
			_, _ = n.Poll()
			return nil
		case <-t.C:
		}
	}
}
