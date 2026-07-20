package service

import (
	"fmt"
	"sort"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// This file holds FEAT-028's pure, no-I/O deploy-engine logic: the bits
// TopoSortServices (compose_order.go) and ParseComposeYAML
// (compose_parser.go) already established the pattern for - small,
// independently unit-testable functions the Docker-touching orchestration
// in project_service.go composes together. Nothing in this file talks to
// Docker, the DB, or Traefik.

// gitBuildServiceName is the single synthesized service name a legacy
// git-build project's already-built image folds into (TEST-011 §2a's
// "1-service compose"). Not user-visible - there's only ever one service
// for these projects - it just needs to be stable and non-empty.
const gitBuildServiceName = "app"

// projectNetworkName returns the per-project Docker network name a project's
// whole compose stack joins (closes BUG-029 - see
// project_service.go's deployStack doc comment for the full design).
// Mirrors agent_service.go's agentNetworkName(id) ("agent-net-<id>")
// one-for-one, just for project stacks instead of agent sandboxes.
func projectNetworkName(projectID int64) string {
	return fmt.Sprintf("project-net-%d", projectID)
}

// serviceContainerName returns the container name FEAT-028's deploy engine
// gives one compose service belonging to a project. Every service -
// including the exposed one - uses this same uniform shape: the Traefik
// route/service *name* (not the container name) is what C1's metric
// attribution depends on staying "project-<id>"
// (repository/traefik.Client.AddRoute always names its router/service
// "project-<id>" regardless of what upstream host:port it's given), so
// there is no need to special-case the exposed container's own name.
func serviceContainerName(projectID int64, serviceName string) string {
	return fmt.Sprintf("project-%d-%s", projectID, serviceName)
}

// synthesizeGitBuildService folds a legacy git-build project's
// already-built image into a single domain.ComposeService, so
// ProjectService.deployStack has exactly ONE deploy path for both real
// compose projects and folded-git-build projects (TEST-011 §2a). tag is
// the already-built image (project_service.go's "tamga-project-<id>"
// convention, unchanged).
//
// No Ports are set - the synthesized service never declares one, since a
// legacy git-build project's exposed port has always come from
// post-create image introspection (GetContainerPort), never explicit
// config. This is safe because it's always the sole member of the
// service list passed to detectExposedService, whose single-service rule
// picks it as the exposed service unconditionally, with no port needed to
// disambiguate against anything else.
func synthesizeGitBuildService(tag string) domain.ComposeService {
	return domain.ComposeService{
		Name:  gitBuildServiceName,
		Image: tag,
	}
}

// applyDatabaseEnvironment makes the database the only deployment-time
// environment source. Compose values are imported once when configuration is
// first accepted, then deliberately ignored here so later YAML edits cannot
// override DB-owned values.
func applyDatabaseEnvironment(services []domain.ComposeService, serviceNames map[int64]string, scoped []*domain.ServiceEnvVar) []domain.ComposeService {
	byService := make(map[string]map[string]string)
	for _, value := range scoped {
		name, ok := serviceNames[value.ServiceID]
		if !ok {
			continue
		}
		if byService[name] == nil {
			byService[name] = make(map[string]string)
		}
		byService[name][value.Key] = value.Value
	}
	for i := range services {
		env := make(map[string]string, len(byService[services[i].Name]))
		for key, value := range byService[services[i].Name] {
			env[key] = value
		}
		if len(env) == 0 {
			services[i].Environment = nil
		} else {
			services[i].Environment = env
		}
	}
	return services
}

// envMapToSlice converts a compose service's normalized environment map
// (domain.ComposeService.Environment - whether parsed from real compose
// YAML or produced by synthesizeGitBuildService) into the "KEY=VALUE"
// slice docker/client.go's CreateContainerOpts expects. Sorted by key so
// container Env order is deterministic across deploys/restarts (map
// iteration order is not).
func envMapToSlice(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(env))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s=%s", k, env[k]))
	}
	return out
}

// composeVolumesToMounts converts a compose service's normalized volumes
// into the "source:target[:ro]" slice CreateContainerOpts's mounts
// parameter already accepts (TEST-011 §2b: already fully supported,
// HostConfig.Binds natively accepts both bind-mount and named-volume
// source strings in this exact shape). Anonymous volumes (bare "/data",
// no source) are skipped rather than emitting a malformed mount - they
// weren't called out in TEST-011 §2b's supported subset.
func composeVolumesToMounts(volumes []domain.ComposeVolume) []string {
	if len(volumes) == 0 {
		return nil
	}
	out := make([]string, 0, len(volumes))
	for _, v := range volumes {
		if v.Source == "" || v.Target == "" {
			continue
		}
		m := v.Source + ":" + v.Target
		if v.ReadOnly {
			m += ":ro"
		}
		out = append(out, m)
	}
	return out
}

// exposedTargetPort returns the container-side port a service explicitly
// declares (its first Ports entry's Target), formatted for use as
// Traefik's upstream port. "" means the service declared no port at all -
// the caller falls back to GetContainerPort's post-create inspection,
// exactly the pre-FEAT-028 fallback behavior (always used by the folded
// git-build case, which never declares a port up front).
func exposedTargetPort(svc domain.ComposeService) string {
	if len(svc.Ports) == 0 {
		return ""
	}
	return fmt.Sprintf("%d", svc.Ports[0].Target)
}

// detectExposedService picks the compose service that should receive the
// project's Traefik route (TEST-011 §2c's heuristic):
//
//  1. override (project.ExposedService) always wins when non-empty - an
//     explicit user choice is trusted as-is, regardless of what that
//     service does or doesn't declare.
//  2. A single-service stack is always exposed - there's no other
//     candidate to disambiguate against. This is what makes the folded
//     git-build case (§2a, which never declares a port up front) resolve
//     correctly without a special case: "did it declare a port" alone
//     would otherwise wrongly exclude it.
//  3. Otherwise (a genuine multi-service stack, no override): exactly one
//     service declaring a port is the default. Zero or more than one
//     declaring a port is ambiguous - no safe default exists, and the
//     caller must not create a route (ok=false).
func detectExposedService(services []domain.ComposeService, override string) (name string, ok bool) {
	if override != "" {
		return override, true
	}
	if len(services) == 1 {
		return services[0].Name, true
	}
	var withPort string
	count := 0
	for _, s := range services {
		if len(s.Ports) > 0 {
			withPort = s.Name
			count++
		}
	}
	if count == 1 {
		return withPort, true
	}
	return "", false
}

// aggregateStatus derives a whole-stack domain.ProjectStatus from every
// persisted service-container's live running state (TEST-011 §2d): all
// running is Running; any not running - including the empty case, no
// containers deployed at all - is Error. Deliberately coarse, no
// partial/degraded state: a project owner needing to know *which* service
// is down already has the existing per-container inspect/logs UI; this is
// only the project-list-view's one-line status.
func aggregateStatus(running []bool) domain.ProjectStatus {
	if len(running) == 0 {
		return domain.ProjectStatusError
	}
	for _, r := range running {
		if !r {
			return domain.ProjectStatusError
		}
	}
	return domain.ProjectStatusRunning
}

// toComposeServiceDeps converts a parsed/synthesized service list into the
// minimal {Name, DependsOn} shape service.TopoSortServices needs - a
// direct field-for-field conversion, no transformation, per
// domain.ComposeService's own doc comment.
func toComposeServiceDeps(services []domain.ComposeService) []ComposeServiceDep {
	deps := make([]ComposeServiceDep, 0, len(services))
	for _, s := range services {
		deps = append(deps, ComposeServiceDep{Name: s.Name, DependsOn: s.DependsOn})
	}
	return deps
}
