package service

import (
	"crypto/sha256"
	"fmt"
	"net/url"

	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
)

// GitCredentialService owns the single, global git credential (see
// FEAT-008): used by project_service.go's cloneRepo for `git clone`/`pull`
// of private repos, and injected into every agent sandbox by
// agent_service.go's StartSandbox so `git commit`/`push` works from the
// terminal. Same single-row "Get/Set" shape as ResourceLimitService, plus
// the AES-GCM at-rest encryption pattern from crypto.go.
type GitCredentialService struct {
	db      *sqlite.DB
	authKey []byte
}

func NewGitCredentialService(db *sqlite.DB, jwtSecret string) *GitCredentialService {
	h := sha256.Sum256([]byte(jwtSecret))
	return &GitCredentialService{db: db, authKey: h[:]}
}

// Get returns the credential's metadata (never the decrypted token) for
// display in Settings.
func (s *GitCredentialService) Get() (*domain.GitCredentialResponse, error) {
	c, err := s.db.GetGitCredential()
	if err != nil {
		return nil, err
	}
	return s.toResponse(c), nil
}

// Set encrypts and stores a new credential, replacing any existing one.
func (s *GitCredentialService) Set(provider, username, token string) (*domain.GitCredentialResponse, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}
	enc, err := encryptSecret(s.authKey, token)
	if err != nil {
		return nil, fmt.Errorf("encrypt token: %w", err)
	}
	c := &domain.GitCredential{Provider: provider, Username: username, TokenEnc: enc}
	if err := s.db.UpdateGitCredential(c); err != nil {
		return nil, fmt.Errorf("save git credential: %w", err)
	}
	return s.Get()
}

// Delete clears the stored credential (provider/username/token all reset
// to empty - Get() then reports has_token=false).
func (s *GitCredentialService) Delete() error {
	return s.db.UpdateGitCredential(&domain.GitCredential{})
}

// decrypted returns the credential's username and decrypted token, for
// internal use by cloneRepo/StartSandbox. token is empty if no credential
// has been configured yet, in which case callers should proceed
// unauthenticated rather than error.
func (s *GitCredentialService) decrypted() (username, token string, err error) {
	c, err := s.db.GetGitCredential()
	if err != nil {
		return "", "", err
	}
	if c.TokenEnc == "" {
		return "", "", nil
	}
	tok, err := decryptSecret(s.authKey, c.TokenEnc)
	if err != nil {
		return "", "", fmt.Errorf("decrypt token: %w", err)
	}
	return c.Username, tok, nil
}

func (s *GitCredentialService) toResponse(c *domain.GitCredential) *domain.GitCredentialResponse {
	return &domain.GitCredentialResponse{
		Provider:  c.Provider,
		Username:  c.Username,
		HasToken:  c.TokenEnc != "",
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// AuthenticatedCloneURL returns repoURL with the stored credential's token
// injected as HTTP basic-auth userinfo (e.g. https://TOKEN@github.com/org/repo.git,
// or https://user:TOKEN@host/... if a username is set). Only http(s) URLs
// are rewritten - ssh/git URLs are returned unchanged since token auth
// doesn't apply to them. Returns repoURL unchanged if no credential is
// configured, so callers can use the result unconditionally.
func (s *GitCredentialService) AuthenticatedCloneURL(repoURL string) (string, error) {
	username, token, err := s.decrypted()
	if err != nil {
		return "", err
	}
	if token == "" {
		return repoURL, nil
	}
	return injectToken(repoURL, username, token), nil
}

// SandboxEnv returns the env vars that configure git inside an agent
// sandbox to authenticate as the stored credential for commit/push, using
// git's GIT_CONFIG_COUNT/GIT_CONFIG_KEY_n/GIT_CONFIG_VALUE_n mechanism
// (git >= 2.31, present via `apk add git` in the node:22-alpine agent
// image) rather than writing files into the container - there's no
// container-creation hook to run setup commands, and the credential can
// change at runtime independent of the (static, shared) agent image.
//
// Also sets user.name/user.email, since the sandbox otherwise has no git
// identity at all and `git commit` would fail outright - the Test Plan's
// commit-then-push flow needs both.
//
// Returns nil, nil if no credential is configured, in which case sandbox
// creation proceeds with git's untouched defaults.
func (s *GitCredentialService) SandboxEnv() ([]string, error) {
	username, token, err := s.decrypted()
	if err != nil {
		return nil, err
	}
	if token == "" {
		return nil, nil
	}
	name := username
	if name == "" {
		name = "tamga-agent"
	}
	return []string{
		"GIT_CRED_USERNAME=" + username,
		"GIT_CRED_TOKEN=" + token,
		"GIT_CONFIG_COUNT=3",
		"GIT_CONFIG_KEY_0=credential.helper",
		"GIT_CONFIG_VALUE_0=!f() { echo username=${GIT_CRED_USERNAME}; echo password=${GIT_CRED_TOKEN}; }; f",
		"GIT_CONFIG_KEY_1=user.name",
		"GIT_CONFIG_VALUE_1=" + name,
		"GIT_CONFIG_KEY_2=user.email",
		"GIT_CONFIG_VALUE_2=" + name + "@tamga.local",
	}, nil
}

// injectToken rewrites an http(s) repo URL to embed username/token as
// basic-auth userinfo. Non-http(s) URLs (ssh, git@host:...) or unparseable
// input are returned unchanged - token auth over the clone URL only
// applies to http(s) transport.
func injectToken(repoURL, username, token string) string {
	u, err := url.Parse(repoURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return repoURL
	}
	if username != "" {
		u.User = url.UserPassword(username, token)
	} else {
		u.User = url.User(token)
	}
	return u.String()
}
