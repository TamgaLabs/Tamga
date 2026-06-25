package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func sseWriter(w http.ResponseWriter) func(string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		return func(s string) {
			fmt.Fprintf(w, "data: %s\n\n", s)
		}
	}
	return func(s string) {
		fmt.Fprintf(w, "data: %s\n\n", s)
		flusher.Flush()
	}
}

type AgentHandler struct {
	svc *service.AgentService
}

func NewAgentHandler(svc *service.AgentService) *AgentHandler {
	return &AgentHandler{svc: svc}
}

func (h *AgentHandler) ChatStream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req struct {
		Message   string        `json:"message"`
		Messages  []interface{} `json:"messages"`
		SessionID *string       `json:"session_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	message := req.Message
	if message == "" && len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1]
		if m, ok := lastMsg.(map[string]interface{}); ok {
			if c, ok := m["content"].(string); ok {
				message = c
			}
		}
	}
	if message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	write := sseWriter(w)
	events, err := h.svc.ChatStream(r.Context(), id, message, req.SessionID)
	if err != nil {
		write(fmt.Sprintf(`{"type":"text","text":%s}`, jsonEncodeStr(err.Error())))
		return
	}

	for event := range events {
		write(event)
	}
}

func (h *AgentHandler) Chat(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	var req struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	task, err := h.svc.Chat(r.Context(), id, req.Message, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"task_id": task.ID})
}

func (h *AgentHandler) GetTask(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	taskID := chi.URLParam(r, "taskId")
	if taskID == "" {
		http.Error(w, "task id required", http.StatusBadRequest)
		return
	}

	task, err := h.svc.GetTask(r.Context(), id, taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(task)
}

func (h *AgentHandler) ListTasks(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	tasks, err := h.svc.ListTasks(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []*domain.AgentTask{}
	}

	json.NewEncoder(w).Encode(tasks)
}

func jsonEncodeStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
