package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/compose-spec/compose-go/v2/loader"
	"github.com/compose-spec/compose-go/v2/types"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

// composeProjectName is a fixed placeholder passed to compose-go's loader,
// which requires a non-empty project name to load anything at all (it's
// normally derived from a directory name or an explicit top-level `name:`
// field). Tamga has no equivalent concept - a compose stack is scoped by
// Tamga's own project ID, not by compose-go's project-name mechanism - so
// this value is never surfaced anywhere; it only satisfies the library's
// precondition.
const composeProjectName = "tamga-project"

// ParseComposeYAML parses a project's compose_yaml (FEAT-025's
// Project.ComposeYAML field) into the normalized service model FEAT-028's
// deploy engine consumes, using compose-go/v2 (TEST-011's recommendation -
// the same library `docker compose` itself uses) to handle the full
// grammar's short/long syntax variants so Tamga's own code never
// hand-parses YAML.
//
// Only the supported subset - image, ports, environment, volumes,
// networks, depends_on - is carried into the returned model. Compose
// features that would change runtime behavior but that Tamga's deploy
// engine doesn't implement (build:, profiles:, secrets:, healthcheck:) are
// rejected outright with a clear, actionable error rather than silently
// dropped. Every other field compose-go parses but this function doesn't
// map into domain.ComposeService (e.g. `hostname:`, `labels:`,
// `restart:`) is likewise not carried forward to FEAT-028, but those are
// metadata/tuning knobs with no correctness impact if ignored, unlike the
// four rejected above - see TEST-011 §2f and this task's Out of Scope.
//
// depends_on's long map form may include a `condition:` (e.g.
// service_healthy). Per Requirements the condition itself is out of scope
// (Tamga's deploy order is a plain topological sort over the dependency
// edges - service.TopoSortServices - not a health-check gate), so only the
// dependency edge (the target service name) is kept; the condition value
// is read by compose-go and discarded here, not rejected.
//
// The returned []domain.ComposeService's DependsOn is exactly the shape
// service.ComposeServiceDep needs: for each entry `cs`,
// `service.ComposeServiceDep{Name: cs.Name, DependsOn: cs.DependsOn}`
// converts with no further transformation, so FEAT-028 can feed a parsed
// stack straight into TopoSortServices before creating any containers.
func ParseComposeYAML(yamlContent string) ([]domain.ComposeService, error) {
	details := types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{Filename: "docker-compose.yml", Content: []byte(yamlContent)},
		},
		Environment: types.Mapping{},
	}

	project, err := loader.LoadWithContext(context.Background(), details, func(o *loader.Options) {
		// Tamga enforces the supported subset itself (see doc comment)
		// rather than relying on compose-go's own consistency rules, so
		// every rejection/validation error below comes from one place
		// with one consistent message style.
		o.SkipConsistencyCheck = true
		// "*" activates every profile so a profile-gated service still
		// appears in project.Services instead of being silently dropped
		// by compose-go before this function ever gets a chance to see
		// (and reject) its `profiles:` field.
		o.Profiles = []string{"*"}
		o.SetProjectName(composeProjectName, true)
	})
	if err != nil {
		return nil, fmt.Errorf("parse compose YAML: %w", err)
	}

	if len(project.Services) == 0 {
		return nil, fmt.Errorf("compose file declares no services")
	}

	names := make(map[string]struct{}, len(project.Services))
	serviceNames := make([]string, 0, len(project.Services))
	for name := range project.Services {
		names[name] = struct{}{}
		serviceNames = append(serviceNames, name)
	}
	sort.Strings(serviceNames) // deterministic output order

	result := make([]domain.ComposeService, 0, len(serviceNames))
	for _, name := range serviceNames {
		svc := project.Services[name]

		if err := rejectUnsupportedFeatures(name, svc); err != nil {
			return nil, err
		}
		if err := validateNoHostPortMappings(types.Services{name: svc}); err != nil {
			return nil, err
		}

		dependsOn := make([]string, 0, len(svc.DependsOn))
		for dep := range svc.DependsOn {
			dependsOn = append(dependsOn, dep)
		}
		sort.Strings(dependsOn)
		for _, dep := range dependsOn {
			if _, ok := names[dep]; !ok {
				return nil, fmt.Errorf("service %q depends_on undefined service %q", name, dep)
			}
		}

		result = append(result, domain.ComposeService{
			Name:        name,
			Image:       svc.Image,
			Ports:       normalizePorts(svc.Ports),
			Environment: normalizeEnvironment(svc.Environment),
			Volumes:     normalizeVolumes(svc.Volumes),
			Networks:    normalizeNetworks(svc.Networks),
			DependsOn:   dependsOn,
		})
	}

	return result, nil
}

// rejectUnsupportedFeatures returns a clear, actionable error if svc uses
// any compose feature outside the supported subset. Detection follows
// compose-go's own parsed types directly (non-nil/non-empty on the fields
// it populates for build:/profiles:/secrets:/healthcheck:), per this
// task's guidance - no hand-parsing of the raw YAML.
func rejectUnsupportedFeatures(name string, svc types.ServiceConfig) error {
	if svc.Build != nil {
		return fmt.Errorf("service %q: build is not supported yet; use a prebuilt image", name)
	}
	if len(svc.Profiles) > 0 {
		return fmt.Errorf("service %q: profiles is not supported yet; remove profiles and deploy the service unconditionally", name)
	}
	if len(svc.Secrets) > 0 {
		return fmt.Errorf("service %q: secrets is not supported yet; pass sensitive values via environment instead", name)
	}
	if svc.HealthCheck != nil {
		return fmt.Errorf("service %q: healthcheck is not supported yet; remove the healthcheck block", name)
	}
	return nil
}

// validateNoHostPortMappings rejects every compose syntax that publishes a
// container port on the host. compose-go has already normalized short and long
// forms into ServicePortConfig, so Published is the one fail-closed signal to
// check. A target-only entry remains valid: it documents a container port but
// creates no host listener.
func validateNoHostPortMappings(services types.Services) error {
	for name, svc := range services {
		for _, port := range svc.Ports {
			if port.Published != "" {
				return fmt.Errorf("service %q must not publish host ports", name)
			}
		}
	}
	return nil
}

// normalizePorts converts compose-go's already-normalized []ServicePortConfig
// (short "8080:80"/"8080:80/udp" and long mapping syntax alike, both
// already merged into one shape by compose-go's loader) into
// domain.ComposePort. Published is "" when the entry doesn't publish to
// the host (e.g. a bare "80" short entry) - it is a container-only port
// number in that case, still worth keeping so FEAT-028 knows the service
// listens on it.
func normalizePorts(ports []types.ServicePortConfig) []domain.ComposePort {
	if len(ports) == 0 {
		return nil
	}
	out := make([]domain.ComposePort, 0, len(ports))
	for _, p := range ports {
		out = append(out, domain.ComposePort{
			Published: p.Published,
			Target:    p.Target,
			Protocol:  p.Protocol,
		})
	}
	return out
}

// normalizeEnvironment converts compose-go's MappingWithEquals
// (map[string]*string, already merged from both the list form
// `["FOO=bar", "BAZ"]` and the map form `{FOO: bar, BAZ: null}` by
// compose-go's loader) into a plain map[string]string. A nil value - a
// name-only entry with no `=value` (e.g. bare "BAZ") - means "inherit from
// the environment compose runs in" under real docker compose semantics;
// Tamga has no such host-process environment to inherit from (and
// wouldn't want to leak the API server's own env into a user's container
// if it did), so a nil value normalizes to "" rather than being resolved
// against anything on the Tamga host.
func normalizeEnvironment(env types.MappingWithEquals) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		if v == nil {
			out[k] = ""
			continue
		}
		out[k] = *v
	}
	return out
}

// normalizeVolumes converts compose-go's already-normalized
// []ServiceVolumeConfig (short "/host:/container", "myvol:/data", bare
// "/data" anonymous, and long mapping syntax alike) into
// domain.ComposeVolume.
func normalizeVolumes(volumes []types.ServiceVolumeConfig) []domain.ComposeVolume {
	if len(volumes) == 0 {
		return nil
	}
	out := make([]domain.ComposeVolume, 0, len(volumes))
	for _, v := range volumes {
		out = append(out, domain.ComposeVolume{
			Type:     v.Type,
			Source:   v.Source,
			Target:   v.Target,
			ReadOnly: v.ReadOnly,
		})
	}
	return out
}

// normalizeNetworks returns the sorted list of network names a service
// attaches to (compose-go's already-normalized map, merged from both the
// short list form `["frontend", "backend"]` and the long mapping form).
// A service that declares no `networks:` at all still gets compose-go's
// implicit "default" network here, faithfully reflecting real compose
// semantics.
func normalizeNetworks(networks map[string]*types.ServiceNetworkConfig) []string {
	if len(networks) == 0 {
		return nil
	}
	out := make([]string, 0, len(networks))
	for name := range networks {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
