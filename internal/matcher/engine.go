// Package matcher holds the in-memory domain/regex matching engine that the
// DNS resolver consults on every query. Snapshots are rebuilt off the hot
// path (on gravity updates or domain-list edits) and swapped atomically, so
// query handling never blocks on a lock or a database read.
package matcher

import (
	"log"
	"strings"
	"sync/atomic"

	"yoshi-pihole/internal/db"
	"yoshi-pihole/internal/gravity"
)

// Decision is the outcome of evaluating one query against the engine.
type Decision struct {
	Blocked bool
	// Source describes why a blocked query was blocked: "gravity" (blocklist
	// subscription), "manual" (user blacklist), or "regex".
	Source  string
	RegexID int64
	Reply   *gravity.ReplyOverride
}

type snapshot struct {
	exactAllow map[string]struct{}
	exactDeny  map[string]string
	regexAllow []*gravity.RegexRule
	regexDeny  []*gravity.RegexRule
}

// Engine is safe for concurrent use: Load may run concurrently with any
// number of Evaluate calls.
type Engine struct {
	ptr atomic.Pointer[snapshot]
}

func New() *Engine {
	e := &Engine{}
	e.ptr.Store(&snapshot{
		exactAllow: map[string]struct{}{},
		exactDeny:  map[string]string{},
	})
	return e
}

// Load compiles a fresh db.Snapshot into the engine and atomically swaps it
// in. Regex rules that fail to compile are logged and skipped rather than
// aborting the whole load.
func (e *Engine) Load(data *db.Snapshot) {
	snap := &snapshot{
		exactAllow: data.ExactAllow,
		exactDeny:  data.ExactDeny,
	}
	if snap.exactAllow == nil {
		snap.exactAllow = map[string]struct{}{}
	}
	if snap.exactDeny == nil {
		snap.exactDeny = map[string]string{}
	}

	for _, raw := range data.RegexAllow {
		rule, err := gravity.ParseRegex(raw.Raw)
		if err != nil {
			log.Printf("matcher: skipping invalid allow-regex %q: %v", raw.Raw, err)
			continue
		}
		rule.ID = raw.ID
		snap.regexAllow = append(snap.regexAllow, rule)
	}
	for _, raw := range data.RegexDeny {
		rule, err := gravity.ParseRegex(raw.Raw)
		if err != nil {
			log.Printf("matcher: skipping invalid deny-regex %q: %v", raw.Raw, err)
			continue
		}
		rule.ID = raw.ID
		snap.regexDeny = append(snap.regexDeny, rule)
	}

	e.ptr.Store(snap)
}

// Stats reports the current snapshot's rule counts, for the dashboard.
type Stats struct {
	ExactAllow int
	ExactDeny  int
	RegexAllow int
	RegexDeny  int
}

func (e *Engine) Stats() Stats {
	s := e.ptr.Load()
	return Stats{
		ExactAllow: len(s.exactAllow),
		ExactDeny:  len(s.exactDeny),
		RegexAllow: len(s.regexAllow),
		RegexDeny:  len(s.regexDeny),
	}
}

// Evaluate applies Pi-hole's precedence order: allow-exact, allow-regex,
// deny-exact (gravity + manual blacklist merged), deny-regex, then "forward".
func (e *Engine) Evaluate(domain string, qtype uint16) Decision {
	domain = strings.ToLower(strings.TrimSuffix(domain, "."))
	snap := e.ptr.Load()

	if _, ok := snap.exactAllow[domain]; ok {
		return Decision{Blocked: false}
	}
	for _, r := range snap.regexAllow {
		if r.Matches(domain, qtype) {
			return Decision{Blocked: false}
		}
	}
	if source, ok := snap.exactDeny[domain]; ok {
		return Decision{Blocked: true, Source: source}
	}
	for _, r := range snap.regexDeny {
		if r.Matches(domain, qtype) {
			return Decision{Blocked: true, Source: "regex", RegexID: r.ID, Reply: r.Reply}
		}
	}

	return Decision{Blocked: false}
}
