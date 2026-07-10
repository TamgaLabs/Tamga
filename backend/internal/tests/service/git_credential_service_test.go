package service_test

import (
	"os"
	"testing"

	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/TamgaLabs/Tamga/backend/internal/service"
)

// newTestGitCredentialService also returns the underlying *sqlite.DB so
// tests can check what's actually stored at rest (GitCredentialService's own
// db field is unexported, so a black-box test needs its own handle rather
// than reaching through the service).
func newTestGitCredentialService(t *testing.T) (*service.GitCredentialService, *sqlite.DB) {
	t.Helper()
	dbPath := "/tmp/test_git_credential_service_" + t.Name() + ".db"
	os.Remove(dbPath)
	t.Cleanup(func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-wal")
		os.Remove(dbPath + "-shm")
	})

	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return service.NewGitCredentialService(db, "test-jwt-secret"), db
}

func TestGitCredentialServiceGetSetDelete(t *testing.T) {
	svc, db := newTestGitCredentialService(t)

	// Seeded row exists but no credential configured yet.
	resp, err := svc.Get()
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if resp.HasToken {
		t.Fatalf("expected no credential configured yet, got %+v", resp)
	}

	// AuthenticatedCloneURL/SandboxEnv should be no-ops with no credential.
	url, err := svc.AuthenticatedCloneURL("https://github.com/org/repo.git")
	if err != nil {
		t.Fatalf("authenticated clone url: %v", err)
	}
	if url != "https://github.com/org/repo.git" {
		t.Fatalf("expected unchanged url with no credential, got %q", url)
	}
	env, err := svc.SandboxEnv()
	if err != nil {
		t.Fatalf("sandbox env: %v", err)
	}
	if env != nil {
		t.Fatalf("expected nil sandbox env with no credential, got %v", env)
	}

	// Set a credential.
	resp, err = svc.Set("github", "my-user", "ghp_supersecret")
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	if resp.Provider != "github" || resp.Username != "my-user" || !resp.HasToken {
		t.Fatalf("unexpected response after set: %+v", resp)
	}
	if resp.CreatedAt.IsZero() || resp.UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps populated: %+v", resp)
	}

	// Now the clone URL should carry the credential.
	url, err = svc.AuthenticatedCloneURL("https://github.com/org/repo.git")
	if err != nil {
		t.Fatalf("authenticated clone url after set: %v", err)
	}
	if want := "https://my-user:ghp_supersecret@github.com/org/repo.git"; url != want {
		t.Fatalf("expected %q, got %q", want, url)
	}

	// And SandboxEnv should carry the credential helper + identity.
	env, err = svc.SandboxEnv()
	if err != nil {
		t.Fatalf("sandbox env after set: %v", err)
	}
	assertContains(t, env, "GIT_CRED_USERNAME=my-user")
	assertContains(t, env, "GIT_CRED_TOKEN=ghp_supersecret")
	assertContains(t, env, "GIT_CONFIG_KEY_1=user.name")
	assertContains(t, env, "GIT_CONFIG_VALUE_1=my-user")

	// The token must never be stored in plaintext in the DB.
	raw, err := db.GetGitCredential()
	if err != nil {
		t.Fatalf("raw get: %v", err)
	}
	if raw.TokenEnc == "" || raw.TokenEnc == "ghp_supersecret" {
		t.Fatalf("expected token to be encrypted at rest, got %q", raw.TokenEnc)
	}

	// Delete clears it.
	if err := svc.Delete(); err != nil {
		t.Fatalf("delete: %v", err)
	}
	resp, err = svc.Get()
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if resp.HasToken {
		t.Fatalf("expected no credential after delete, got %+v", resp)
	}
}

func TestGitCredentialServiceSetRequiresToken(t *testing.T) {
	svc, _ := newTestGitCredentialService(t)
	if _, err := svc.Set("github", "user", ""); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func assertContains(t *testing.T, haystack []string, needle string) {
	t.Helper()
	for _, s := range haystack {
		if s == needle {
			return
		}
	}
	t.Fatalf("expected %v to contain %q", haystack, needle)
}
