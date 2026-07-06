package dns

import (
	"net"
	"time"

	"github.com/miekg/dns"

	yoshidb "yoshi-pihole/internal/db"
	"yoshi-pihole/internal/matcher"
)

func (s *Server) handleQuery(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = true

	if len(r.Question) == 0 {
		m.Rcode = dns.RcodeFormatError
		_ = w.WriteMsg(m)
		return
	}
	q := r.Question[0]
	client := clientIP(w.RemoteAddr())

	if !s.Bedtime.IsDisabled() {
		decision := s.Engine.Evaluate(q.Name, q.Qtype)
		if decision.Blocked {
			send := applyBlockedResponse(m, q, s.BlockMode, decision.Reply)
			if send {
				_ = w.WriteMsg(m)
			}
			s.logQuery(q, client, statusForDecision(decision), "", start, replyTypeFromMsg(m, send))
			return
		}
	}

	s.forwardAndReply(w, r, m, q, client, start)
}

func (s *Server) forwardAndReply(w dns.ResponseWriter, r *dns.Msg, m *dns.Msg, q dns.Question, client string, start time.Time) {
	resp, upstream, err := forwardUpstream(s.Upstreams, s.UpstreamTimeout, r)
	if err != nil {
		m.Rcode = dns.RcodeServerFailure
		_ = w.WriteMsg(m)
		s.logQuery(q, client, yoshidb.StatusBlockedUpstreamFail, "", start, 7)
		return
	}

	resp.Id = r.Id
	_ = w.WriteMsg(resp)
	s.logQuery(q, client, yoshidb.StatusForwarded, upstream, start, replyTypeFromMsg(resp, true))
}

func statusForDecision(d matcher.Decision) int {
	switch d.Source {
	case "gravity":
		return yoshidb.StatusBlockedGravity
	case "manual":
		return yoshidb.StatusBlockedExact
	case "regex":
		return yoshidb.StatusBlockedRegex
	default:
		return yoshidb.StatusBlockedGravity
	}
}

func (s *Server) logQuery(q dns.Question, client string, status int, forward string, start time.Time, replyType int) {
	if s.QueryStore == nil {
		return
	}
	var regexID *int64
	ev := yoshidb.QueryEvent{
		Timestamp:   start,
		QType:       q.Qtype,
		Domain:      trimDot(q.Name),
		Client:      client,
		Status:      status,
		ReplyType:   replyType,
		ReplyTimeMS: time.Since(start).Milliseconds(),
		Forward:     forward,
		RegexID:     regexID,
	}
	s.QueryStore.Log(ev)
}

// replyTypeFromMsg classifies a response for the query log: 0=unknown,
// 2=nxdomain, 3=cname, 4=ip, 7=servfail.
func replyTypeFromMsg(m *dns.Msg, sent bool) int {
	if !sent {
		return 0
	}
	switch m.Rcode {
	case dns.RcodeNameError:
		return 2
	case dns.RcodeServerFailure:
		return 7
	}
	for _, rr := range m.Answer {
		switch rr.Header().Rrtype {
		case dns.TypeCNAME:
			return 3
		case dns.TypeA, dns.TypeAAAA:
			return 4
		}
	}
	return 0
}

func clientIP(addr net.Addr) string {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return a.IP.String()
	case *net.TCPAddr:
		return a.IP.String()
	default:
		host, _, err := net.SplitHostPort(addr.String())
		if err == nil {
			return host
		}
		return addr.String()
	}
}

func trimDot(s string) string {
	if len(s) > 0 && s[len(s)-1] == '.' {
		return s[:len(s)-1]
	}
	return s
}
