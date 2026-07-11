package domain

// ComposeService is one service parsed from a project's compose_yaml
// (FEAT-027), normalized from whichever short/long compose syntax the user
// wrote it in. This is the exact contract FEAT-028's deploy engine consumes
// to create containers - the seam between "parse a compose file" and
// "stand up containers from it".
//
// It is a parse-time value only, never persisted directly. Once a service
// is actually deployed, its running state is tracked separately in
// ServiceContainer (one row per running container); ComposeService is
// recomputed from Project.ComposeYAML on demand (e.g. on every deploy),
// never stored on its own.
//
// DependsOn is deliberately just service names, matching
// service.ComposeServiceDep's {Name, DependsOn []string} shape one field at
// a time - a ComposeService converts to a ComposeServiceDep with no
// transformation needed (`service.ComposeServiceDep{Name: cs.Name,
// DependsOn: cs.DependsOn}`), so FEAT-028 can feed a parsed stack straight
// into service.TopoSortServices before creating any containers.
type ComposeService struct {
	Name        string
	Image       string
	Ports       []ComposePort
	Environment map[string]string
	Volumes     []ComposeVolume
	Networks    []string
	DependsOn   []string
}

// ComposePort is one normalized `ports:` entry, covering both the short
// string syntax (e.g. "8080:80", "8080:80/udp") and the long mapping
// syntax. Published is the host-side port ("" if the entry only exposes a
// container port without publishing it to the host, e.g. a bare "80" short
// entry). Target is always the container-side port. Protocol is "tcp" or
// "udp" (compose-go defaults it to "tcp" when unspecified).
type ComposePort struct {
	Published string
	Target    uint32
	Protocol  string
}

// ComposeVolume is one normalized `volumes:` entry, covering both the short
// string syntax (e.g. "/host:/container", "myvol:/data", or a bare
// "/data" anonymous volume) and the long mapping syntax. Type is "bind",
// "volume", or "tmpfs" (compose-go's ServiceVolumeConfig.Type). Source is
// the host path or named volume ("" for an anonymous volume).
type ComposeVolume struct {
	Type     string
	Source   string
	Target   string
	ReadOnly bool
}
