package dns

import (
	"errors"
	"time"

	"github.com/miekg/dns"
)

// forwardUpstream tries each upstream server in order, retrying over TCP if
// a UDP response comes back truncated. It returns the first successful
// answer along with which upstream produced it.
func forwardUpstream(upstreams []string, timeout time.Duration, req *dns.Msg) (*dns.Msg, string, error) {
	if len(upstreams) == 0 {
		return nil, "", errors.New("no upstream DNS servers configured")
	}

	udpClient := &dns.Client{Timeout: timeout}
	tcpClient := &dns.Client{Net: "tcp", Timeout: timeout}

	var lastErr error
	for _, up := range upstreams {
		resp, _, err := udpClient.Exchange(req, up)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.Truncated {
			if tcpResp, _, err2 := tcpClient.Exchange(req, up); err2 == nil {
				return tcpResp, up, nil
			}
			// Fall through and use the (truncated) UDP response rather than
			// failing outright — a partial answer beats none.
		}
		return resp, up, nil
	}

	return nil, "", lastErr
}
