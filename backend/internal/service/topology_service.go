package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/network"

	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
)

// TopologyNode represents a container in the infrastructure graph.
type TopologyNode struct {
	ID         string `json:"id"`                    // container ID
	Name       string `json:"name"`                  // container name
	Image      string `json:"image"`                 // container image
	Type       string `json:"type"`                  // classified type (redis/postgres/mysql/mongo/proxy/web/generic)
	ProjectID  int64  `json:"project_id"`            // project ID (0 if system container)
	SystemType string `json:"system_type"`           // system type (e.g., "tamga-backend-1", "agent-system")
	State      string `json:"state"`                 // container state (running/exited/etc.)
	Status     string `json:"status"`                // human-readable status (e.g., "Up 3 hours")
	StatsRef   string `json:"stats_ref"`             // ref to per-container stats endpoint
	TrafficRef string `json:"traffic_ref,omitempty"` // ref for metrics (project-<id>), only for project nodes
}

// TopologyEdge represents a connection between two nodes via a docker network.
type TopologyEdge struct {
	Network string `json:"network"` // network name
	Source  string `json:"source"`  // source container name
	Target  string `json:"target"`  // target container name
}

// Topology is the full infra graph response.
type Topology struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// TopologyService builds topology graphs from docker containers and networks.
type TopologyService struct {
	docker *dockerclient.Client
}

func NewTopologyService(docker *dockerclient.Client) *TopologyService {
	return &TopologyService{docker: docker}
}

func (s *TopologyService) requireDocker() error {
	if s.docker == nil {
		return fmt.Errorf("docker daemon not available")
	}
	return nil
}

// ClassifyImage maps an image name to a semantic type for the topology.
// Uses simple substring matching (case-insensitive, first match).
func ClassifyImage(image string) string {
	imageLower := strings.ToLower(image)

	// Classification order matters - more specific matches first
	classifiers := []struct {
		substring string
		nodeType  string
	}{
		{"redis", "cache"},
		{"postgres", "database"},
		{"postgresql", "database"},
		{"mysql", "database"},
		{"mariadb", "database"},
		{"mongo", "database"},
		{"caddy", "proxy"},
		{"traefik", "proxy"},
		{"nginx", "proxy"},
		{"node", "web"},
		{"python", "web"},
		{"golang", "web"},
		{"ruby", "web"},
		{"php", "web"},
		{"rabbitmq", "queue"},
		{"kafka", "queue"},
	}

	for _, c := range classifiers {
		if strings.Contains(imageLower, c.substring) {
			return c.nodeType
		}
	}

	return "generic"
}

// isNoiseNetwork returns true if the network should be excluded from the topology
// (default docker networks that carry no meaningful app connectivity).
func isNoiseNetwork(networkName string) bool {
	noiseNetworks := map[string]bool{
		"bridge": true,
		"host":   true,
		"none":   true,
	}
	return noiseNetworks[networkName]
}

// GetSystemTopology returns the topology of all containers and their connections.
func (s *TopologyService) GetSystemTopology(ctx context.Context) (*Topology, error) {
	if err := s.requireDocker(); err != nil {
		return nil, err
	}

	containers, err := s.docker.ListContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	networks, err := s.docker.ListNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	// Build nodes from all containers
	nodes := s.buildNodes(containers)

	// Build edges from networks
	edges := s.buildEdgesFromNetworks(networks, containers)

	return &Topology{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// GetProjectTopology returns the topology for a specific project.
// Includes the project's containers and the traefik node (if traefik is running and
// connected to the project network).
func (s *TopologyService) GetProjectTopology(ctx context.Context, projectID int64) (*Topology, error) {
	if err := s.requireDocker(); err != nil {
		return nil, err
	}

	allContainers, err := s.docker.ListContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	// Filter to this project's containers
	var projectContainers []dockerclient.ContainerInfo
	for _, c := range allContainers {
		// Include containers that belong to this project or are agents for this project
		if c.ProjectID == projectID {
			projectContainers = append(projectContainers, c)
		}
	}

	// If no containers, return empty topology (not an error)
	if len(projectContainers) == 0 {
		return &Topology{
			Nodes: []TopologyNode{},
			Edges: []TopologyEdge{},
		}, nil
	}

	// Get all networks to find edges and traefik connection
	networks, err := s.docker.ListNetworks(ctx)
	if err != nil {
		return nil, fmt.Errorf("list networks: %w", err)
	}

	// Build nodes from project containers
	nodes := s.buildNodes(projectContainers)

	// Find the project's network (project-net-<id>)
	projectNetName := fmt.Sprintf("project-net-%d", projectID)
	var projectNetContainers map[string]string // container ID -> name
	for _, net := range networks {
		if net.Name == projectNetName {
			projectNetContainers = make(map[string]string)
			for containerID, endpoint := range net.Containers {
				projectNetContainers[containerID] = endpoint.Name
			}
			break
		}
	}

	// Find traefik container and check if it's on the project network
	var traefikNode *TopologyNode
	if projectNetContainers != nil {
		for _, c := range allContainers {
			if c.SystemType == "tamga-traefik-1" || strings.HasPrefix(c.Name, "traefik") {
				// Check if traefik is on the project network
				if _, isOnProjNet := projectNetContainers[c.ID]; isOnProjNet {
					traefikNode = s.containerToNode(c)
					nodes = append(nodes, *traefikNode)
					break
				}
			}
		}
	}

	// Build edges (only within the project network and between project containers)
	edges := s.buildProjectEdges(networks, projectContainers, projectNetName, traefikNode)

	return &Topology{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// buildNodes converts a list of containers to topology nodes.
func (s *TopologyService) buildNodes(containers []dockerclient.ContainerInfo) []TopologyNode {
	var nodes []TopologyNode
	for _, c := range containers {
		nodes = append(nodes, *s.containerToNode(c))
	}
	return nodes
}

// containerToNode converts a single container to a topology node.
func (s *TopologyService) containerToNode(c dockerclient.ContainerInfo) *TopologyNode {
	node := &TopologyNode{
		ID:         c.ID,
		Name:       c.Name,
		Image:      c.Image,
		Type:       ClassifyImage(c.Image),
		ProjectID:  c.ProjectID,
		SystemType: c.SystemType,
		State:      c.State,
		Status:     c.Status,
		StatsRef:   fmt.Sprintf("/api/system/containers/%s/stats", c.ID),
	}

	// Add traffic_ref for project containers (nodes with non-zero project_id)
	if c.ProjectID > 0 {
		node.TrafficRef = fmt.Sprintf("project-%d", c.ProjectID)
	}

	return node
}

// buildEdgesFromNetworks derives pairwise edges from network memberships.
// One edge per pair of containers sharing a network. Ignores noise networks.
func (s *TopologyService) buildEdgesFromNetworks(networks []network.Inspect, allContainers []dockerclient.ContainerInfo) []TopologyEdge {
	var edges []TopologyEdge
	edgeSeen := make(map[string]bool) // track seen edges to de-duplicate

	// Create container name lookup
	containersByID := make(map[string]string)
	for _, c := range allContainers {
		containersByID[c.ID] = c.Name
	}

	// For each network, derive edges between containers attached to it
	for _, net := range networks {
		if isNoiseNetwork(net.Name) {
			continue // Skip default bridge/host/none networks
		}

		// Get container names attached to this network
		var containerNames []string
		for containerID := range net.Containers {
			if name, ok := containersByID[containerID]; ok {
				containerNames = append(containerNames, name)
			}
		}

		// Create pairwise edges between all containers on this network
		for i := 0; i < len(containerNames); i++ {
			for j := i + 1; j < len(containerNames); j++ {
				source := containerNames[i]
				target := containerNames[j]

				// De-duplicate: normalize direction
				key := edgeKey(source, target, net.Name)
				if edgeSeen[key] {
					continue
				}
				edgeSeen[key] = true

				edges = append(edges, TopologyEdge{
					Network: net.Name,
					Source:  source,
					Target:  target,
				})
			}
		}
	}

	return edges
}

// buildProjectEdges builds edges for containers in a specific project.
// Only includes edges on the project-specific network.
func (s *TopologyService) buildProjectEdges(networks []network.Inspect, projectContainers []dockerclient.ContainerInfo, projectNetName string, traefikNode *TopologyNode) []TopologyEdge {
	var edges []TopologyEdge
	edgeSeen := make(map[string]bool)

	// Create lookup of project container names (project containers + traefik)
	projectContainerNames := make(map[string]bool)
	for _, c := range projectContainers {
		projectContainerNames[c.Name] = true
	}

	if traefikNode != nil {
		projectContainerNames[traefikNode.Name] = true
	}

	// Find the project network and derive edges from it
	for _, net := range networks {
		if net.Name != projectNetName {
			continue
		}

		if isNoiseNetwork(net.Name) {
			continue
		}

		// Get container names on this network that are in our project
		var containerNames []string
		for _, endpoint := range net.Containers {
			if projectContainerNames[endpoint.Name] {
				containerNames = append(containerNames, endpoint.Name)
			}
		}

		// Create pairwise edges
		for i := 0; i < len(containerNames); i++ {
			for j := i + 1; j < len(containerNames); j++ {
				source := containerNames[i]
				target := containerNames[j]

				key := edgeKey(source, target, net.Name)
				if edgeSeen[key] {
					continue
				}
				edgeSeen[key] = true

				edges = append(edges, TopologyEdge{
					Network: net.Name,
					Source:  source,
					Target:  target,
				})
			}
		}
	}

	return edges
}

// Helper function to create a de-dup key for edges
func edgeKey(source, target, network string) string {
	// Normalize to ensure (A,B) and (B,A) are the same edge
	if source > target {
		source, target = target, source
	}
	return fmt.Sprintf("%s:%s:%s", source, target, network)
}
