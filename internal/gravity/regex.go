package gravity

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/miekg/dns"
)

// inlineCommentRE strips Pi-hole/PCRE-style "(?#free text)" inline comments,
// which Go's RE2 engine (unlike PCRE) cannot parse directly.
var inlineCommentRE = regexp.MustCompile(`\(\?#[^)]*\)`)

// ReplyOverride customizes the DNS response for domains matching a regex
// rule, overriding the engine's default blocking mode.
type ReplyOverride struct {
	Kind string // "nodata", "nxdomain", "refused", "none", or "ip"
	IP   string // set when Kind == "ip"
}

// RegexRule is a compiled Pi-hole-style regex filter, including its
// non-standard trailing ";key=value" extensions.
type RegexRule struct {
	ID               int64
	Raw              string
	Pattern          *regexp.Regexp
	Invert           bool
	QueryTypes       map[uint16]bool // nil = matches any query type
	NegateQueryTypes bool
	Reply            *ReplyOverride
}

// Matches reports whether domain (already lowercased, no trailing dot) and
// qtype satisfy this rule, honoring ;querytype= scoping and ;invert.
func (r *RegexRule) Matches(domain string, qtype uint16) bool {
	if r.QueryTypes != nil {
		_, present := r.QueryTypes[qtype]
		typeOK := present
		if r.NegateQueryTypes {
			typeOK = !present
		}
		if !typeOK {
			return false
		}
	}

	matched := r.Pattern.MatchString(domain)
	if r.Invert {
		matched = !matched
	}
	return matched
}

// ParseRegex parses a raw Pi-hole style regex filter string: a POSIX-ERE-ish
// pattern (Go RE2 in practice) optionally followed by ";key=value" tags
// (;querytype=A,AAAA / ;querytype=!A / ;invert / ;reply=nxdomain|nodata|
// refused|none|ip|<literal IP>) and inline "(?#comment)" segments.
func ParseRegex(raw string) (*RegexRule, error) {
	parts := strings.Split(raw, ";")
	patternPart := inlineCommentRE.ReplaceAllString(strings.TrimSpace(parts[0]), "")

	compiled, err := regexp.Compile(patternPart)
	if err != nil {
		return nil, fmt.Errorf("compiling regex %q: %w", patternPart, err)
	}

	rule := &RegexRule{Raw: raw, Pattern: compiled}

	for _, tag := range parts[1:] {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		key, value, _ := strings.Cut(tag, "=")
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "invert":
			rule.Invert = true
		case "querytype":
			negate := strings.HasPrefix(value, "!")
			value = strings.TrimPrefix(value, "!")
			types := map[uint16]bool{}
			for _, name := range strings.Split(value, ",") {
				name = strings.ToUpper(strings.TrimSpace(name))
				t, ok := dns.StringToType[name]
				if !ok {
					return nil, fmt.Errorf("unknown query type %q in %q", name, raw)
				}
				types[t] = true
			}
			rule.QueryTypes = types
			rule.NegateQueryTypes = negate
		case "reply":
			value = strings.ToLower(value)
			switch value {
			case "nodata", "nxdomain", "refused", "none":
				rule.Reply = &ReplyOverride{Kind: value}
			case "":
				return nil, fmt.Errorf(";reply= requires a value in %q", raw)
			default:
				// Anything else is expected to be a literal IPv4/IPv6 address.
				rule.Reply = &ReplyOverride{Kind: "ip", IP: value}
			}
		default:
			return nil, fmt.Errorf("unknown regex extension %q in %q", key, raw)
		}
	}

	return rule, nil
}
