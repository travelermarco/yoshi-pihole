// Command yoshi-pihole is a local, single-Mac ad/tracker blocker modeled on
// Pi-hole: a DNS sinkhole plus a REST API and embedded web dashboard, all in
// one binary.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"yoshi-pihole/internal/api"
	"yoshi-pihole/internal/config"
	"yoshi-pihole/internal/db"
	yoshidns "yoshi-pihole/internal/dns"
	"yoshi-pihole/internal/gravity"
	"yoshi-pihole/internal/matcher"
	"yoshi-pihole/internal/service"
	"yoshi-pihole/web"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		runServe(nil)
		return
	}

	switch os.Args[1] {
	case "serve":
		runServe(os.Args[2:])
	case "version":
		fmt.Printf("yoshi-pihole %s\n", version)
	case "install":
		fmt.Println("Installazione non automatizzata da questo binario.")
		fmt.Println("Esegui manualmente, con i permessi necessari:")
		fmt.Println("  sudo scripts/install.sh")
	case "uninstall":
		fmt.Println("Disinstallazione non automatizzata da questo binario.")
		fmt.Println("Esegui manualmente:")
		fmt.Println("  sudo scripts/uninstall.sh")
	case "status":
		runStatus()
	case "-h", "--help", "help":
		printUsage()
	default:
		// Unknown first arg: assume it's a flag for the default "serve" command.
		runServe(os.Args[1:])
	}
}

func printUsage() {
	fmt.Println(`yoshi-pihole - blocco pubblicità locale per questo Mac

Comandi:
  serve [flags]   Avvia il motore DNS e la dashboard (predefinito)
  status          Controlla se il servizio locale risponde
  install         Istruzioni per installare come LaunchDaemon
  uninstall       Istruzioni per disinstallare
  version         Stampa la versione`)
}

func runServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "config/config.yaml", "percorso del file di configurazione")
	dnsAddr := fs.String("dns-addr", "", "sovrascrive dns.listen dalla configurazione")
	webAddr := fs.String("web-addr", "", "sovrascrive web.listen dalla configurazione")
	_ = fs.Parse(args)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("configurazione non valida: %v", err)
	}
	if *dnsAddr != "" {
		cfg.DNS.Listen = *dnsAddr
	}
	if *webAddr != "" {
		cfg.Web.Listen = *webAddr
	}

	gravityDB, queriesDB, err := db.Open(cfg.Storage.DataDir)
	if err != nil {
		log.Fatalf("apertura database: %v", err)
	}
	defer gravityDB.Close()
	defer queriesDB.Close()

	gravityStore := db.NewGravityStore(gravityDB)
	queryStore := db.NewQueryStore(queriesDB)
	defer queryStore.Close()

	if err := gravity.EnsureDefaultAdlists(gravityStore, cfg.Gravity.DefaultAdlists); err != nil {
		log.Fatalf("inizializzazione blocklist predefinite: %v", err)
	}

	engine := matcher.New()
	reload := func() {
		snap, err := gravityStore.LoadSnapshot()
		if err != nil {
			log.Printf("ricarica motore di blocco fallita: %v", err)
			return
		}
		engine.Load(snap)
	}

	builder := gravity.NewBuilder(gravityStore)
	log.Println("scaricamento blocklist iniziali...")
	initCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := builder.Run(initCtx); err != nil {
		log.Printf("aggiornamento gravity iniziale fallito: %v", err)
	}
	cancel()
	reload()

	bedtime := service.NewBedtime()

	apiServer := &api.Server{
		GravityStore: gravityStore,
		QueryStore:   queryStore,
		Engine:       engine,
		Bedtime:      bedtime,
		Builder:      builder,
		Reload:       reload,
	}
	httpHandler := api.NewRouter(apiServer, web.FS())
	httpServer := &http.Server{Addr: cfg.Web.Listen, Handler: httpHandler}

	dnsServer := yoshidns.NewServer(
		cfg.DNS.Listen,
		engine,
		queryStore,
		bedtime,
		cfg.DNS.Upstreams,
		time.Duration(cfg.DNS.UpstreamTimeoutMS)*time.Millisecond,
		cfg.Blocking.Mode,
	)

	errCh := make(chan error, 2)
	go func() { errCh <- dnsServer.ListenAndServe() }()
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	log.Printf("Yoshi Pi-hole in ascolto: DNS su %s, dashboard su http://%s", cfg.DNS.Listen, cfg.Web.Listen)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		log.Printf("errore del server: %v", err)
	case <-sigCh:
		log.Println("arresto in corso...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = dnsServer.Shutdown(shutdownCtx)
	_ = httpServer.Shutdown(shutdownCtx)
}

func runStatus() {
	client := http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/api/health")
	if err != nil {
		fmt.Println("Yoshi Pi-hole non risponde:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		fmt.Println("Yoshi Pi-hole è attivo.")
		return
	}
	fmt.Printf("Yoshi Pi-hole ha risposto con stato %d\n", resp.StatusCode)
	os.Exit(1)
}
