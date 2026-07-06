package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"yoshi-pihole/internal/gravity"
)

func (s *Server) handleDomainsList(w http.ResponseWriter, r *http.Request) {
	domains, err := s.GravityStore.ListDomains()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"domains": domains})
}

func (s *Server) handleDomainsCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Domain  string `json:"domain"`
		Type    int    `json:"type"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Domain == "" || body.Type < 0 || body.Type > 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required and type must be 0-3"})
		return
	}
	if body.Type == 2 || body.Type == 3 {
		if _, err := gravity.ParseRegex(body.Domain); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid regex: " + err.Error()})
			return
		}
	}

	id, err := s.GravityStore.AddDomain(body.Domain, body.Type, body.Comment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.Reload()
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (s *Server) handleDomainsPatch(w http.ResponseWriter, r *http.Request) {
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
	if err := s.GravityStore.SetDomainEnabled(id, body.Enabled); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleDomainsDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := s.GravityStore.RemoveDomain(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.Reload()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
