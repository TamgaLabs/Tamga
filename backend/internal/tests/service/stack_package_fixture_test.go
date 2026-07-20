package service_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

func TestStackSelectionFixturesUseRepositorySignals(t *testing.T) {
	tests := []struct {
		name  string
		files map[string][]byte
		want  string
		port  int
	}{
		{"Next.js", map[string][]byte{"package.json": []byte(`{"dependencies":{"next":"15"},"scripts":{"build":"next build","start":"next start"}}`)}, "nextjs", 3000},
		{"Maven Spring", map[string][]byte{"pom.xml": []byte("<artifactId>spring-boot</artifactId>")}, "spring-boot-maven", 8080},
		{"Gradle Spring", map[string][]byte{"build.gradle": []byte("id 'org.springframework.boot'"), "settings.gradle": []byte("rootProject.name='app'")}, "spring-boot-gradle", 8080},
		{"static Nginx", map[string][]byte{"nginx.conf": []byte("events {}"), "dist/index.html": []byte("ok")}, "static-nginx", 80},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := service.SelectStackDefinition(service.StackSelectionInput{Files: test.files})
			if got.ID != test.want || got.Recipe.Port != test.port || got.Confidence == service.ConfidenceUnknown {
				t.Fatalf("SelectStackDefinition() = %+v, want %q port %d", got, test.want, test.port)
			}
		})
	}
}

func TestStackSelectionAuthorityAndAmbiguityRemainManual(t *testing.T) {
	next := []byte(`{"dependencies":{"next":"15"},"scripts":{"build":"next build","start":"next start"}}`)
	tests := []struct {
		name     string
		files    map[string][]byte
		want     string
		evidence string
	}{
		{"Compose authority", map[string][]byte{"compose.yaml": []byte("services: {}"), "package.json": next}, "compose", "user-supplied"},
		{"Dockerfile authority", map[string][]byte{"Dockerfile": []byte("FROM node"), "package.json": next}, "dockerfile", "user-supplied"},
		{"conflicting authority", map[string][]byte{"compose.yaml": []byte("services: {}"), "Dockerfile": []byte("FROM node")}, "unknown", "conflicting"},
		{"incompatible framework", map[string][]byte{"pom.xml": []byte("spring-boot"), "build.gradle": []byte("spring-boot"), "settings.gradle": []byte("rootProject.name='app'")}, "unknown", "incompatible"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := service.SelectStackDefinition(service.StackSelectionInput{Files: test.files})
			if got.ID != test.want || (test.want == "unknown" && (!got.Recipe.Manual || got.Confidence != service.ConfidenceUnknown)) || !strings.Contains(got.Evidence[0], test.evidence) {
				t.Fatalf("SelectStackDefinition() = %+v, want %q / %q", got, test.want, test.evidence)
			}
		})
	}
}

func TestPackageManagerFixturesRespectSafePrecedence(t *testing.T) {
	tests := []struct {
		name       string
		input      service.PackageManagerInput
		want       service.PackageManager
		confidence service.Confidence
		install    []string
	}{
		{"native manifest wins", service.PackageManagerInput{Files: map[string][]byte{"go.mod": []byte("module example"), "pnpm-lock.yaml": []byte("lock"), "package.json": []byte(`{"packageManager":"yarn@4"}`)}, Metadata: map[string]string{"packageManager": "bun@1"}}, service.PackageManagerGo, service.ConfidenceHigh, []string{"go", "mod", "download"}},
		{"lockfile wins", service.PackageManagerInput{Files: map[string][]byte{"package.json": []byte(`{"packageManager":"yarn@4"}`), "pnpm-lock.yaml": []byte("lock")}}, service.PackageManagerPNPM, service.ConfidenceHigh, []string{"pnpm", "install", "--frozen-lockfile"}},
		{"metadata wins before fallback", service.PackageManagerInput{Files: map[string][]byte{"package.json": []byte(`{"name":"app"}`)}, Metadata: map[string]string{"packageManager": "bun@1"}}, service.PackageManagerBun, service.ConfidenceHigh, []string{"bun", "install", "--frozen-lockfile"}},
		{"package json fallback", service.PackageManagerInput{Files: map[string][]byte{"package.json": []byte(`{"name":"app"}`)}}, service.PackageManagerNPM, service.ConfidenceLow, []string{"npm", "ci"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := service.ResolvePackageManager(test.input)
			if got.Manager != test.want || got.Confidence != test.confidence || got.Recipe.Manual || !reflect.DeepEqual(got.Recipe.Install, test.install) {
				t.Fatalf("ResolvePackageManager() = %+v", got)
			}
		})
	}
}

func TestPackageManagerConflictsAreManual(t *testing.T) {
	got := service.ResolvePackageManager(service.PackageManagerInput{Files: map[string][]byte{"package-lock.json": []byte("{}"), "yarn.lock": []byte("lock")}})
	if got.Manager != service.PackageManagerUnknown || !got.Recipe.Manual || got.Confidence != service.ConfidenceUnknown || !strings.Contains(got.Evidence[0], "conflicting") {
		t.Fatalf("conflicting lockfiles = %+v", got)
	}
}
