package repository_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"

	dockerclient "github.com/TamgaLabs/Tamga/backend/internal/repository/docker"
)

// newTestDockerClient returns a Docker client backed by the daemon
// available in this environment, skipping the test if none is reachable -
// same gating pattern as agent_service_test.go's newTestAgentService, so
// this test degrades gracefully somewhere Docker isn't available (e.g.
// plain CI without docker-in-docker) instead of failing outright.
func newTestDockerClient(t *testing.T) *dockerclient.Client {
	t.Helper()
	docker, err := dockerclient.New()
	if err != nil {
		t.Skipf("docker client not available: %v", err)
	}
	if _, err := docker.DockerInfo(context.Background()); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}
	return docker
}

// TestDockerClientPullImageAndConnectNetworks covers FEAT-026's two
// Docker-dependent primitives together: PullImage brings a public image
// down to completion, and ConnectNetworks attaches an already-created
// container to networks beyond the one it was created on. A tiny public
// image (alpine) and two throwaway bridge networks are used so this test
// doesn't touch anything the live stack depends on; everything it creates
// is torn down at the end regardless of outcome.
func TestDockerClientPullImageAndConnectNetworks(t *testing.T) {
	docker := newTestDockerClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const image = "alpine:3.21"
	if err := docker.PullImage(ctx, image); err != nil {
		t.Fatalf("PullImage(%s): %v", image, err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	netA := "tamga-test-net-a-" + suffix
	netB := "tamga-test-net-b-" + suffix
	containerName := "tamga-test-container-" + suffix

	if err := docker.EnsureNetwork(ctx, netA, true); err != nil {
		t.Fatalf("EnsureNetwork(%s): %v", netA, err)
	}
	t.Cleanup(func() { docker.NetworkRemove(context.Background(), netA) })

	if err := docker.EnsureNetwork(ctx, netB, true); err != nil {
		t.Fatalf("EnsureNetwork(%s): %v", netB, err)
	}
	t.Cleanup(func() { docker.NetworkRemove(context.Background(), netB) })

	containerID, err := docker.CreateContainerOpts(ctx, containerName, image, nil, netA, nil, container.Resources{}, false, nil)
	if err != nil {
		t.Fatalf("CreateContainerOpts: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		docker.StopContainer(cleanupCtx, containerID)
		docker.RemoveContainer(cleanupCtx, containerID)
	})

	// Created on netA only so far - attach netB as the "additional network"
	// a compose service with more than one declared network would need.
	if err := docker.ConnectNetworks(ctx, containerName, []string{netB}, nil); err != nil {
		t.Fatalf("ConnectNetworks: %v", err)
	}

	info, err := docker.InspectContainer(ctx, containerID)
	if err != nil {
		t.Fatalf("InspectContainer: %v", err)
	}
	if info.NetworkSettings == nil {
		t.Fatalf("inspect returned no NetworkSettings")
	}
	if _, ok := info.NetworkSettings.Networks[netA]; !ok {
		t.Errorf("container not attached to %s (created-on network); attached networks: %+v", netA, info.NetworkSettings.Networks)
	}
	if _, ok := info.NetworkSettings.Networks[netB]; !ok {
		t.Errorf("container not attached to %s (ConnectNetworks-attached network); attached networks: %+v", netB, info.NetworkSettings.Networks)
	}

	// ConnectNetworks is idempotent (it wraps the already-idempotent
	// NetworkConnect) - reconnecting an already-joined network must not
	// error.
	if err := docker.ConnectNetworks(ctx, containerName, []string{netB}, nil); err != nil {
		t.Errorf("ConnectNetworks (repeat, should be idempotent): %v", err)
	}
}

// TestDockerClientServiceAliasDNSResolution is TEST-014's FAIL, closed:
// creates two containers on a throwaway bridge network the way
// deployStack does - one ("project-<suffix>-redis") created with alias
// "redis" via CreateContainerOpts, mirroring a compose service container -
// then confirms a second container on the same network can resolve and
// reach the FIRST one by its BARE alias ("redis"), not just its full
// container name. This is exactly the reachability TEST-014 found broken
// (nslookup redis -> NXDOMAIN) before CreateContainerOpts grew an aliases
// parameter.
func TestDockerClientServiceAliasDNSResolution(t *testing.T) {
	docker := newTestDockerClient(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// redis:7-alpine for both containers (not bare alpine): its
	// entrypoint stays running in the foreground by default, so there's
	// still a live process to exec into a moment after start. Bare
	// alpine has no CMD, exits immediately, and - combined with
	// CreateContainerOpts's unless-stopped restart policy - ends up
	// stuck cycling through "restarting" instead of "running", which
	// ContainerExecCreate rejects.
	const image = "redis:7-alpine"
	if err := docker.PullImage(ctx, image); err != nil {
		t.Fatalf("PullImage(%s): %v", image, err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	netName := "tamga-test-alias-net-" + suffix
	redisName := "tamga-test-alias-redis-" + suffix
	webName := "tamga-test-alias-web-" + suffix

	if err := docker.EnsureNetwork(ctx, netName, true); err != nil {
		t.Fatalf("EnsureNetwork(%s): %v", netName, err)
	}
	t.Cleanup(func() { docker.NetworkRemove(context.Background(), netName) })

	// The "redis" service container - created with alias "redis", exactly
	// what deployStack now passes as svc.Name.
	redisID, err := docker.CreateContainerOpts(ctx, redisName, image, []string{}, netName, nil, container.Resources{}, false, []string{"redis"})
	if err != nil {
		t.Fatalf("CreateContainerOpts(redis): %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		docker.StopContainer(cleanupCtx, redisID)
		docker.RemoveContainer(cleanupCtx, redisID)
	})
	if err := docker.StartContainer(ctx, redisID); err != nil {
		t.Fatalf("StartContainer(redis): %v", err)
	}

	// The "web" peer container - no alias needed, it's the one doing the
	// resolving, matching a compose service that never gets addressed by
	// its own alias.
	webID, err := docker.CreateContainerOpts(ctx, webName, image, nil, netName, nil, container.Resources{}, false, nil)
	if err != nil {
		t.Fatalf("CreateContainerOpts(web): %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		docker.StopContainer(cleanupCtx, webID)
		docker.RemoveContainer(cleanupCtx, webID)
	})
	if err := docker.StartContainer(ctx, webID); err != nil {
		t.Fatalf("StartContainer(web): %v", err)
	}

	// getent hosts uses the container's embedded DNS resolver, same path a
	// real app connecting to "redis:6379" would take - this is the exact
	// check TEST-014 ran (nslookup) and found NXDOMAIN on.
	execID, err := docker.ExecCreate(ctx, webID, []string{"getent", "hosts", "redis"}, "")
	if err != nil {
		t.Fatalf("ExecCreate: %v", err)
	}
	hijacked, err := docker.ExecAttach(ctx, execID)
	if err != nil {
		t.Fatalf("ExecAttach: %v", err)
	}
	defer hijacked.Close()

	output, err := io.ReadAll(hijacked.Reader)
	if err != nil {
		t.Fatalf("read exec output: %v", err)
	}
	if !strings.Contains(string(output), "redis") {
		t.Errorf("bare service-name alias %q did not resolve from peer container; getent output: %q", "redis", string(output))
	}
}
