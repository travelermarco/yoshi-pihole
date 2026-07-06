// Package dns implements the Yoshi Pi-hole DNS sinkhole: a miekg/dns based
// server that checks every query against the in-memory matcher engine and
// either answers locally (blocked) or forwards it upstream.
package dns

import (
	"context"
	"log"
	"time"

	"github.com/miekg/dns"

	yoshidb "yoshi-pihole/internal/db"
	"yoshi-pihole/internal/matcher"
	"yoshi-pihole/internal/service"
)

type Server struct {
	Engine          *matcher.Engine
	QueryStore      *yoshidb.QueryStore
	Bedtime         *service.Bedtime
	Upstreams       []string
	UpstreamTimeout time.Duration
	BlockMode       string // "nxdomain" or "null"

	udp *dns.Server
	tcp *dns.Server
}

func NewServer(listen string, engine *matcher.Engine, qs *yoshidb.QueryStore, bedtime *service.Bedtime, upstreams []string, upstreamTimeout time.Duration, blockMode string) *Server {
	s := &Server{
		Engine:          engine,
		QueryStore:      qs,
		Bedtime:         bedtime,
		Upstreams:       upstreams,
		UpstreamTimeout: upstreamTimeout,
		BlockMode:       blockMode,
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.handleQuery)

	s.udp = &dns.Server{Addr: listen, Net: "udp", Handler: mux}
	s.tcp = &dns.Server{Addr: listen, Net: "tcp", Handler: mux}
	return s
}

// ListenAndServe starts both the UDP and TCP listeners and blocks until
// either one fails or the server is shut down.
func (s *Server) ListenAndServe() error {
	errCh := make(chan error, 2)
	go func() { errCh <- s.udp.ListenAndServe() }()
	go func() { errCh <- s.tcp.ListenAndServe() }()

	err := <-errCh
	if err != nil {
		log.Printf("dns: server error: %v", err)
	}
	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	udpErr := s.udp.ShutdownContext(ctx)
	tcpErr := s.tcp.ShutdownContext(ctx)
	if udpErr != nil {
		return udpErr
	}
	return tcpErr
}
