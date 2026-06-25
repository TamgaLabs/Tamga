package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Message     string     `json:"message"`
	Response    string     `json:"response"`
	Diff        string     `json:"diff,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	mu          sync.Mutex
}

type Bridge struct {
	mu       sync.Mutex
	tasks    map[string]*Task
	agentCmd string
}

func NewBridge(agentCmd string) *Bridge {
	return &Bridge{
		tasks:    make(map[string]*Task),
		agentCmd: agentCmd,
	}
}

func (b *Bridge) HandleChat(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID     string `json:"task_id"`
		Message    string `json:"message"`
		ProjectDir string `json:"project_dir"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	taskID := req.TaskID
	if taskID == "" {
		taskID = uuid.New().String()
	}

	task := &Task{
		ID:      taskID,
		Status:  "processing",
		Message: req.Message,
	}

	b.mu.Lock()
	b.tasks[taskID] = task
	b.mu.Unlock()

	go b.runAgent(task, req.ProjectDir, req.Message)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func (b *Bridge) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/tasks/")
	if taskID == "" {
		http.Error(w, "task id required", http.StatusBadRequest)
		return
	}

	b.mu.Lock()
	task, ok := b.tasks[taskID]
	b.mu.Unlock()

	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

func (b *Bridge) runAgent(task *Task, projectDir, message string) {
	defer func() {
		task.mu.Lock()
		if task.Status != "completed" && task.Status != "failed" {
			task.Status = "completed"
		}
		now := time.Now()
		task.CompletedAt = &now
		task.mu.Unlock()
	}()

	task.mu.Lock()
	task.Status = "processing"
	task.mu.Unlock()

	parts := strings.Fields(b.agentCmd)
	if len(parts) == 0 {
		task.mu.Lock()
		task.Status = "failed"
		task.Response = "no agent command configured"
		task.mu.Unlock()
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = projectDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		task.mu.Lock()
		task.Status = "failed"
		task.Response = fmt.Sprintf("stdin pipe: %v", err)
		task.mu.Unlock()
		return
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		task.mu.Lock()
		task.Status = "failed"
		task.Response = fmt.Sprintf("stdout pipe: %v", err)
		task.mu.Unlock()
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		task.mu.Lock()
		task.Status = "failed"
		task.Response = fmt.Sprintf("stderr pipe: %v", err)
		task.mu.Unlock()
		return
	}

	if err := cmd.Start(); err != nil {
		task.mu.Lock()
		task.Status = "failed"
		task.Response = fmt.Sprintf("start agent: %v", err)
		task.mu.Unlock()
		return
	}

	io.WriteString(stdin, message+"\n")
	stdin.Close()

	go io.Copy(os.Stderr, stderr)

	var output strings.Builder
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		output.WriteString(scanner.Text())
		output.WriteString("\n")
	}

	cmd.Wait()

	raw := output.String()
	response, diff := splitDiff(raw)

	task.mu.Lock()
	task.Response = response
	task.Diff = diff
	task.Status = "completed"
	task.mu.Unlock()
}

func splitDiff(output string) (response, diff string) {
	idx := strings.Index(output, "---DIFF---")
	if idx >= 0 {
		response = strings.TrimSpace(output[:idx])
		diff = strings.TrimSpace(output[idx+len("---DIFF---"):])
		return
	}

	lines := strings.Split(output, "\n")
	var respLines, diffLines []string
	inDiff := false
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "```diff") {
			inDiff = true
		}
		if inDiff {
			diffLines = append(diffLines, line)
		} else {
			respLines = append(respLines, line)
		}
	}

	response = strings.TrimSpace(strings.Join(respLines, "\n"))
	diff = strings.TrimSpace(strings.Join(diffLines, "\n"))
	return
}

func envVar(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	cwd, _ := os.Getwd()
	log.Printf("agent-bridge starting, cwd=%s", cwd)

	agentCmd := envVar("AGENT_CMD", "opencode --stdin --diff")
	port := envVar("PORT", "9000")

	entries, _ := os.ReadDir(cwd)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	log.Printf("workspace contents: %s", strings.Join(names, ", "))

	bridge := NewBridge(agentCmd)

	mux := http.NewServeMux()
	mux.HandleFunc("/chat", bridge.HandleChat)
	mux.HandleFunc("/tasks/", bridge.HandleGetTask)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := fmt.Sprintf(":%s", port)
	log.Printf("agent-bridge listening on %s, agent=%s", addr, agentCmd)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func init() {
	_ = filepath.Join
}
