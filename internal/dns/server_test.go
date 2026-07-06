package dns

import (
	"context"
	"testing"
	"time"

	"github.com/miekg/dns"

	yoshidb "yoshi-pihole/internal/db"
	"yoshi-pihole/internal/matcher"
	"yoshi-pihole/internal/service"
)

// startFakeUpstream runs a minimal DNS server that answers every A query
// with a fixed IP, so forwarding tests don't depend on real network access.
func startFakeUpstream(t *testing.T, addr string) *dns.Server {
	t.Helper()
	mux := dns.NewServeMux()
	mux.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		m := new(dns.Msg)
		m.SetReply(r)
		if len(r.Question) > 0 && r.Question[0].Qtype == dns.TypeA {
			rr, _ := dns.NewRR(r.Question[0].Name + " 300 IN A 93.184.216.34")
			m.Answer = append(m.Answer, rr)
		}
		_ = w.WriteMsg(m)
	})
	srv := &dns.Server{Addr: addr, Net: "udp", Handler: mux}
	ready := make(chan struct{})
	srv.NotifyStartedFunc = func() { close(ready) }
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			t.Logf("fake upstream stopped: %v", err)
		}
	}()
	waitReady(t, ready)
	return srv
}

func waitReady(t *testing.T, ready <-chan struct{}) {
	t.Helper()
	select {
	case <-ready:
	case <-time.After(2 * time.Second):
		t.Fatal("server did not start in time")
	}
}

func TestServerBlocksAndForwards(t *testing.T) {
	upstreamAddr := "127.0.0.1:17154"
	upstream := startFakeUpstream(t, upstreamAddr)
	defer upstream.Shutdown()

	dataDir := t.TempDir()
	gravityDB, queriesDB, err := yoshidb.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer gravityDB.Close()
	defer queriesDB.Close()

	gravityStore := yoshidb.NewGravityStore(gravityDB)
	if _, err := gravityStore.AddDomain("blocked.example.com", yoshidb.TypeDenyExact, "test"); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}

	snap, err := gravityStore.LoadSnapshot()
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	engine := matcher.New()
	engine.Load(snap)

	queryStore := yoshidb.NewQueryStore(queriesDB)
	defer queryStore.Close()

	bedtime := service.NewBedtime()

	listenAddr := "127.0.0.1:17155"
	srv := NewServer(listenAddr, engine, queryStore, bedtime, []string{upstreamAddr}, time.Second, "null")
	go srv.ListenAndServe()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	waitForUDP(t, listenAddr)

	client := &dns.Client{Timeout: 2 * time.Second}

	// Blocked domain should get a null-routed 0.0.0.0 answer, not forwarded.
	blockedMsg := new(dns.Msg)
	blockedMsg.SetQuestion("blocked.example.com.", dns.TypeA)
	resp, _, err := client.Exchange(blockedMsg, listenAddr)
	if err != nil {
		t.Fatalf("querying blocked domain: %v", err)
	}
	if len(resp.Answer) != 1 {
		t.Fatalf("expected 1 answer for blocked domain, got %d", len(resp.Answer))
	}
	a, ok := resp.Answer[0].(*dns.A)
	if !ok || a.A.String() != "0.0.0.0" {
		t.Errorf("expected blocked domain to resolve to 0.0.0.0, got %v", resp.Answer[0])
	}

	// Allowed domain should be forwarded to the fake upstream.
	allowedMsg := new(dns.Msg)
	allowedMsg.SetQuestion("allowed.example.com.", dns.TypeA)
	resp2, _, err := client.Exchange(allowedMsg, listenAddr)
	if err != nil {
		t.Fatalf("querying allowed domain: %v", err)
	}
	if len(resp2.Answer) != 1 {
		t.Fatalf("expected 1 answer for forwarded domain, got %d", len(resp2.Answer))
	}
	a2, ok := resp2.Answer[0].(*dns.A)
	if !ok || a2.A.String() != "93.184.216.34" {
		t.Errorf("expected forwarded domain to resolve to 93.184.216.34, got %v", resp2.Answer[0])
	}
}

func waitForUDP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	msg := new(dns.Msg)
	msg.SetQuestion("ready-check.invalid.", dns.TypeA)
	client := &dns.Client{Timeout: 100 * time.Millisecond}
	for time.Now().Before(deadline) {
		if _, _, err := client.Exchange(msg, addr); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("server at %s did not become ready in time", addr)
}
