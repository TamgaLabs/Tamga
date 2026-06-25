package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

type AgentProvider interface {
	ForwardTask(ctx context.Context, taskID, message, projectDir string) error
	PollTask(ctx context.Context, taskID string, task *domain.AgentTask) error
	Ping(ctx context.Context) error
}

type dockerBridge struct {
	containerName string
	port          string
	httpClient    *http.Client
}

func newDockerBridge(containerName, port string) *dockerBridge {
	return &dockerBridge{
		containerName: containerName,
		port:          port,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *dockerBridge) ForwardTask(ctx context.Context, taskID, message, projectDir string) error {
	agentURL := fmt.Sprintf("http://%s:%s/chat", b.containerName, b.port)
	payload := map[string]string{
		"task_id":     taskID,
		"message":     message,
		"project_dir": projectDir,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", agentURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("forward to agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("agent returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (b *dockerBridge) PollTask(ctx context.Context, taskID string, task *domain.AgentTask) error {
	agentURL := fmt.Sprintf("http://%s:%s/tasks/%s", b.containerName, b.port, taskID)
	req, err := http.NewRequestWithContext(ctx, "GET", agentURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("poll agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent returned %d", resp.StatusCode)
	}

	var agentTask struct {
		Status      string `json:"status"`
		Response    string `json:"response"`
		Diff        string `json:"diff"`
		CompletedAt string `json:"completed_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agentTask); err != nil {
		return fmt.Errorf("decode agent task: %w", err)
	}

	task.Status = domain.AgentTaskStatus(agentTask.Status)
	task.Response = agentTask.Response
	task.Diff = agentTask.Diff
	if agentTask.CompletedAt != "" {
		t, _ := time.Parse(time.RFC3339, agentTask.CompletedAt)
		task.CompletedAt = &t
	}
	return nil
}

func (b *dockerBridge) Ping(ctx context.Context) error {
	agentURL := fmt.Sprintf("http://%s:%s/health", b.containerName, b.port)
	req, err := http.NewRequestWithContext(ctx, "GET", agentURL, nil)
	if err != nil {
		return err
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type httpBridge struct {
	endpoint   string
	authToken  string
	httpClient *http.Client
}

func newHTTPBridge(endpoint, authToken string) *httpBridge {
	return &httpBridge{
		endpoint:   endpoint,
		authToken:  authToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (b *httpBridge) ForwardTask(ctx context.Context, taskID, message, projectDir string) error {
	url := fmt.Sprintf("%s/chat", b.endpoint)
	payload := map[string]string{
		"task_id":     taskID,
		"message":     message,
		"project_dir": projectDir,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if b.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.authToken)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("forward to remote agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("remote agent returned %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (b *httpBridge) PollTask(ctx context.Context, taskID string, task *domain.AgentTask) error {
	url := fmt.Sprintf("%s/tasks/%s", b.endpoint, taskID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if b.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.authToken)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("poll remote agent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("remote agent returned %d", resp.StatusCode)
	}

	var agentTask struct {
		Status      string `json:"status"`
		Response    string `json:"response"`
		Diff        string `json:"diff"`
		CompletedAt string `json:"completed_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&agentTask); err != nil {
		return fmt.Errorf("decode remote agent task: %w", err)
	}

	task.Status = domain.AgentTaskStatus(agentTask.Status)
	task.Response = agentTask.Response
	task.Diff = agentTask.Diff
	if agentTask.CompletedAt != "" {
		t, _ := time.Parse(time.RFC3339, agentTask.CompletedAt)
		task.CompletedAt = &t
	}
	return nil
}

func (b *httpBridge) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", b.endpoint+"/health", nil)
	if err != nil {
		return err
	}
	if b.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+b.authToken)
	}
	resp, err := b.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
