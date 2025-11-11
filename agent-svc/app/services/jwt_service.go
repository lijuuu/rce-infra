package services

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTService handles JWT token generation and validation
type JWTService struct {
	secret     []byte
	expiration time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, expirationSec int64) *JWTService {
	return &JWTService{
		secret:     []byte(secret),
		expiration: time.Duration(expirationSec) * time.Second,
	}
}

// Claims represents JWT claims
type Claims struct {
	NodeID string `json:"node_id"`
	jwt.RegisteredClaims
}

// GenerateToken generates a JWT token for a node
func (j *JWTService) GenerateToken(nodeID string) (string, error) {
	now := time.Now()
	claims := &Claims{
		NodeID: nodeID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   nodeID,
			Issuer:    "agent-svc",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(j.expiration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(j.secret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the node ID
func (j *JWTService) ValidateToken(tokenString string) (string, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secret, nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	return claims.NodeID, nil
}
