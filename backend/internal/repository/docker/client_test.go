package docker

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/, following project_service_test.go's precedent: it
// exercises containerProjectInfo, an unexported pure function, directly -
// no Docker daemon or DB involved, so it belongs next to the code it
// tests rather than in the daemon-gated internal/tests/repository suite
// (docker_client_test.go).

import "testing"

// TestContainerSealInfo locks in Seal ownership attribution for multi-service
// compose container names ("seal-<id>-<service>") and agent sandboxes.
func TestContainerSealInfo(t *testing.T) {
	tests := []struct {
		name           string
		wantSealID     int64
		wantSystemType string
	}{
		{name: "seal-1-web", wantSealID: 1, wantSystemType: ""},
		{name: "seal-12-database", wantSealID: 12, wantSystemType: ""},
		{name: "seal-7", wantSealID: 7, wantSystemType: ""},
		{name: "agent-3", wantSealID: 3, wantSystemType: ""},
		{name: "agent-system", wantSealID: 0, wantSystemType: "agent-system"},
		{name: "caddy", wantSealID: 0, wantSystemType: "caddy"},
		{name: "tamga-backend", wantSealID: 0, wantSystemType: "tamga-backend"},
		{name: "unrelated-container", wantSealID: 0, wantSystemType: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSealID, gotSystemType := containerSealInfo(tt.name)
			if gotSealID != tt.wantSealID {
				t.Errorf("containerSealInfo(%q) sealID = %d, want %d", tt.name, gotSealID, tt.wantSealID)
			}
			if gotSystemType != tt.wantSystemType {
				t.Errorf("containerSealInfo(%q) systemType = %q, want %q", tt.name, gotSystemType, tt.wantSystemType)
			}
		})
	}
}
