package service

import "testing"

// This file is a deliberate exception to FEAT-021's move of tests into
// internal/tests/: TestInjectToken exercises injectToken, an unexported
// pure URL-rewriting helper, directly - this is how clone-credential
// injection is verified across several URL shapes (with/without username,
// http vs ssh) without spinning up a DB-backed GitCredentialService for
// each case or relying on a live GitHub/GitLab remote. There is no exported
// equivalent to call it through in isolation, so it stays colocated.
// GitCredentialService's own CRUD behavior (Get/Set/Delete/
// AuthenticatedCloneURL/SandboxEnv) is covered black-box in
// internal/tests/service/git_credential_service_test.go.
func TestInjectToken(t *testing.T) {
	cases := []struct {
		name     string
		repoURL  string
		username string
		token    string
		want     string
	}{
		{
			name:    "https no username uses token as userinfo",
			repoURL: "https://github.com/org/repo.git",
			token:   "TOKEN123",
			want:    "https://TOKEN123@github.com/org/repo.git",
		},
		{
			name:     "https with username uses user:token",
			repoURL:  "https://gitlab.example.com/org/repo.git",
			username: "oauth2",
			token:    "TOKEN123",
			want:     "https://oauth2:TOKEN123@gitlab.example.com/org/repo.git",
		},
		{
			name:    "http scheme also rewritten (local test remotes)",
			repoURL: "http://127.0.0.1:8080/repo.git",
			token:   "local-token",
			want:    "http://local-token@127.0.0.1:8080/repo.git",
		},
		{
			name:    "ssh url left untouched",
			repoURL: "git@github.com:org/repo.git",
			token:   "TOKEN123",
			want:    "git@github.com:org/repo.git",
		},
		{
			name:    "ssh scheme url left untouched",
			repoURL: "ssh://git@github.com/org/repo.git",
			token:   "TOKEN123",
			want:    "ssh://git@github.com/org/repo.git",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := injectToken(tc.repoURL, tc.username, tc.token)
			if got != tc.want {
				t.Errorf("injectToken(%q, %q, %q) = %q, want %q", tc.repoURL, tc.username, tc.token, got, tc.want)
			}
		})
	}
}
