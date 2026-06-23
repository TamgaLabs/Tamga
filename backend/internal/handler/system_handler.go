package handler

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

type SystemHandler struct {
	startTime time.Time
}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{startTime: time.Now()}
}

func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"uptime":    time.Since(h.startTime).String(),
		"go_version": runtime.Version(),
	})
}
