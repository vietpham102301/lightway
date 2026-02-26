package jwt

import (
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the custom JWT claims.
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateToken creates a JWT with RS256 for the given user.
func GenerateToken(privateKey *rsa.PrivateKey, userID int, username, role string, expiresInHours int) (string, error) {
	if privateKey == nil {
		return "", jwt.ErrInvalidKey
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id":  userID,
		"username": username,
		"role":     role,
		"exp":      now.Add(time.Duration(expiresInHours) * time.Hour).Unix(),
		"iat":      now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(privateKey)
}

// ValidateToken parses and validates a JWT token string using the given RSA public key.
// Returns the parsed Claims if the token is valid, or an error otherwise.
func ValidateToken(publicKey *rsa.PublicKey, tokenString string) (*Claims, error) {
	if publicKey == nil {
		return nil, jwt.ErrInvalidKey
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
