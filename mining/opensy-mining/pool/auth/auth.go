package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config holds authentication configuration
type Config struct {
	SecretKey       string
	TokenExpiry     time.Duration
	RefreshExpiry   time.Duration
	Issuer          string
	AllowedOrigins  []string
	RateLimitPerMin int
}

// DefaultConfig returns default configuration
func DefaultConfig() Config {
	return Config{
		TokenExpiry:     1 * time.Hour,
		RefreshExpiry:   24 * time.Hour,
		Issuer:          "opensy-pool",
		RateLimitPerMin: 60,
	}
}

// Auth handles JWT authentication
type Auth struct {
	cfg       Config
	secretKey []byte

	// Token blacklist (for logout)
	blacklist   map[string]time.Time
	blacklistMu sync.RWMutex

	// API keys for service accounts
	apiKeys   map[string]*APIKey
	apiKeysMu sync.RWMutex
}

// APIKey represents a service API key
type APIKey struct {
	ID        string
	Name      string
	Key       string
	Scopes    []string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// Claims holds JWT claims
type Claims struct {
	jwt.RegisteredClaims
	Address string   `json:"address"`
	Scopes  []string `json:"scopes,omitempty"`
	Type    string   `json:"type"` // "access" or "refresh"
}

// New creates a new auth handler
func New(cfg Config) (*Auth, error) {
	if cfg.SecretKey == "" {
		// Generate random secret
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		cfg.SecretKey = hex.EncodeToString(key)
	}

	a := &Auth{
		cfg:       cfg,
		secretKey: []byte(cfg.SecretKey),
		blacklist: make(map[string]time.Time),
		apiKeys:   make(map[string]*APIKey),
	}
	go a.cleanupLoop()
	return a, nil
}

// GenerateTokenPair generates access and refresh tokens
func (a *Auth) GenerateTokenPair(address string, scopes []string) (accessToken, refreshToken string, err error) {
	now := time.Now()

	// Access token
	accessClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.cfg.Issuer,
			Subject:   address,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.cfg.TokenExpiry)),
			ID:        generateID(),
		},
		Address: address,
		Scopes:  scopes,
		Type:    "access",
	}
	accessToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(a.secretKey)
	if err != nil {
		return "", "", err
	}

	// Refresh token
	refreshClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.cfg.Issuer,
			Subject:   address,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.cfg.RefreshExpiry)),
			ID:        generateID(),
		},
		Address: address,
		Type:    "refresh",
	}
	refreshToken, err = jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims).SignedString(a.secretKey)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// ValidateToken validates a JWT token
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Check blacklist
	a.blacklistMu.RLock()
	_, blacklisted := a.blacklist[claims.ID]
	a.blacklistMu.RUnlock()

	if blacklisted {
		return nil, errors.New("token has been revoked")
	}

	return claims, nil
}

// RefreshToken creates new access token from refresh token
func (a *Auth) RefreshToken(refreshTokenString string) (string, error) {
	claims, err := a.ValidateToken(refreshTokenString)
	if err != nil {
		return "", err
	}

	if claims.Type != "refresh" {
		return "", errors.New("not a refresh token")
	}

	// Generate new access token
	now := time.Now()
	accessClaims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    a.cfg.Issuer,
			Subject:   claims.Subject,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.cfg.TokenExpiry)),
			ID:        generateID(),
		},
		Address: claims.Address,
		Scopes:  claims.Scopes,
		Type:    "access",
	}

	return jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString(a.secretKey)
}

// RevokeToken adds token to blacklist
func (a *Auth) RevokeToken(claims *Claims) {
	a.blacklistMu.Lock()
	a.blacklist[claims.ID] = claims.ExpiresAt.Time
	a.blacklistMu.Unlock()
}

// CreateAPIKey creates a new API key
func (a *Auth) CreateAPIKey(name string, scopes []string, expiresIn *time.Duration) (*APIKey, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}

	apiKey := &APIKey{
		ID:        generateID(),
		Name:      name,
		Key:       "sk_" + hex.EncodeToString(key),
		Scopes:    scopes,
		CreatedAt: time.Now(),
	}

	if expiresIn != nil {
		exp := time.Now().Add(*expiresIn)
		apiKey.ExpiresAt = &exp
	}

	a.apiKeysMu.Lock()
	a.apiKeys[apiKey.Key] = apiKey
	a.apiKeysMu.Unlock()

	return apiKey, nil
}

// ValidateAPIKey validates an API key
func (a *Auth) ValidateAPIKey(key string) (*APIKey, error) {
	a.apiKeysMu.RLock()
	apiKey, exists := a.apiKeys[key]
	a.apiKeysMu.RUnlock()

	if !exists {
		return nil, errors.New("invalid API key")
	}

	if apiKey.ExpiresAt != nil && time.Now().After(*apiKey.ExpiresAt) {
		return nil, errors.New("API key expired")
	}

	return apiKey, nil
}

// RevokeAPIKey revokes an API key
func (a *Auth) RevokeAPIKey(key string) {
	a.apiKeysMu.Lock()
	delete(a.apiKeys, key)
	a.apiKeysMu.Unlock()
}

// HasScope checks if claims have a specific scope
func (a *Auth) HasScope(claims *Claims, scope string) bool {
	for _, s := range claims.Scopes {
		if s == scope || s == "*" {
			return true
		}
	}
	return false
}

func (a *Auth) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()

		// Cleanup expired blacklist entries
		a.blacklistMu.Lock()
		for id, exp := range a.blacklist {
			if now.After(exp) {
				delete(a.blacklist, id)
			}
		}
		a.blacklistMu.Unlock()

		// Cleanup expired API keys
		a.apiKeysMu.Lock()
		for key, apiKey := range a.apiKeys {
			if apiKey.ExpiresAt != nil && now.After(*apiKey.ExpiresAt) {
				delete(a.apiKeys, key)
			}
		}
		a.apiKeysMu.Unlock()
	}
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Middleware provides HTTP middleware for authentication

// ContextKey is the context key type
type ContextKey string

const (
	// ClaimsKey is the context key for JWT claims
	ClaimsKey ContextKey = "claims"
	// APIKeyKey is the context key for API key
	APIKeyKey ContextKey = "api_key"
)

// Middleware returns authentication middleware
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try Bearer token first
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := a.ValidateToken(tokenString)
			if err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}
			if claims.Type != "access" {
				http.Error(w, "Invalid token type", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Try API key
		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			key, err := a.ValidateAPIKey(apiKey)
			if err != nil {
				http.Error(w, "Invalid API key: "+err.Error(), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), APIKeyKey, key)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		http.Error(w, "Authentication required", http.StatusUnauthorized)
	})
}

// OptionalMiddleware allows unauthenticated requests but adds claims if present
func (a *Auth) OptionalMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := a.ValidateToken(tokenString)
			if err == nil && claims.Type == "access" {
				ctx := context.WithValue(r.Context(), ClaimsKey, claims)
				r = r.WithContext(ctx)
			}
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey != "" {
			key, err := a.ValidateAPIKey(apiKey)
			if err == nil {
				ctx := context.WithValue(r.Context(), APIKeyKey, key)
				r = r.WithContext(ctx)
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RequireScope middleware checks for required scope
func (a *Auth) RequireScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check JWT claims
			if claims, ok := r.Context().Value(ClaimsKey).(*Claims); ok {
				if a.HasScope(claims, scope) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Check API key scopes
			if key, ok := r.Context().Value(APIKeyKey).(*APIKey); ok {
				for _, s := range key.Scopes {
					if s == scope || s == "*" {
						next.ServeHTTP(w, r)
						return
					}
				}
			}

			http.Error(w, "Insufficient permissions", http.StatusForbidden)
		})
	}
}

// GetClaims extracts claims from context
func GetClaims(ctx context.Context) *Claims {
	claims, _ := ctx.Value(ClaimsKey).(*Claims)
	return claims
}

// GetAPIKey extracts API key from context
func GetAPIKey(ctx context.Context) *APIKey {
	key, _ := ctx.Value(APIKeyKey).(*APIKey)
	return key
}
