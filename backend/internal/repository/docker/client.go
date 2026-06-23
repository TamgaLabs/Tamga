package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
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
	resp, err := c.cli.ContainerCreate(ctx, &container.Config{
		Image: imageName,
		Env:   env,
	}, &container.HostConfig{
		NetworkMode:   container.NetworkMode(network),
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
	}, nil, nil, name)
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
	resp, err := c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
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
