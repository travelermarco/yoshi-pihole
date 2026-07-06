package dns

import (
	"net"

	"github.com/miekg/dns"

	"yoshi-pihole/internal/gravity"
)

// applyBlockedResponse fills m (already SetReply'd against the original
// request) to represent a blocked answer, honoring either the engine-wide
// default blockMode ("nxdomain" or "null") or a per-rule regex ;reply=
// override. It reports whether a response should be sent at all — a
// ";reply=none" override means silently drop the query, matching Pi-hole.
func applyBlockedResponse(m *dns.Msg, q dns.Question, blockMode string, override *gravity.ReplyOverride) (send bool) {
	if override != nil {
		switch override.Kind {
		case "none":
			return false
		case "nxdomain":
			m.Rcode = dns.RcodeNameError
			return true
		case "refused":
			m.Rcode = dns.RcodeRefused
			return true
		case "nodata":
			m.Rcode = dns.RcodeSuccess
			return true
		case "ip":
			return applyIPOverride(m, q, override.IP)
		}
	}

	switch blockMode {
	case "nxdomain":
		m.Rcode = dns.RcodeNameError
		return true
	default: // "null": answer address queries with 0.0.0.0/::, NODATA otherwise
		return applyNullRoute(m, q)
	}
}

func applyNullRoute(m *dns.Msg, q dns.Question) bool {
	m.Rcode = dns.RcodeSuccess
	switch q.Qtype {
	case dns.TypeA:
		rr, err := dns.NewRR(q.Name + " 300 IN A 0.0.0.0")
		if err == nil {
			m.Answer = append(m.Answer, rr)
		}
	case dns.TypeAAAA:
		rr, err := dns.NewRR(q.Name + " 300 IN AAAA ::")
		if err == nil {
			m.Answer = append(m.Answer, rr)
		}
	}
	// Any other query type gets NOERROR with no answer (NODATA) — there's no
	// meaningful null route for e.g. MX/TXT records.
	return true
}

func applyIPOverride(m *dns.Msg, q dns.Question, ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return applyNullRoute(m, q)
	}
	m.Rcode = dns.RcodeSuccess

	isV4 := parsed.To4() != nil
	if q.Qtype == dns.TypeA && isV4 {
		if rr, err := dns.NewRR(q.Name + " 300 IN A " + parsed.String()); err == nil {
			m.Answer = append(m.Answer, rr)
		}
	} else if q.Qtype == dns.TypeAAAA && !isV4 {
		if rr, err := dns.NewRR(q.Name + " 300 IN AAAA " + parsed.String()); err == nil {
			m.Answer = append(m.Answer, rr)
		}
	}
	return true
}
