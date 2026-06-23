package service

import (
	"fmt"
	"time"

	"github.com/TamgaLabs/Tamga/backend/internal/config"
	"github.com/TamgaLabs/Tamga/backend/internal/domain"
	"github.com/TamgaLabs/Tamga/backend/internal/repository/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	db  *sqlite.DB
	cfg config.Config
}

func NewAuthService(db *sqlite.DB, cfg config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg}
}

func (s *AuthService) Setup(password string) (*domain.User, error) {
	exists, err := s.db.HasUsers()
	if err != nil {
		return nil, fmt.Errorf("check users: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("already setup")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	return s.db.CreateUser(string(hash))
}

func (s *AuthService) Login(password string) (string, error) {
	user, err := s.db.FindFirstUser()
	if err != nil {
		return "", domain.ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", domain.ErrUnauthorized
	}
	token, err := s.generateToken(user.ID)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func (s *AuthService) ValidateToken(tokenStr string) (int64, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return 0, domain.ErrUnauthorized
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, domain.ErrUnauthorized
	}
	userID, ok := claims["user_id"].(float64)
	if !ok {
		return 0, domain.ErrUnauthorized
	}
	return int64(userID), nil
}

func (s *AuthService) IsSetup() (bool, error) {
	return s.db.HasUsers()
}

func (s *AuthService) generateToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(72 * time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}
