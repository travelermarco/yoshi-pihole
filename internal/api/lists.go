package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"yoshi-pihole/internal/db"
)

func (s *Server) handleListsList(w http.ResponseWriter, r *http.Request) {
	lists, err := s.GravityStore.ListAdlists()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"lists": lists})
}

func (s *Server) handleListsCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Address string `json:"address"`
		Type    int    `json:"type"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Address == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "address is required"})
		return
	}
	if body.Type != db.AdlistAllow {
		body.Type = db.AdlistBlock
	}

	id, err := s.GravityStore.AddAdlist(body.Address, body.Type, body.Comment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleListsPatch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := s.GravityStore.SetAdlistEnabled(id, body.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleListsDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.GravityStore.RemoveAdlist(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleListsUpdate triggers a gravity rebuild (fetch + reparse every enabled
// adlist). It runs asynchronously since downloading lists can take seconds;
// the dashboard polls /api/lists afterward to see updated counts/status.
func (s *Server) handleListsUpdate(w http.ResponseWriter, r *http.Request) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.Builder.Run(ctx); err != nil {
			log.Printf("api: gravity update failed: %v", err)
		}
		s.Reload()
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "update started"})
}
