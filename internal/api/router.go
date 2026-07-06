// Package api implements Yoshi Pi-hole's REST API — a facade over
// gravity.db/queries.db shaped after Pi-hole's own v6 API, serving both
// JSON endpoints and the embedded dashboard static assets. No authentication:
// this is a single-user local tool bound to 127.0.0.1, by design.
package api

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"yoshi-pihole/internal/db"
	"yoshi-pihole/internal/gravity"
	"yoshi-pihole/internal/matcher"
	"yoshi-pihole/internal/service"
)

type Server struct {
	GravityStore *db.GravityStore
	QueryStore   *db.QueryStore
	Engine       *matcher.Engine
	Bedtime      *service.Bedtime
	Builder      *gravity.Builder
	// Reload is called after any change to domains/adlists so the matcher
	// engine picks up the new snapshot immediately.
	Reload func()
}

func NewRouter(s *Server, webFS fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)

	mux.HandleFunc("GET /api/stats/summary", s.handleStatsSummary)
	mux.HandleFunc("GET /api/queries", s.handleQueriesList)

	mux.HandleFunc("GET /api/domains", s.handleDomainsList)
	mux.HandleFunc("POST /api/domains", s.handleDomainsCreate)
	mux.HandleFunc("PATCH /api/domains/{id}", s.handleDomainsPatch)
	mux.HandleFunc("DELETE /api/domains/{id}", s.handleDomainsDelete)

	mux.HandleFunc("GET /api/lists", s.handleListsList)
	mux.HandleFunc("POST /api/lists", s.handleListsCreate)
	mux.HandleFunc("PATCH /api/lists/{id}", s.handleListsPatch)
	mux.HandleFunc("DELETE /api/lists/{id}", s.handleListsDelete)
	mux.HandleFunc("POST /api/lists/update", s.handleListsUpdate)

	mux.HandleFunc("GET /api/groups", s.handleGroupsList)
	mux.HandleFunc("POST /api/groups", s.handleGroupsCreate)
	mux.HandleFunc("PATCH /api/groups/{id}", s.handleGroupsPatch)

	mux.HandleFunc("GET /api/clients", s.handleClientsList)
	mux.HandleFunc("POST /api/clients", s.handleClientsCreate)

	mux.HandleFunc("GET /api/dns/blocking", s.handleBlockingGet)
	mux.HandleFunc("POST /api/dns/blocking", s.handleBlockingPost)

	mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"name": "Yoshi Pi-hole", "status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
