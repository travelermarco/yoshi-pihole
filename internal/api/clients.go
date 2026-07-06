package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) handleClientsList(w http.ResponseWriter, r *http.Request) {
	clients, err := s.GravityStore.ListClients()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"clients": clients})
}

func (s *Server) handleClientsCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IP      string `json:"ip"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.IP == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip is required"})
		return
	}
	id, err := s.GravityStore.AddClient(body.IP, body.Comment)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}
