// Package gravity fetches and parses blocklist subscriptions (hosts-file,
// plain-domain, and Adblock-Plus syntax) into flat domain lists, mirroring
// Pi-hole's gravity.sh.
package gravity

import (
	"bufio"
	"strings"
)

// ParseResult holds the domains extracted from one adlist source plus a count
// of lines that looked like rules but failed to yield a usable domain.
type ParseResult struct {
	Domains        []string
	InvalidDomains int
}

// ParseHosts parses classic /etc/hosts-format blocklists, e.g.:
//
//	0.0.0.0 ads.example.com
//	127.0.0.1 tracker.example.com # comment
//
// This is the format used by StevenBlack's unified hosts list.
func ParseHosts(body string) ParseResult {
	var res ParseResult
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := stripComment(sc.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		ip := fields[0]
		if !isSinkholeIP(ip) {
			// A hosts-format line not pointing at a null route (0.0.0.0/127.0.0.1/::)
			// isn't a blocking rule (e.g. real /etc/hosts entries) — skip silently.
			continue
		}

		for _, domain := range fields[1:] {
			d, ok := normalizeDomain(domain)
			if !ok {
				res.InvalidDomains++
				continue
			}
			if d == "localhost" || d == "localhost.localdomain" || d == "broadcasthost" {
				continue
			}
			res.Domains = append(res.Domains, d)
		}
	}
	return res
}

func isSinkholeIP(ip string) bool {
	switch ip {
	case "0.0.0.0", "127.0.0.1", "::", "::1":
		return true
	default:
		return false
	}
}

// ParsePlain parses a plain one-domain-per-line blocklist (no IP prefix),
// as used by many Firebog/HaGeZi/OISD lists.
func ParsePlain(body string) ParseResult {
	var res ParseResult
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := stripComment(sc.Text())
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		d, ok := normalizeDomain(line)
		if !ok {
			res.InvalidDomains++
			continue
		}
		res.Domains = append(res.Domains, d)
	}
	return res
}

// ParseABP parses Adblock-Plus style rules, extracting the domain-blocking
// subset: "||domain.tld^" (optionally with "^$important" etc. trailing
// options, which are ignored). Non domain-anchor rules (cosmetic filters,
// path-based rules, exceptions starting with "@@") are skipped, not counted
// as invalid, since they're simply out of scope for a DNS-level blocker.
func ParseABP(body string) ParseResult {
	var res ParseResult
	sc := bufio.NewScanner(strings.NewReader(body))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "!") || strings.HasPrefix(line, "[") {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			continue // exception rule, not a DNS-level block
		}
		if !strings.HasPrefix(line, "||") {
			continue // cosmetic/path rule, out of scope for DNS blocking
		}

		rest := strings.TrimPrefix(line, "||")
		end := strings.IndexAny(rest, "^/*$")
		if end == -1 {
			end = len(rest)
		}
		candidate := rest[:end]

		d, ok := normalizeDomain(candidate)
		if !ok {
			res.InvalidDomains++
			continue
		}
		res.Domains = append(res.Domains, d)
	}
	return res
}

func stripComment(line string) string {
	if i := strings.IndexByte(line, '#'); i >= 0 {
		return line[:i]
	}
	return line
}

// normalizeDomain lowercases and validates a candidate domain string,
// rejecting empty/malformed entries.
func normalizeDomain(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return "", false
	}
	if strings.ContainsAny(s, " \t/\\*?\"'<>|") {
		return "", false
	}
	if !strings.Contains(s, ".") {
		return "", false
	}
	return s, true
}
