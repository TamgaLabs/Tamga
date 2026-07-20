package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

type CodeHandler struct {
	sealSvc *service.SealService
	cfg     config.Config
}

func NewCodeHandler(sealSvc *service.SealService, cfg config.Config) *CodeHandler {
	return &CodeHandler{sealSvc: sealSvc, cfg: cfg}
}

type Codebase struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // "project" or "system"
	Path      string `json:"path"`
	ProjectID int64  `json:"project_id,omitempty"`
}

func (h *CodeHandler) ListCodebases(w http.ResponseWriter, r *http.Request) {
	var codebases []Codebase

	seals, err := h.sealSvc.List(r.Context())
	if err == nil {
		for _, seal := range seals {
			projects, err := h.sealSvc.ListProjects(r.Context(), seal.ID)
			if err != nil {
				continue
			}
			for _, p := range projects {
				path, err := h.sealSvc.ProjectCheckoutPath(seal.ID, p)
				if err != nil {
					continue
				}
				codebases = append(codebases, Codebase{
					ID:        p.ID,
					Name:      p.Name,
					Type:      "project",
					Path:      path,
					ProjectID: p.ID,
				})
			}
		}
	}

	sysPath := h.cfg.SystemCodeDir
	if sysPath != "" {
		codebases = append(codebases, Codebase{
			ID:   0,
			Name: "Tamga (System)",
			Type: "system",
			Path: sysPath,
		})
	}

	if codebases == nil {
		codebases = []Codebase{}
	}
	json.NewEncoder(w).Encode(codebases)
}

type FileEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "dir"
	Size int64  `json:"size,omitempty"`
}

func (h *CodeHandler) getProjectDir(ctx context.Context, projectID int64) (string, error) {
	if projectID == 0 || projectID == -1 {
		return h.cfg.SystemCodeDir, nil
	}
	seals, err := h.sealSvc.List(ctx)
	if err != nil {
		return "", err
	}
	for _, seal := range seals {
		project, err := h.sealSvc.FindProject(ctx, seal.ID, projectID)
		if err == nil {
			return h.sealSvc.ProjectCheckoutPath(seal.ID, project)
		}
	}
	return "", fmt.Errorf("project root not found")
}

func (h *CodeHandler) FileTree(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	root, err := h.getProjectDir(r.Context(), pid)
	if err != nil || root == "" {
		http.Error(w, "project root not found", http.StatusNotFound)
		return
	}

	var entries []FileEntry
	filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		// skip .git
		if strings.HasPrefix(rel, ".git") || strings.Contains(rel, "/.git/") {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// skip node_modules
		if strings.HasPrefix(rel, "node_modules") || strings.Contains(rel, "/node_modules/") {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entryType := "file"
		if fi.IsDir() {
			entryType = "dir"
		}
		entries = append(entries, FileEntry{
			Name: fi.Name(),
			Path: rel,
			Type: entryType,
			Size: fi.Size(),
		})
		return nil
	})

	if entries == nil {
		entries = []FileEntry{}
	}
	json.NewEncoder(w).Encode(entries)
}

func (h *CodeHandler) ReadFile(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	root, err := h.getProjectDir(r.Context(), pid)
	if err != nil || root == "" {
		http.Error(w, "project root not found", http.StatusNotFound)
		return
	}
	fullPath := filepath.Join(root, filePath)

	if !strings.HasPrefix(fullPath, root) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"content": string(content),
	})
}

func (h *CodeHandler) WriteFile(w http.ResponseWriter, r *http.Request) {
	pid, err := strconv.ParseInt(chi.URLParam(r, "projectID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid project id", http.StatusBadRequest)
		return
	}

	if pid == 0 {
		http.Error(w, "system codebase is read-only", http.StatusForbidden)
		return
	}

	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	root, err := h.getProjectDir(r.Context(), pid)
	if err != nil || root == "" {
		http.Error(w, "project root not found", http.StatusNotFound)
		return
	}
	fullPath := filepath.Join(root, filePath)

	if !strings.HasPrefix(fullPath, root) {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *CodeHandler) TreeAsJSON(entries []FileEntry) []map[string]interface{} {
	type node struct {
		name     string
		path     string
		children map[string]*node
		isDir    bool
	}
	root := &node{children: make(map[string]*node)}

	for _, e := range entries {
		parts := strings.Split(e.Path, string(filepath.Separator))
		current := root
		for i, part := range parts {
			if _, ok := current.children[part]; !ok {
				current.children[part] = &node{
					name:     part,
					path:     strings.Join(parts[:i+1], string(filepath.Separator)),
					children: make(map[string]*node),
					isDir:    i < len(parts)-1 || e.Type == "dir",
				}
			}
			current = current.children[part]
		}
	}

	var buildTree func(*node) map[string]interface{}
	buildTree = func(n *node) map[string]interface{} {
		m := map[string]interface{}{
			"name": n.name,
			"path": n.path,
			"type": "dir",
		}
		if !n.isDir {
			m["type"] = "file"
			return m
		}
		var children []map[string]interface{}
		for _, child := range n.children {
			children = append(children, buildTree(child))
		}
		if children == nil {
			children = []map[string]interface{}{}
		}
		m["children"] = children
		return m
	}

	var result []map[string]interface{}
	for _, child := range root.children {
		result = append(result, buildTree(child))
	}
	return result
}
