package service_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// index maps each service name to its position in order, so tests can
// assert "before" relationships without depending on exact ordering where
// multiple orders are equally valid (e.g. sibling branches of a diamond).
func index(order []string) map[string]int {
	idx := make(map[string]int, len(order))
	for i, name := range order {
		idx[name] = i
	}
	return idx
}

func assertBefore(t *testing.T, order []string, first, second string) {
	t.Helper()
	idx := index(order)
	fi, ok := idx[first]
	if !ok {
		t.Fatalf("order %v missing %q", order, first)
	}
	si, ok := idx[second]
	if !ok {
		t.Fatalf("order %v missing %q", order, second)
	}
	if fi >= si {
		t.Errorf("expected %q before %q, got order %v", first, second, order)
	}
}

func TestTopoSortServicesNoDeps(t *testing.T) {
	services := []service.ComposeServiceDep{
		{Name: "web"},
		{Name: "worker"},
		{Name: "cache"},
	}
	order, err := service.TopoSortServices(services)
	if err != nil {
		t.Fatalf("TopoSortServices: %v", err)
	}
	// No deps at all: input order is preserved (deterministic).
	want := []string{"web", "worker", "cache"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestTopoSortServicesEmpty(t *testing.T) {
	order, err := service.TopoSortServices(nil)
	if err != nil {
		t.Fatalf("TopoSortServices: %v", err)
	}
	if len(order) != 0 {
		t.Errorf("order = %v, want empty", order)
	}
}

func TestTopoSortServicesLinearChain(t *testing.T) {
	// db <- api <- web (web depends_on api, api depends_on db)
	services := []service.ComposeServiceDep{
		{Name: "web", DependsOn: []string{"api"}},
		{Name: "api", DependsOn: []string{"db"}},
		{Name: "db"},
	}
	order, err := service.TopoSortServices(services)
	if err != nil {
		t.Fatalf("TopoSortServices: %v", err)
	}
	want := []string{"db", "api", "web"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("order = %v, want %v", order, want)
	}
}

func TestTopoSortServicesDiamond(t *testing.T) {
	// base <- (left, right) <- top
	services := []service.ComposeServiceDep{
		{Name: "base"},
		{Name: "left", DependsOn: []string{"base"}},
		{Name: "right", DependsOn: []string{"base"}},
		{Name: "top", DependsOn: []string{"left", "right"}},
	}
	order, err := service.TopoSortServices(services)
	if err != nil {
		t.Fatalf("TopoSortServices: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("order = %v, want 4 entries", order)
	}
	assertBefore(t, order, "base", "left")
	assertBefore(t, order, "base", "right")
	assertBefore(t, order, "left", "top")
	assertBefore(t, order, "right", "top")
	// Deterministic tie-break: left appears before right in the input, and
	// both become ready at the same time (right after base), so left must
	// stay ahead of right in the output too.
	assertBefore(t, order, "left", "right")
}

func TestTopoSortServicesCycleErrors(t *testing.T) {
	// a -> b -> c -> a
	services := []service.ComposeServiceDep{
		{Name: "a", DependsOn: []string{"c"}},
		{Name: "b", DependsOn: []string{"a"}},
		{Name: "c", DependsOn: []string{"b"}},
	}
	order, err := service.TopoSortServices(services)
	if err == nil {
		t.Fatalf("expected cycle error, got order %v", order)
	}
	if !strings.Contains(err.Error(), "circular") {
		t.Errorf("error = %q, want it to mention a cycle", err.Error())
	}
}

func TestTopoSortServicesSelfCycleErrors(t *testing.T) {
	services := []service.ComposeServiceDep{
		{Name: "a", DependsOn: []string{"a"}},
	}
	if _, err := service.TopoSortServices(services); err == nil {
		t.Fatalf("expected cycle error for self-dependency, got nil")
	}
}

func TestTopoSortServicesMissingDependencyErrors(t *testing.T) {
	// "web" depends on "db", which is never declared - this is invalid
	// compose config (a typo'd or removed service), so it must error
	// rather than silently starting "web" as if it had no dependencies.
	services := []service.ComposeServiceDep{
		{Name: "web", DependsOn: []string{"db"}},
	}
	order, err := service.TopoSortServices(services)
	if err == nil {
		t.Fatalf("expected undefined-dependency error, got order %v", order)
	}
	if !strings.Contains(err.Error(), "undefined service") {
		t.Errorf("error = %q, want it to mention the undefined service", err.Error())
	}
}

func TestTopoSortServicesDuplicateNameErrors(t *testing.T) {
	services := []service.ComposeServiceDep{
		{Name: "web"},
		{Name: "web"},
	}
	if _, err := service.TopoSortServices(services); err == nil {
		t.Fatalf("expected duplicate-name error, got nil")
	}
}
