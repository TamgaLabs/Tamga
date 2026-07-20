package sqlite_test

import (
	"strings"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
)

func TestAuxiliaryRepositoriesUseProjectServiceOwnership(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec(`INSERT INTO services (project_id, name, internal_port) VALUES (1, 'web', 8080), (2, 'web', 8080)`); err != nil {
		t.Fatalf("seed service ownership: %v", err)
	}
	var projectOneServiceID, projectTwoServiceID int64
	if err := db.QueryRow("SELECT id FROM services WHERE project_id = 1 AND name = 'web'").Scan(&projectOneServiceID); err != nil {
		t.Fatalf("find project one service: %v", err)
	}
	if err := db.QueryRow("SELECT id FROM services WHERE project_id = 2 AND name = 'web'").Scan(&projectTwoServiceID); err != nil {
		t.Fatalf("find project two service: %v", err)
	}

	if err := db.ReplaceServiceContainers(1, []*domain.ServiceContainer{{
		ServiceID: projectOneServiceID, ContainerID: "project-one-web", ContainerName: "project-one-web", Status: "running",
	}}); err != nil {
		t.Fatalf("replace project one service containers: %v", err)
	}
	containers, err := db.ListServiceContainers(1)
	if err != nil {
		t.Fatalf("list project one service containers: %v", err)
	}
	if len(containers) != 1 || containers[0].ServiceID != projectOneServiceID {
		t.Fatalf("unexpected project one containers: %+v", containers)
	}
	otherContainers, err := db.ListServiceContainers(2)
	if err != nil {
		t.Fatalf("list project two service containers: %v", err)
	}
	if len(otherContainers) != 0 {
		t.Fatalf("project two received project one's container: %+v", otherContainers)
	}
	if err := db.ReplaceServiceContainers(1, []*domain.ServiceContainer{{ServiceID: projectTwoServiceID, ContainerID: "missing"}}); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("replace unknown owned service error = %v, want ownership error", err)
	}

	env := &domain.ServiceEnvVar{ServiceID: projectOneServiceID, Key: "API_KEY", Value: "one"}
	if err := db.UpsertServiceEnvVar(1, env); err != nil {
		t.Fatalf("upsert project one service env var: %v", err)
	}
	if env.ID == 0 {
		t.Fatal("upserted service env var has no ID")
	}
	if err := db.UpsertServiceEnvVar(2, &domain.ServiceEnvVar{
		ID: 99, ServiceID: 999, Key: "API_KEY", Value: "two",
	}); err == nil {
		t.Fatal("upsert missing service with a preassigned ID unexpectedly succeeded")
	}
	projectOneEnv, err := db.ListServiceEnvVars(1, projectOneServiceID)
	if err != nil {
		t.Fatalf("list project one service env vars: %v", err)
	}
	if len(projectOneEnv) != 1 || projectOneEnv[0].Value != "one" {
		t.Fatalf("unexpected project one service env vars: %+v", projectOneEnv)
	}
	projectTwoEnv, err := db.ListServiceEnvVars(2, projectTwoServiceID)
	if err != nil {
		t.Fatalf("list project two service env vars: %v", err)
	}
	if len(projectTwoEnv) != 0 {
		t.Fatalf("project two received project one's env var: %+v", projectTwoEnv)
	}
	if err := db.DeleteServiceEnvVar(2, projectTwoServiceID, env.ID); err != nil {
		t.Fatalf("cross-project delete must be harmless: %v", err)
	}
	projectOneEnv, err = db.ListServiceEnvVars(1, projectOneServiceID)
	if err != nil || len(projectOneEnv) != 1 {
		t.Fatalf("cross-project delete changed project one env vars: %+v, %v", projectOneEnv, err)
	}

	deployment := &domain.Deployment{ProjectID: 1, Status: domain.DeploymentStatusSuccess}
	if err := db.CreateDeployment(deployment); err != nil {
		t.Fatalf("create project deployment: %v", err)
	}
	deployments, err := db.ListDeployments(1)
	if err != nil {
		t.Fatalf("list project deployments: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ProjectID != 1 {
		t.Fatalf("unexpected project deployments: %+v", deployments)
	}
	otherDeployment := &domain.Deployment{ProjectID: 2, Status: domain.DeploymentStatusPending}
	if err := db.CreateDeployment(otherDeployment); err != nil {
		t.Fatalf("create project two deployment: %v", err)
	}
	if err := db.DeleteDeploymentsByProject(1); err != nil {
		t.Fatalf("delete project one deployments: %v", err)
	}
	deployments, err = db.ListDeployments(1)
	if err != nil {
		t.Fatalf("list deleted project one deployments: %v", err)
	}
	if len(deployments) != 0 {
		t.Fatalf("project one deployments remain after delete: %+v", deployments)
	}
	deployments, err = db.ListDeployments(2)
	if err != nil {
		t.Fatalf("list project two deployments: %v", err)
	}
	if len(deployments) != 1 || deployments[0].ID != otherDeployment.ID || deployments[0].ProjectID != 2 {
		t.Fatalf("project two deployment was not isolated from project one delete: %+v", deployments)
	}
}
