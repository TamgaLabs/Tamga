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

func (c *Client) CreateContainer(ctx context.Context, name, imageName string, env []string, network string) (string, error) {
	return c.CreateContainerOpts(ctx, name, imageName, env, network, nil)
}

func (c *Client) CreateContainerOpts(ctx context.Context, name, imageName string, env []string, network string, mounts []string) (string, error) {
	hostCfg := &container.HostConfig{
		NetworkMode:   container.NetworkMode(network),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
	}
	for _, m := range mounts {
		parts := strings.SplitN(m, ":", 2)
		if len(parts) == 2 {
			hostCfg.Binds = append(hostCfg.Binds, m)
		}
	}
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Env:   env,
	}, hostCfg, nil, nil, name)
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

		var projectID int64
		systemType := ""
		if strings.HasPrefix(name, "project-") {
			fmt.Sscanf(name, "project-%d", &projectID)
		} else if strings.HasPrefix(name, "agent-") {
			// agent containers: check if it's system or project
			if name == "agent-system" {
				systemType = "agent-system"
			} else {
				fmt.Sscanf(name, "agent-%d", &projectID)
			}
		} else if name == "caddy" || strings.HasPrefix(name, "tamga-") {
			systemType = name
		}

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

func (c *Client) DockerInfo(ctx context.Context) (system.Info, error) {
	info, err := c.cli.Info(ctx)
	if err != nil {
		return system.Info{}, fmt.Errorf("docker info: %w", err)
	}
	return info, nil
}
