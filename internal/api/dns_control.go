package api

import (
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleBlockingGet(w http.ResponseWriter, r *http.Request) {
	disabled, until := s.Bedtime.Status()
	writeJSON(w, http.StatusOK, map[string]any{"blocking": !disabled, "disabled_until": until})
}

// handleBlockingPost toggles blocking on/off, with an optional "timer"
// (seconds) for a temporary disable — Pi-hole's "bedtime mode" primitive.
func (s *Server) handleBlockingPost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Blocking bool `json:"blocking"`
		Timer    int  `json:"timer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if body.Blocking {
		s.Bedtime.Enable()
	} else {
		s.Bedtime.Disable(time.Duration(body.Timer) * time.Second)
	}

	disabled, until := s.Bedtime.Status()
	writeJSON(w, http.StatusOK, map[string]any{"blocking": !disabled, "disabled_until": until})
}
