package docker

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/, following project_service_test.go's precedent: it
// exercises containerProjectInfo, an unexported pure function, directly -
// no Docker daemon or DB involved, so it belongs next to the code it
// tests rather than in the daemon-gated internal/tests/repository suite
// (docker_client_test.go).

import "testing"

// TestContainerProjectInfo locks in FEAT-029's finding that
// containerProjectInfo already correctly attributes a multi-service
// compose project's container names ("project-<id>-<service>",
// deploy_engine.go's serviceContainerName) back to their project ID - the
// concern flagged in FEAT-029's task write-up as a potential defect
// (fmt.Sscanf's "%d" stops at the first non-digit, so it never needed to
// consume the "-<service>" suffix in the first place). Also covers the
// legacy single "project-<id>" shape and the other two naming families
// ListContainers recognizes, so a future change to any of them gets
// caught here instead of only failing at the Docker-integration layer.
func TestContainerProjectInfo(t *testing.T) {
	tests := []struct {
		name           string
		wantProjectID  int64
		wantSystemType string
	}{
		{name: "project-1-web", wantProjectID: 1, wantSystemType: ""},
		{name: "project-12-database", wantProjectID: 12, wantSystemType: ""},
		{name: "project-7", wantProjectID: 7, wantSystemType: ""}, // legacy pre-FEAT-028 shape
		{name: "agent-3", wantProjectID: 3, wantSystemType: ""},
		{name: "agent-system", wantProjectID: 0, wantSystemType: "agent-system"},
		{name: "caddy", wantProjectID: 0, wantSystemType: "caddy"},
		{name: "tamga-backend", wantProjectID: 0, wantSystemType: "tamga-backend"},
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
