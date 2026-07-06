package gravity

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Fetcher downloads adlist sources over HTTP(S).
type Fetcher struct {
	Client *http.Client
}

func NewFetcher() *Fetcher {
	return &Fetcher{Client: &http.Client{Timeout: 30 * time.Second}}
}

// Fetch downloads the body at url. etag, if non-empty, is sent as
// If-None-Match; a 304 response yields notModified=true and an empty body.
func (f *Fetcher) Fetch(ctx context.Context, url, etag string) (body string, newEtag string, notModified bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", false, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "yoshi-pihole/1.0 (+local ad-blocker)")
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := f.Client.Do(req)
	if err != nil {
		return "", "", false, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return "", etag, true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", false, fmt.Errorf("fetching %s: unexpected status %s", url, resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024*1024))
	if err != nil {
		return "", "", false, fmt.Errorf("reading body of %s: %w", url, err)
	}

	return string(data), resp.Header.Get("ETag"), false, nil
}

// DetectAndParse guesses the blocklist format from a sample of its
// non-comment lines and parses it accordingly. Formats supported: hosts-file
// (StevenBlack et al.), plain one-domain-per-line, and Adblock-Plus (||domain^).
func DetectAndParse(body string) ParseResult {
	const sampleSize = 50
	var hostsVotes, abpVotes, plainVotes int

	lines := strings.Split(body, "\n")
	sampled := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "||"):
			abpVotes++
		case strings.HasPrefix(line, "0.0.0.0 ") || strings.HasPrefix(line, "127.0.0.1 ") ||
			strings.HasPrefix(line, "::1 ") || strings.HasPrefix(line, ":: "):
			hostsVotes++
		default:
			plainVotes++
		}
		sampled++
		if sampled >= sampleSize {
			break
		}
	}

	switch {
	case abpVotes >= hostsVotes && abpVotes >= plainVotes && abpVotes > 0:
		return ParseABP(body)
	case hostsVotes >= plainVotes && hostsVotes > 0:
		return ParseHosts(body)
	default:
		return ParsePlain(body)
	}
}
