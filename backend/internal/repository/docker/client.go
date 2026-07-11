package docker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Client struct {
	cli *client.Client
}

func New() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) BuildImage(ctx context.Context, tag, dockerfile string, buildCtx io.Reader) error {
	resp, err := c.cli.ImageBuild(ctx, buildCtx, build.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: dockerfile,
		Remove:     true,
		PullParent: true,
	})
	if err != nil {
		return fmt.Errorf("build image: %w", err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	return err
}

// PullImage pulls ref (e.g. "redis:7-alpine") from its registry, blocking
// until the pull completes. Public images only - no registry auth is
// wired up (FEAT-026: private registries are out of scope for the compose
// deploy engine). ImagePull returns a streaming progress reader as soon as
// the pull starts, not when it finishes, so the reader must be read to EOF
// (discarding the JSON progress lines - callers don't need them) before the
// image is actually usable by a subsequent ContainerCreate.
func (c *Client) PullImage(ctx context.Context, ref string) error {
	reader, err := c.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %s: %w", ref, err)
	}
	defer reader.Close()
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("pull image %s: %w", ref, err)
	}
	return nil
}

func (c *Client) CreateContainer(ctx context.Context, name, imageName string, env []string, network string) (string, error) {
	return c.CreateContainerOpts(ctx, name, imageName, env, network, nil, container.Resources{}, false, nil)
}

// CreateContainerOpts creates a container with the given mounts and
// resource limits applied via HostConfig.Resources (zero value means no
// limit - i.e. Docker's own default). See FEAT-007: agent sandbox creation
// always passes a non-zero Resources so no sandbox is ever created
// unlimited.
//
// aliases sets the container's network aliases on the network it's created
// on (network's DNS name(s) beyond its own container name/ID, which Docker
// always resolves automatically). This is what makes a compose service
// reachable by its bare service name (e.g. "redis") rather than only by
// its full container name ("project-<id>-redis") - real docker compose
// does the same by aliasing every service container with its service name
// on the compose network. A nil/empty aliases leaves behavior exactly as
// before (no extra alias, matching every pre-FEAT-028 caller). Aliases
// only take effect on user-defined networks (Docker ignores them on the
// default "bridge"/"host"/"none" network modes), which is fine since
// callers that pass aliases always create on a project's own bridge
// network. NetworkMode is set to the same network name as the
// EndpointsConfig entry - the Docker API merges the two for a
// user-defined network at create time (this is exactly how the `docker
// compose` CLI itself wires up service aliases).
func (c *Client) CreateContainerOpts(ctx context.Context, name, imageName string, env []string, netName string, mounts []string, resources container.Resources, initProcess bool, aliases []string) (string, error) {
	hostCfg := &container.HostConfig{
		NetworkMode:   container.NetworkMode(netName),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
		Resources:     resources,
	}
	if initProcess {
		initVal := true
		hostCfg.Init = &initVal
	}
	for _, m := range mounts {
		parts := strings.SplitN(m, ":", 2)
		if len(parts) == 2 {
			hostCfg.Binds = append(hostCfg.Binds, m)
		}
	}
	var netCfg *network.NetworkingConfig
	if len(aliases) > 0 {
		netCfg = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				netName: {Aliases: aliases},
			},
		}
	}
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Env:   env,
	}, hostCfg, netCfg, nil, name)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}
	return resp.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (c *Client) StopContainerTimeout(ctx context.Context, containerID string, timeoutSecs int) error {
	timeout := timeoutSecs
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
}

func (c *Client) ContainerLogs(ctx context.Context, containerID string, tail int) (string, error) {
	return c.ContainerLogsSince(ctx, containerID, tail, "")
}

func (c *Client) ContainerLogsSince(ctx context.Context, containerID string, tail int, since string) (string, error) {
	resp, err := c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
		Since:      since,
	})
	if err != nil {
		return "", fmt.Errorf("container logs: %w", err)
	}
	defer resp.Close()

	var buf bytes.Buffer
	stdcopy.StdCopy(&buf, &buf, resp)
	return buf.String(), nil
}

func (c *Client) ContainerExists(ctx context.Context, name string) bool {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return false
	}
	for _, ct := range containers {
		for _, n := range ct.Names {
			if strings.TrimPrefix(n, "/") == name {
				return true
			}
		}
	}
	return false
}

func (c *Client) ContainerIsRunning(ctx context.Context, name string) bool {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: false})
	if err != nil {
		return false
	}
	for _, ct := range containers {
		for _, n := range ct.Names {
			if strings.TrimPrefix(n, "/") == name {
				return true
			}
		}
	}
	return false
}

func (c *Client) GetContainerPort(ctx context.Context, containerID string) (string, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspect container: %w", err)
	}
	for port := range info.NetworkSettings.Ports {
		return port.Port(), nil
	}
	return "80", nil
}

type ContainerInfo struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Status     string            `json:"status"`
	State      string            `json:"state"`
	Ports      []string          `json:"ports"`
	Created    time.Time         `json:"created"`
	Labels     map[string]string `json:"labels"`
	ProjectID  int64             `json:"project_id,omitempty"`
	SystemType string            `json:"system_type,omitempty"`
}

// containerProjectInfo derives ListContainers' project_id/system_type
// attribution from a container's name, matching Tamga's naming
// conventions: a project's service containers are named
// "project-<id>-<service>" (FEAT-028's deploy_engine.go
// serviceContainerName) - fmt.Sscanf's "%d" verb stops at the first
// non-digit it hits, so it parses the leading "<id>" correctly whether or
// not a "-<service>" suffix follows, which also keeps this working
// unchanged for a pre-FEAT-028 project's legacy single "project-<id>"
// container name. Agent sandboxes ("agent-<id>", or the singleton
// "agent-system") and Tamga's own system containers ("caddy", "tamga-*")
// are the other two naming families ListContainers has always
// recognized.
func containerProjectInfo(name string) (projectID int64, systemType string) {
	switch {
	case strings.HasPrefix(name, "project-"):
		fmt.Sscanf(name, "project-%d", &projectID)
	case name == "agent-system":
		systemType = "agent-system"
	case strings.HasPrefix(name, "agent-"):
		fmt.Sscanf(name, "agent-%d", &projectID)
	case name == "caddy" || strings.HasPrefix(name, "tamga-"):
		systemType = name
	}
	return projectID, systemType
}

func (c *Client) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	var result []ContainerInfo
	for _, ct := range containers {
		name := ""
		if len(ct.Names) > 0 {
			name = strings.TrimPrefix(ct.Names[0], "/")
		}
		ports := []string{}
		for _, p := range ct.Ports {
			if p.PublicPort > 0 {
				ports = append(ports, fmt.Sprintf("%d→%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				ports = append(ports, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}

		projectID, systemType := containerProjectInfo(name)

		result = append(result, ContainerInfo{
			ID:         ct.ID,
			Name:       name,
			Image:      ct.Image,
			Status:     ct.Status,
			State:      ct.State,
			Ports:      ports,
			Created:    time.Unix(ct.Created, 0),
			Labels:     ct.Labels,
			ProjectID:  projectID,
			SystemType: systemType,
		})
	}
	return result, nil
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	info, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return types.ContainerJSON{}, fmt.Errorf("inspect container: %w", err)
	}
	return info, nil
}

func (c *Client) ContainerStats(ctx context.Context, containerID string) (*container.Stats, error) {
	resp, err := c.cli.ContainerStats(ctx, containerID, false)
	if err != nil {
		return nil, fmt.Errorf("container stats: %w", err)
	}
	defer resp.Body.Close()

	var stats container.Stats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		return nil, fmt.Errorf("decode stats: %w", err)
	}
	return &stats, nil
}

func (c *Client) RestartContainer(ctx context.Context, containerID string) error {
	return c.cli.ContainerRestart(ctx, containerID, container.StopOptions{})
}

func (c *Client) UpdateContainerResources(ctx context.Context, containerID string, opts container.UpdateConfig) error {
	_, err := c.cli.ContainerUpdate(ctx, containerID, opts)
	return err
}

func (c *Client) PruneContainers(ctx context.Context) error {
	_, err := c.cli.ContainersPrune(ctx, filters.Args{})
	return err
}

func (c *Client) PruneImages(ctx context.Context) error {
	_, err := c.cli.ImagesPrune(ctx, filters.Args{})
	return err
}

func (c *Client) PruneVolumes(ctx context.Context) error {
	_, err := c.cli.VolumesPrune(ctx, filters.Args{})
	return err
}

func (c *Client) PruneNetworks(ctx context.Context) error {
	_, err := c.cli.NetworksPrune(ctx, filters.Args{})
	return err
}

// NetworkExists reports whether a network with the given name already
// exists.
func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
	_, err := c.cli.NetworkInspect(ctx, name, network.InspectOptions{})
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("inspect network: %w", err)
	}
	return true, nil
}

// EnsureNetwork creates a bridge network with the given name if it doesn't
// already exist. When internal is true, the network has no default route
// to the internet - containers on it can only reach other containers
// attached to the same network.
func (c *Client) EnsureNetwork(ctx context.Context, name string, internal bool) error {
	exists, err := c.NetworkExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = c.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver:   "bridge",
		Internal: internal,
	})
	if err != nil {
		return fmt.Errorf("create network: %w", err)
	}
	return nil
}

// NetworkConnect attaches a running container to a network. It is
// idempotent: connecting an already-attached container is not an error.
// aliases sets the container's network aliases on that network (see
// CreateContainerOpts's doc comment) - nil/empty leaves it alias-less,
// matching every pre-FEAT-028 caller (Traefik, the egress proxy).
func (c *Client) NetworkConnect(ctx context.Context, networkName, containerName string, aliases []string) error {
	var epSettings *network.EndpointSettings
	if len(aliases) > 0 {
		epSettings = &network.EndpointSettings{Aliases: aliases}
	}
	err := c.cli.NetworkConnect(ctx, networkName, containerName, epSettings)
	if err != nil && strings.Contains(err.Error(), "already exists") {
		return nil
	}
	return err
}

// ConnectNetworks attaches an already-created container to each of the
// given networks, on top of whatever network it was created on.
// CreateContainerOpts only joins one network at create time; a compose
// service can declare several (FEAT-026), so the rest are connected
// afterward here - same create-on-one, connect-the-rest shape as
// ensureEgressProxy's multi-network attach in agent_service.go.
// NetworkConnect is already idempotent, so re-connecting an already-joined
// network is not an error. aliases (same meaning as CreateContainerOpts's)
// is applied to every network in the list, so a service with more than one
// declared compose network still resolves by its bare service name on all
// of them, not just the one it was created on.
func (c *Client) ConnectNetworks(ctx context.Context, containerName string, networks []string, aliases []string) error {
	for _, n := range networks {
		if err := c.NetworkConnect(ctx, n, containerName, aliases); err != nil {
			return fmt.Errorf("connect network %s: %w", n, err)
		}
	}
	return nil
}

// NetworkDisconnect detaches a container from a network. Missing
// network/container is not an error - there's nothing left to disconnect.
func (c *Client) NetworkDisconnect(ctx context.Context, networkName, containerName string) error {
	err := c.cli.NetworkDisconnect(ctx, networkName, containerName, true)
	if err != nil && client.IsErrNotFound(err) {
		return nil
	}
	return err
}

// NetworkRemove removes a network. Missing network is not an error.
func (c *Client) NetworkRemove(ctx context.Context, name string) error {
	err := c.cli.NetworkRemove(ctx, name)
	if err != nil && client.IsErrNotFound(err) {
		return nil
	}
	return err
}

// FindContainerByComposeService returns the container name Docker Compose
// gave the container it created for a given compose service name (matched
// via the standard "com.docker.compose.service" label Compose sets on
// every container it creates), regardless of the compose project name
// Compose derived (which depends on the checkout directory name /
// COMPOSE_PROJECT_NAME and isn't otherwise known to this code - a
// hardcoded container name like "tamga-traefik-1" would be fragile).
// FEAT-028's deploy engine uses this to locate the running `traefik`
// container so it can be attached to a project's network - see
// project_service.go's deployStack.
func (c *Client) FindContainerByComposeService(ctx context.Context, serviceName string) (string, error) {
	args := filters.NewArgs(filters.Arg("label", "com.docker.compose.service="+serviceName))
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: args})
	if err != nil {
		return "", fmt.Errorf("list containers by compose service %q: %w", serviceName, err)
	}
	if len(containers) == 0 || len(containers[0].Names) == 0 {
		return "", fmt.Errorf("no container found for compose service %q", serviceName)
	}
	return strings.TrimPrefix(containers[0].Names[0], "/"), nil
}

// ContainerEnv returns the environment variables a running/existing
// container was created with.
func (c *Client) ContainerEnv(ctx context.Context, containerName string) ([]string, error) {
	info, err := c.cli.ContainerInspect(ctx, containerName)
	if err != nil {
		return nil, fmt.Errorf("inspect container: %w", err)
	}
	if info.Config == nil {
		return nil, nil
	}
	return info.Config.Env, nil
}

// ExecCreate creates a shell/PTY exec session inside a running container.
func (c *Client) ExecCreate(ctx context.Context, containerID string, cmd []string, workDir string) (string, error) {
	resp, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
		WorkingDir:   workDir,
		Env:          []string{"TERM=xterm-256color"},
	})
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}
	return resp.ID, nil
}

// ExecAttach attaches to an exec session, returning the hijacked stdio
// stream to proxy stdin/stdout over.
func (c *Client) ExecAttach(ctx context.Context, execID string) (types.HijackedResponse, error) {
	resp, err := c.cli.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{Tty: true})
	if err != nil {
		return types.HijackedResponse{}, fmt.Errorf("exec attach: %w", err)
	}
	return resp, nil
}

// ExecResize resizes the PTY behind an exec session.
func (c *Client) ExecResize(ctx context.Context, execID string, height, width uint) error {
	return c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{Height: height, Width: width})
}

// ExecRun runs cmd inside containerID to completion (no TTY) and discards
// its output, blocking until the process exits. Used for short,
// fire-and-forget maintenance commands (e.g. FEAT-015's terminal session
// kill, which signals one specific exec's shell process by PID) where
// nothing needs to be streamed back - just "run this and wait".
//
// CRITICAL FIX (FEAT-015 rework 2026-07-09): The original implementation
// called ContainerExecStart with no ExecStartOptions, which doesn't
// actually execute the command - it only marks it ready to start. The
// polling loop then polls ExecInspect indefinitely, never detecting the
// exec as running or finished (because it never started). Now we attach
// to the exec (which implicitly starts it with attached output streams),
// consume the output until EOF (which blocks until the command finishes),
// then return. This ensures the command actually runs and we wait for it.
//
// TIMEOUT FIX (2026-07-09, third review): io.Copy on a hijacked stream
// can block indefinitely if the Docker daemon hiccups, because the net.Conn
// returned by ContainerExecAttach has no relationship to ctx — no deadline
// is ever set on the underlying socket, so context cancellation alone won't
// interrupt an in-flight Read(). The fix: a watcher goroutine that closes
// the hijacked connection itself when the timeout fires, so the blocked Read
// actually unblocks with an error. After io.Copy returns, check execCtx.Err()
// and treat a deadline-exceeded timeout as a real error (not swallowable as EOF),
// since a timed-out kill means "we don't know if the shell actually died."
func (c *Client) ExecRun(ctx context.Context, containerID string, cmd []string) error {
	// Create a 5s timeout for the entire exec sequence (create, attach, run, drain)
	execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := c.cli.ContainerExecCreate(execCtx, containerID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		return fmt.Errorf("exec create: %w", err)
	}
	// Attach to the exec - this implicitly starts it and returns hijacked streams
	hijacked, err := c.cli.ContainerExecAttach(execCtx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("exec attach: %w", err)
	}
	defer hijacked.Close()

	// Watcher goroutine: if the 5s timeout fires, close the hijacked connection
	// to force-unblock the io.Copy below. The connection has no relationship to
	// execCtx (confirmed by reading Docker SDK hijack.go), so context cancellation
	// alone won't interrupt Read() — we must close the connection itself.
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		select {
		case <-execCtx.Done():
			hijacked.Close()
		case <-watchDone:
		}
	}()

	// Consume all output until EOF (which means the exec finished).
	// If execCtx times out, the watcher above closes the connection,
	// which causes this Read to error.
	_, err = io.Copy(io.Discard, hijacked.Reader)
	if err != nil && err != io.EOF {
		return fmt.Errorf("exec output read: %w", err)
	}

	// After the copy returns, check if the timeout fired. A timed-out kill
	// is a real error — it means we don't know whether the shell process
	// actually died, so we must surface it rather than silently returning nil.
	if execCtx.Err() != nil {
		return fmt.Errorf("kill command timed out: %w", execCtx.Err())
	}

	return nil
}

func (c *Client) DockerInfo(ctx context.Context) (system.Info, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return system.Info{}, fmt.Errorf("docker info: %w", err)
	}
	return info, nil
}
