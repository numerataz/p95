package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"sixtyseven/internal/config"
	"sixtyseven/internal/domain"
	"sixtyseven/internal/repository/postgres"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrInvalidAPIKey      = errors.New("invalid API key")
	ErrAPIKeyExpired      = errors.New("API key expired")
)

// AuthService handles authentication operations
type AuthService struct {
	cfg      *config.Config
	users    *postgres.UserRepository
	apiKeys  *postgres.APIKeyRepository
	sessions *postgres.SessionRepository
	teams    *postgres.TeamRepository
}

// NewAuthService creates a new auth service
func NewAuthService(
	cfg *config.Config,
	users *postgres.UserRepository,
	apiKeys *postgres.APIKeyRepository,
	sessions *postgres.SessionRepository,
	teams *postgres.TeamRepository,
) *AuthService {
	return &AuthService{
		cfg:      cfg,
		users:    users,
		apiKeys:  apiKeys,
		sessions: sessions,
		teams:    teams,
	}
}

// LoginRequest contains login credentials
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse contains authentication tokens
type LoginResponse struct {
	AccessToken  string              `json:"access_token"`
	RefreshToken string              `json:"refresh_token"`
	ExpiresAt    time.Time           `json:"expires_at"`
	User         domain.UserResponse `json:"user"`
}

// JWTClaims contains JWT token claims
type JWTClaims struct {
	UserID  uuid.UUID `json:"user_id"`
	Email   string    `json:"email"`
	Name    string    `json:"name"`
	IsAdmin bool      `json:"is_admin"`
	jwt.RegisteredClaims
}

// Login authenticates a user and returns tokens
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	// Find user by email
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	// Check if user is active
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate tokens
	accessToken, expiresAt, err := s.generateAccessToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.generateRefreshToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         user.ToResponse(),
	}, nil
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req domain.UserCreate) (*domain.User, error) {
	// Check if email already exists
	existing, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("email already registered")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &domain.User{
		ID:           uuid.New(),
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		Name:         req.Name,
		IsActive:     true,
		IsAdmin:      false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create personal team for the user
	personalTeam := &domain.Team{
		ID:         uuid.New(),
		Name:       user.Name + "'s Team",
		Slug:       generateSlug(user.Name),
		Plan:       domain.TeamPlanFree,
		IsPersonal: true,
		Settings:   map[string]any{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.teams.Create(ctx, personalTeam); err != nil {
		return nil, fmt.Errorf("failed to create personal team: %w", err)
	}

	// Add user as owner of personal team
	member := &domain.TeamMember{
		ID:        uuid.New(),
		TeamID:    personalTeam.ID,
		UserID:    user.ID,
		Role:      domain.TeamRoleOwner,
		CreatedAt: time.Now(),
	}

	if err := s.teams.AddMember(ctx, member); err != nil {
		return nil, fmt.Errorf("failed to add user to personal team: %w", err)
	}

	return user, nil
}

// ValidateAccessToken validates a JWT access token
func (s *AuthService) ValidateAccessToken(ctx context.Context, tokenString string) (*domain.User, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrInvalidToken
	}

	if !token.Valid {
		return nil, ErrInvalidToken
	}

	// Get user from database
	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	return user, nil
}

// ValidateAPIKey validates an API key and returns the associated user
func (s *AuthService) ValidateAPIKey(ctx context.Context, rawKey string) (*domain.APIKeyInfo, error) {
	// Extract prefix (first 12 chars: "ss67_" + 7 more)
	if len(rawKey) < 12 {
		return nil, ErrInvalidAPIKey
	}
	prefix := rawKey[:12]

	// Get all keys with this prefix
	keys, err := s.apiKeys.GetByPrefix(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to get API keys: %w", err)
	}

	// Find the matching key by comparing hashes
	for _, key := range keys {
		if err := bcrypt.CompareHashAndPassword([]byte(key.KeyHash), []byte(rawKey)); err == nil {
			// Found matching key
			if key.IsExpired() {
				return nil, ErrAPIKeyExpired
			}

			// Update last used
			_ = s.apiKeys.UpdateLastUsed(ctx, key.ID)

			// Get team if scoped
			var team *domain.Team
			if key.TeamID != nil {
				team, _ = s.teams.GetByID(ctx, *key.TeamID)
			}

			return &domain.APIKeyInfo{
				Key:    key,
				User:   key.User,
				Team:   team,
				Scopes: key.Scopes,
			}, nil
		}
	}

	return nil, ErrInvalidAPIKey
}

// ListAPIKeysByUser lists all API keys for a user
func (s *AuthService) ListAPIKeysByUser(ctx context.Context, userID uuid.UUID) ([]*domain.APIKey, error) {
	keys, err := s.apiKeys.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list API keys: %w", err)
	}
	return keys, nil
}

// DeleteAPIKey deletes an API key (verifying ownership)
func (s *AuthService) DeleteAPIKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error {
	// Get the key to verify ownership
	key, err := s.apiKeys.GetByID(ctx, keyID)
	if err != nil {
		return fmt.Errorf("failed to get API key: %w", err)
	}
	if key == nil {
		return fmt.Errorf("API key not found")
	}

	// Verify ownership
	if key.UserID != userID {
		return fmt.Errorf("unauthorized to delete this API key")
	}

	// Delete the key
	if err := s.apiKeys.Delete(ctx, keyID); err != nil {
		return fmt.Errorf("failed to delete API key: %w", err)
	}

	return nil
}

// CreateAPIKey creates a new API key for a user
func (s *AuthService) CreateAPIKey(ctx context.Context, userID uuid.UUID, req domain.APIKeyCreate) (*domain.APIKeyResponse, error) {
	// Generate random key
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	rawKey := domain.APIKeyPrefix + base64.URLEncoding.EncodeToString(keyBytes)

	// Hash for storage
	hash, err := bcrypt.GenerateFromPassword([]byte(rawKey), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash key: %w", err)
	}

	// Set default scopes
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []domain.APIKeyScope{domain.APIKeyScopeRead, domain.APIKeyScopeWrite}
	}

	// Create API key
	apiKey := &domain.APIKey{
		ID:        uuid.New(),
		UserID:    userID,
		TeamID:    req.TeamID,
		KeyHash:   string(hash),
		KeyPrefix: rawKey[:12],
		Name:      req.Name,
		Scopes:    scopes,
		ExpiresAt: req.ExpiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.apiKeys.Create(ctx, apiKey); err != nil {
		return nil, fmt.Errorf("failed to create API key: %w", err)
	}

	return &domain.APIKeyResponse{
		APIKey: *apiKey,
		RawKey: rawKey,
	}, nil
}

// CreateSession creates a new session for a user
func (s *AuthService) CreateSession(ctx context.Context, userID uuid.UUID, deviceInfo map[string]any, ipAddress string) (string, error) {
	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}

	rawToken := base64.URLEncoding.EncodeToString(tokenBytes)
	tokenHash := hashToken(rawToken)

	session := &domain.Session{
		ID:         uuid.New(),
		UserID:     userID,
		TokenHash:  tokenHash,
		DeviceInfo: deviceInfo,
		IPAddress:  ipAddress,
		ExpiresAt:  time.Now().Add(s.cfg.SessionDuration),
		CreatedAt:  time.Now(),
	}

	if err := s.sessions.Create(ctx, session); err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return rawToken, nil
}

// ValidateSession validates a session token
func (s *AuthService) ValidateSession(ctx context.Context, rawToken string) (*domain.User, error) {
	tokenHash := hashToken(rawToken)

	session, err := s.sessions.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, ErrInvalidToken
	}

	if session.IsExpired() {
		_ = s.sessions.Delete(ctx, session.ID)
		return nil, ErrTokenExpired
	}

	return session.User, nil
}

// generateAccessToken generates a JWT access token
func (s *AuthService) generateAccessToken(user *domain.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.cfg.JWTAccessExpiration)

	claims := JWTClaims{
		UserID:  user.ID,
		Email:   user.Email,
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// generateRefreshToken generates a refresh token
func (s *AuthService) generateRefreshToken(user *domain.User) (string, error) {
	expiresAt := time.Now().Add(s.cfg.JWTRefreshExpiration)

	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   user.ID.String(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

// hashToken creates a SHA256 hash of a token
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.URLEncoding.EncodeToString(h[:])
}

// SeedAdmin creates the default admin user if no users exist
func (s *AuthService) SeedAdmin(ctx context.Context) error {
	// Check if any users exist
	users, err := s.users.List(ctx, 1, 0)
	if err != nil {
		return fmt.Errorf("failed to check existing users: %w", err)
	}

	// If users already exist, skip seeding
	if len(users) > 0 {
		return nil
	}

	// Get admin credentials from config
	email := s.cfg.AdminEmail
	password := s.cfg.AdminPassword
	if email == "" || password == "" {
		return fmt.Errorf("ADMIN_EMAIL and ADMIN_PASSWORD must be set for initial setup")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create admin user
	user := &domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         "Admin",
		IsActive:     true,
		IsAdmin:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.users.Create(ctx, user); err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Create personal team for admin
	personalTeam := &domain.Team{
		ID:         uuid.New(),
		Name:       "Default",
		Slug:       "default",
		Plan:       domain.TeamPlanFree,
		IsPersonal: true,
		Settings:   map[string]any{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if err := s.teams.Create(ctx, personalTeam); err != nil {
		return fmt.Errorf("failed to create default team: %w", err)
	}

	// Add admin as owner
	member := &domain.TeamMember{
		ID:        uuid.New(),
		TeamID:    personalTeam.ID,
		UserID:    user.ID,
		Role:      domain.TeamRoleOwner,
		CreatedAt: time.Now(),
	}

	if err := s.teams.AddMember(ctx, member); err != nil {
		return fmt.Errorf("failed to add admin to team: %w", err)
	}

	return nil
}

// generateSlug creates a URL-friendly slug from a name
func generateSlug(name string) string {
	// Simple slug generation - in production use a proper slugify library
	slug := ""
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			slug += string(r)
		} else if r >= 'A' && r <= 'Z' {
			slug += string(r + 32) // lowercase
		} else if r == ' ' {
			slug += "-"
		}
	}
	// Add random suffix to ensure uniqueness
	suffix := make([]byte, 4)
	rand.Read(suffix)
	return slug + "-" + fmt.Sprintf("%x", suffix)
}
