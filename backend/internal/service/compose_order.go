package service

import "fmt"

// ComposeServiceDep describes one compose service's name and the names of
// the services it declares in depends_on. It is deliberately minimal - just
// enough for topological ordering - so this file has no dependency on
// FEAT-025's compose schema or FEAT-027's parser; it only needs names and
// edges.
type ComposeServiceDep struct {
	Name      string
	DependsOn []string
}

// TopoSortServices returns a valid container start order for services
// given their depends_on edges (a dependency is always ordered before
// whatever depends on it), using Kahn's algorithm. It is a pure function -
// no I/O, no Docker - so FEAT-028's deploy engine can call it before
// touching the daemon at all, and it is trivially unit-testable on its own.
//
// Ordering is deterministic: services with no unresolved dependencies are
// started in the order they appear in the input slice, ties included, so
// the same input always produces the same output.
//
// Two error cases:
//   - A service's depends_on names a service that isn't in the input set at
//     all. This is treated as an error (not silently ignored) because an
//     undefined dependency is invalid compose config - failing fast here
//     surfaces the mistake before FEAT-028 tries to start anything, rather
//     than the deploy silently starting services in an order that doesn't
//     honor a dependency the author clearly intended.
//   - A cycle among depends_on edges (e.g. A depends on B, B depends on A).
//     There is no valid start order for a cycle, so this returns an error
//     naming the services still stuck with unresolved dependencies.
func TopoSortServices(services []ComposeServiceDep) ([]string, error) {
	index := make(map[string]int, len(services))
	for i, s := range services {
		if _, dup := index[s.Name]; dup {
			return nil, fmt.Errorf("duplicate service name %q", s.Name)
		}
		index[s.Name] = i
	}

	inDegree := make([]int, len(services))
	dependents := make([][]int, len(services)) // dependents[i] = indices depending on services[i]

	for i, s := range services {
		for _, dep := range s.DependsOn {
			depIdx, ok := index[dep]
			if !ok {
				return nil, fmt.Errorf("service %q depends_on undefined service %q", s.Name, dep)
			}
			dependents[depIdx] = append(dependents[depIdx], i)
			inDegree[i]++
		}
	}

	queue := make([]int, 0, len(services))
	for i, d := range inDegree {
		if d == 0 {
			queue = append(queue, i)
		}
	}

	order := make([]string, 0, len(services))
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		order = append(order, services[i].Name)
		for _, dep := range dependents[i] {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(order) != len(services) {
		var stuck []string
		for i, d := range inDegree {
			if d > 0 {
				stuck = append(stuck, services[i].Name)
			}
		}
		return nil, fmt.Errorf("circular depends_on among services: %v", stuck)
	}

	return order, nil
}
