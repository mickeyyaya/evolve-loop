package practicewatch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
)

const (
	bodyLimit      = 8 << 20
	stderrDetail   = 200
	fetchUserAgent = "evolve-practice-watch"
)

type fetched struct {
	text        string
	etag        string
	notModified bool
	truncated   bool
	skip        string
}

func (o Options) fetch(ctx context.Context, src Source, etag string) (fetched, error) {
	if src.Kind == KindCommand {
		return o.fetchCommand(ctx, src)
	}
	return o.fetchHTTP(ctx, src, etag)
}

func (o Options) fetchHTTP(ctx context.Context, src Source, etag string) (fetched, error) {
	u, err := httpLocator(src)
	if err != nil {
		return fetched{}, err
	}
	req := (&http.Request{Method: http.MethodGet, URL: u, Host: u.Host, Header: http.Header{"User-Agent": {fetchUserAgent}}}).WithContext(ctx)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	client := *o.Client
	client.CheckRedirect = guardRedirect
	resp, err := client.Do(req)
	if err != nil {
		return fetched{}, fmt.Errorf("fetch %s: %w", u.Redacted(), errors.Unwrap(err))
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode == http.StatusNotModified:
		return fetched{notModified: true, etag: etag}, nil
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return fetched{}, fmt.Errorf("fetch %s: HTTP %d", u.Redacted(), resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, bodyLimit+1))
	if err != nil {
		return fetched{}, fmt.Errorf("read %s: %w", u.Redacted(), err)
	}
	truncated := len(body) > bodyLimit
	if truncated {
		body = body[:bodyLimit]
	}
	return fetched{text: string(body), etag: resp.Header.Get("ETag"), truncated: truncated}, nil
}

func (o Options) fetchCommand(ctx context.Context, src Source) (fetched, error) {
	argv := strings.Fields(src.Locator)
	var out, errOut strings.Builder
	capped := &cappedWriter{w: &out, left: bodyLimit}
	code, err := o.Run(ctx, argv[0], "", argv[1:], nil, nil, capped, &errOut)
	stdout, stderr := out.String(), errOut.String()
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return fetched{skip: argv[0] + " is not installed"}, nil
	case err != nil:
		return fetched{}, fmt.Errorf("run %q: %w", src.Locator, err)
	case code != 0:
		return fetched{}, fmt.Errorf("run %q: exit %d: %s", src.Locator, code, capText(strings.TrimSpace(stderr), stderrDetail))
	}
	return fetched{text: stdout, truncated: capped.cut}, nil
}

const maxRedirects = 5

func guardRedirect(req *http.Request, via []*http.Request) error {
	switch {
	case len(via) > maxRedirects:
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	case req.URL.Scheme != "http" && req.URL.Scheme != "https":
		return fmt.Errorf("redirect to %q refused: only http or https", req.URL.Scheme)
	case via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme == "http":
		return fmt.Errorf("redirect from https to http refused: downgrade")
	}
	return nil
}

type cappedWriter struct {
	w    io.Writer
	left int
	cut  bool
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	keep := min(len(p), c.left)
	c.left -= keep
	c.cut = c.cut || keep < len(p)
	if _, err := c.w.Write(p[:keep]); err != nil {
		return 0, err
	}
	return len(p), nil
}
