package service

import (
	"os"
	"testing"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/golang-jwt/jwt/v5"
)

func newTestAuthService(t *testing.T) (*AuthService, config.Config) {
	t.Helper()
	dbPath := "/tmp/test_auth_service_" + t.Name() + ".db"
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

	cfg := config.Config{JWTSecret: "test-jwt-secret"}
	return NewAuthService(db, cfg), cfg
}

// TestAuthServiceSetupAndLogin covers the happy path: first-run setup,
// login with the right password, and token validation of the resulting
// token.
func TestAuthServiceSetupAndLogin(t *testing.T) {
	svc, _ := newTestAuthService(t)

	setup, err := svc.IsSetup()
	if err != nil {
		t.Fatalf("is setup: %v", err)
	}
	if setup {
		t.Fatal("expected not set up yet")
	}

	user, err := svc.Setup("correct-password")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	setup, err = svc.IsSetup()
	if err != nil {
		t.Fatalf("is setup after setup: %v", err)
	}
	if !setup {
		t.Fatal("expected set up after Setup()")
	}

	// Setup again must fail - only one admin user is ever allowed.
	if _, err := svc.Setup("another-password"); err == nil {
		t.Fatal("expected error re-running setup")
	}

	token, err := svc.Login("correct-password")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	userID, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("validate token: %v", err)
	}
	if userID != user.ID {
		t.Fatalf("expected user id %d, got %d", user.ID, userID)
	}
}

// TestAuthServiceLoginWrongPassword covers the invalid-credentials failure
// path.
func TestAuthServiceLoginWrongPassword(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if _, err := svc.Setup("correct-password"); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := svc.Login("wrong-password"); err != domain.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// TestAuthServiceLoginNoUser covers logging in before any user has been
// set up.
func TestAuthServiceLoginNoUser(t *testing.T) {
	svc, _ := newTestAuthService(t)
	if _, err := svc.Login("anything"); err != domain.ErrUnauthorized {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// TestAuthServiceValidateTokenFailures exercises ValidateToken's failure
// modes: garbage input, wrong signing secret, and an expired token. There
// is no server-side session to invalidate on logout (JWTs are stateless -
// see auth_service.go), so from the service's point of view "logout" is
// exactly this: a token that no longer validates, whether because it's
// missing, malformed, expired, or signed with a different secret.
func TestAuthServiceValidateTokenFailures(t *testing.T) {
	svc, cfg := newTestAuthService(t)

	if _, err := svc.ValidateToken("not-a-jwt"); err != domain.ErrUnauthorized {
		t.Errorf("garbage token: expected ErrUnauthorized, got %v", err)
	}

	if _, err := svc.ValidateToken(""); err != domain.ErrUnauthorized {
		t.Errorf("empty token: expected ErrUnauthorized, got %v", err)
	}

	// Signed with a different secret than the service trusts.
	wrongSecretToken := signTestToken(t, "some-other-secret", 1, time.Now().Add(time.Hour))
	if _, err := svc.ValidateToken(wrongSecretToken); err != domain.ErrUnauthorized {
		t.Errorf("wrong-secret token: expected ErrUnauthorized, got %v", err)
	}

	// Correctly signed but already expired.
	expiredToken := signTestToken(t, cfg.JWTSecret, 1, time.Now().Add(-time.Hour))
	if _, err := svc.ValidateToken(expiredToken); err != domain.ErrUnauthorized {
		t.Errorf("expired token: expected ErrUnauthorized, got %v", err)
	}
}

func signTestToken(t *testing.T, secret string, userID int64, expiresAt time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return signed
}
