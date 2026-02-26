package jwt

import (
	"crypto/rsa"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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
