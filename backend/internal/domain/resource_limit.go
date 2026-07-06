package domain

// ResourceLimit is the global default CPU/memory limit applied to every
// agent sandbox container at creation time (see FEAT-007). Unlike
// WhitelistDomain/ApiKey, this is a single-row setting - there's only ever
// one active default, not a list of entries.
//
// Units match the existing UpdateResources admin endpoint
// (container_handler.go): MemoryBytes is bytes, NanoCPUs is CPUs * 1e9
// (Docker's own NanoCPUs convention).
type ResourceLimit struct {
	MemoryBytes int64 `json:"memory_bytes"`
	NanoCPUs    int64 `json:"nano_cpus"`
}
