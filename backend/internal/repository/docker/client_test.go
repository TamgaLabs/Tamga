package docker

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/, following project_service_test.go's precedent: it
// exercises containerProjectInfo, an unexported pure function, directly -
// no Docker daemon or DB involved, so it belongs next to the code it
// tests rather than in the daemon-gated internal/tests/repository suite
// (docker_client_test.go).

import "testing"

// TestContainerProjectInfo locks in project ownership attribution for
// multi-service compose container names and agent sandboxes.
func TestContainerProjectInfo(t *testing.T) {
	tests := []struct {
		name           string
		wantProjectID  int64
		wantSystemType string
	}{
		{name: "project-1-web", wantProjectID: 1, wantSystemType: ""},
		{name: "project-12-database", wantProjectID: 12, wantSystemType: ""},
		{name: "project-7", wantProjectID: 7, wantSystemType: ""},
		{name: "agent-3", wantProjectID: 3, wantSystemType: ""},
		{name: "agent-system", wantProjectID: 0, wantSystemType: "agent-system"},
		{name: "caddy", wantProjectID: 0, wantSystemType: "caddy"},
		{name: "tamga-backend", wantProjectID: 0, wantSystemType: "tamga-backend"},
		{name: "seal-9-web", wantProjectID: 0, wantSystemType: ""},
		{name: "unrelated-container", wantProjectID: 0, wantSystemType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotProjectID, gotSystemType := containerProjectInfo(tt.name)
			if gotProjectID != tt.wantProjectID {
				t.Errorf("containerProjectInfo(%q) projectID = %d, want %d", tt.name, gotProjectID, tt.wantProjectID)
			}
			if gotSystemType != tt.wantSystemType {
				t.Errorf("containerProjectInfo(%q) systemType = %q, want %q", tt.name, gotSystemType, tt.wantSystemType)
			}
		})
	}
}
