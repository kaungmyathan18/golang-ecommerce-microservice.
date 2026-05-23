package service

import (
	"context"
	"errors"
	"strings"

	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/repository"
	"github.com/kaungmyathan18/golang-ecommerce-microservice/auth-service/internal/token"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailTaken         = errors.New("email already registered")
)

type AuthResponse struct {
	AccessToken string           `json:"access_token"`
	TokenType   string           `json:"token_type"`
	ExpiresIn   int64            `json:"expires_in"`
	User        *repository.User `json:"user"`
}

type AuthService struct {
	repo   *repository.Repository
	tokens *token.Issuer
}

func NewAuthService(repo *repository.Repository, tokens *token.Issuer) *AuthService {
	return &AuthService{repo: repo, tokens: tokens}
}

func (s *AuthService) Register(ctx context.Context, email, name, password string) (*AuthResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	name = strings.TrimSpace(name)
	if email == "" || name == "" || len(password) < 8 {
		return nil, errors.New("invalid input")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.CreateUser(ctx, email, name, string(hash))
	if err != nil {
		if errors.Is(err, repository.ErrEmailTaken) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return s.issueToken(user)
}

func (s *AuthService) Login(ctx context.Context, email, password string) (*AuthResponse, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, hash, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return s.issueToken(user)
}

func (s *AuthService) Me(ctx context.Context, userID string) (*repository.User, error) {
	return s.repo.GetByID(ctx, userID)
}

func (s *AuthService) issueToken(user *repository.User) (*AuthResponse, error) {
	accessToken, _, err := s.tokens.Sign(user.ID, user.Email)
	if err != nil {
		return nil, err
	}
	return &AuthResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.tokens.TTL().Seconds()),
		User:        user,
	}, nil
}
