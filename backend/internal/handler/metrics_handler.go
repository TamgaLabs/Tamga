package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// defaultMetricsWindow is the recent-window default MetricsHandler falls
// back to when `from`/`to` are omitted from the query string (task
// requirement: "from/to default to a recent window").
const defaultMetricsWindow = 24 * time.Hour

type MetricsHandler struct {
	svc *service.MetricsQueryService
}

func NewMetricsHandler(svc *service.MetricsQueryService) *MetricsHandler {
	return &MetricsHandler{svc: svc}
}

// System handles GET /api/system/metrics - the global/core scope
// (domain.GlobalProjectID), not any one project.
func (h *MetricsHandler) System(w http.ResponseWriter, r *http.Request) {
	h.respond(w, r, domain.GlobalProjectID)
}

func (h *MetricsHandler) respond(w http.ResponseWriter, r *http.Request, projectID int64) {
	from, to, err := parseMetricsRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resolution := domain.MetricResolution(r.URL.Query().Get("resolution"))

	panels, err := h.svc.GetPanels(projectID, from, to, resolution)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(panels)
}

// parseMetricsRange parses the `from`/`to` query params (Unix seconds - the
// same unit bucket_start is already stored/compared at internally, so
// callers stay in one timestamp format end to end), defaulting to the last
// defaultMetricsWindow when omitted.
func parseMetricsRange(r *http.Request) (from, to time.Time, err error) {
	to = time.Now().UTC()
	if v := r.URL.Query().Get("to"); v != "" {
		to, err = parseUnixSeconds(v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid to: %w", err)
		}
	}

	from = to.Add(-defaultMetricsWindow)
	if v := r.URL.Query().Get("from"); v != "" {
		from, err = parseUnixSeconds(v)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid from: %w", err)
		}
	}

	if !to.After(from) {
		return time.Time{}, time.Time{}, fmt.Errorf("to must be after from")
	}
	return from, to, nil
}

func parseUnixSeconds(v string) (time.Time, error) {
	sec, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("must be a unix timestamp (seconds): %w", err)
	}
	return time.Unix(sec, 0).UTC(), nil
}
