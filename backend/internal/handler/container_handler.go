package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"
)

type ContainerHandler struct {
	docker *dockerclient.Client
}

func NewContainerHandler(docker *dockerclient.Client) *ContainerHandler {
	return &ContainerHandler{docker: docker}
}

func (h *ContainerHandler) requireDocker(w http.ResponseWriter) bool {
	if h.docker == nil {
		http.Error(w, "docker daemon not available", http.StatusServiceUnavailable)
		return false
	}
	return true
}

func (h *ContainerHandler) List(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	containers, err := h.docker.ListContainers(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(containers)
}

func (h *ContainerHandler) Inspect(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	info, err := h.docker.InspectContainer(r.Context(), id)
	if err != nil {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(info)
}

func (h *ContainerHandler) Start(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.docker.StartContainer(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ContainerHandler) Stop(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.docker.StopContainer(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ContainerHandler) Restart(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.docker.RestartContainer(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ContainerHandler) Remove(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.docker.RemoveContainer(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ContainerHandler) Logs(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	tail := 100
	if t := r.URL.Query().Get("tail"); t != "" {
		if v, err := strconv.Atoi(t); err == nil && v > 0 {
			tail = v
		}
	}
	logs, err := h.docker.ContainerLogs(r.Context(), id, tail)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"logs": logs})
}

func (h *ContainerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	stats, err := h.docker.ContainerStats(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	type cpuStats struct {
		Percent float64  `json:"percent"`
		Usage   uint64   `json:"usage"`
		System  uint64   `json:"system"`
		Percpu  []uint64 `json:"percpu,omitempty"`
	}
	type memStats struct {
		Usage   uint64  `json:"usage"`
		Limit   uint64  `json:"limit"`
		Percent float64 `json:"percent"`
	}
	type netStats struct {
		RxBytes   uint64 `json:"rx_bytes"`
		TxBytes   uint64 `json:"tx_bytes"`
		RxPackets uint64 `json:"rx_packets"`
		TxPackets uint64 `json:"tx_packets"`
	}

	cpu := cpuStats{
		Usage:  stats.CPUStats.CPUUsage.TotalUsage,
		System: stats.CPUStats.SystemUsage,
		Percpu: stats.CPUStats.CPUUsage.PercpuUsage,
	}
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	if sysDelta > 0 && cpuDelta > 0 {
		cpu.Percent = (cpuDelta / sysDelta) * 100.0
	}

	mem := memStats{
		Usage: stats.MemoryStats.Usage,
		Limit: stats.MemoryStats.Limit,
	}
	if stats.MemoryStats.Limit > 0 {
		mem.Percent = float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100.0
	}

	var net netStats
	for _, n := range stats.Networks {
		net.RxBytes += n.RxBytes
		net.TxBytes += n.TxBytes
		net.RxPackets += n.RxPackets
		net.TxPackets += n.TxPackets
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"cpu": cpu,
		"mem": mem,
		"net": net,
	})
}

func (h *ContainerHandler) UpdateResources(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	id := chi.URLParam(r, "id")
	var req struct {
		Memory   int64 `json:"memory,omitempty"`
		NanoCPUs int64 `json:"nano_cpus,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resources := container.Resources{}
	if req.Memory > 0 {
		resources.Memory = req.Memory
	}
	if req.NanoCPUs > 0 {
		resources.NanoCPUs = req.NanoCPUs
	}

	if err := h.docker.UpdateContainerResources(r.Context(), id, container.UpdateConfig{Resources: resources}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h *ContainerHandler) Prune(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	var req struct {
		Containers bool `json:"containers"`
		Images     bool `json:"images"`
		Volumes    bool `json:"volumes"`
		Networks   bool `json:"networks"`
		All        bool `json:"all"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.All = true
	}

	ctx := r.Context()
	if req.All || req.Containers {
		h.docker.PruneContainers(ctx)
	}
	if req.All || req.Images {
		h.docker.PruneImages(ctx)
	}
	if req.All || req.Volumes {
		h.docker.PruneVolumes(ctx)
	}
	if req.All || req.Networks {
		h.docker.PruneNetworks(ctx)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "pruned"})
}

func (h *ContainerHandler) Info(w http.ResponseWriter, r *http.Request) {
	if !h.requireDocker(w) {
		return
	}
	info, err := h.docker.DockerInfo(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"version":      info.ServerVersion,
		"os":           info.OperatingSystem,
		"architecture": info.Architecture,
		"containers":   info.Containers,
		"running":      info.ContainersRunning,
		"paused":       info.ContainersPaused,
		"stopped":      info.ContainersStopped,
		"images":       info.Images,
		"name":         info.Name,
		"kernel":       info.KernelVersion,
		"driver":       info.Driver,
		"memory":       info.MemTotal,
		"cpus":         info.NCPU,
	})
}
