// Package auth provides JWT token generation and validation for user authentication.
package auth

import (
	"fmt"
	"time"

	"land-of-stamp-backend/apperrors"
	"land-of-stamp-backend/constants"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey []byte

// Init sets the JWT signing key used for token generation and validation.
func Init(secret string) {
	jwtKey = []byte(secret)
}

// Claims represents the JWT claims payload containing user identity information.
type Claims struct {
	UserID   string `json:"userId"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a signed JWT token for the given user.
func GenerateToken(userID, username, role string) (string, error) {
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(constants.JWTExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ValidateToken parses and validates a JWT token string, returning the claims if valid.
func ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("%w: %v", apperrors.ErrUnexpectedSigningMethod, t.Header["alg"])
		}
		return jwtKey, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, apperrors.ErrInvalidToken
	}
	return claims, nil
}
