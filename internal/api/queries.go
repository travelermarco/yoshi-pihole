package api

import (
	"net/http"
	"strconv"

	"yoshi-pihole/internal/db"
)

func (s *Server) handleQueriesList(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	f := db.Filter{Domain: r.URL.Query().Get("domain")}
	if v := r.URL.Query().Get("status"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			f.Status = &parsed
		}
	}
	if v := r.URL.Query().Get("since"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.Since = parsed
		}
	}

	results, err := s.QueryStore.Recent(limit, offset, f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queries": results})
}
