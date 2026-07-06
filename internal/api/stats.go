package api

import (
	"net/http"
	"strconv"
	"time"
)

func (s *Server) handleStatsSummary(w http.ResponseWriter, r *http.Request) {
	sinceSeconds := int64(86400) // default: last 24h
	if v := r.URL.Query().Get("since"); v != "" {
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			sinceSeconds = parsed
		}
	}

	summary, err := s.QueryStore.Summary(time.Now().Unix() - sinceSeconds)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	disabled, until := s.Bedtime.Status()
	engineStats := s.Engine.Stats()

	writeJSON(w, http.StatusOK, map[string]any{
		"total_queries":       summary.Total,
		"blocked_queries":     summary.Blocked,
		"forwarded_queries":   summary.Forwarded,
		"percent_blocked":     percent(summary.Blocked, summary.Total),
		"top_domains":         summary.TopDomains,
		"top_blocked_domains": summary.TopBlockedDomains,
		"query_types":         summary.QueryTypeCounts,
		"dropped_log_events":  s.QueryStore.Dropped(),
		"blocking_disabled":   disabled,
		"disabled_until":      until,
		"gravity_size":        engineStats.ExactDeny,
		"regex_deny_count":    engineStats.RegexDeny,
		"regex_allow_count":   engineStats.RegexAllow,
		"allow_count":         engineStats.ExactAllow,
	})
}

func percent(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}
